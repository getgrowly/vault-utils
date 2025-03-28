package server

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/getgrowly/vault-utils/pkg/kubernetes"
	"github.com/getgrowly/vault-utils/pkg/vault"
)

const (
	defaultReadTimeout  = 10 * time.Second
	defaultWriteTimeout = 10 * time.Second
	defaultIdleTimeout  = 30 * time.Second
)

// Server represents the HTTP server for health and readiness checks
type Server struct {
	k8sClient *kubernetes.Client
	port      string
}

// NewServer creates a new HTTP server
func NewServer(k8sClient *kubernetes.Client, port string) *Server {
	return &Server{
		k8sClient: k8sClient,
		port:      port,
	}
}

// Start starts the HTTP server
func (s *Server) Start() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/ready", s.handleReady)

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%s", s.port),
		Handler:      mux,
		ReadTimeout:  defaultReadTimeout,
		WriteTimeout: defaultWriteTimeout,
		IdleTimeout:  defaultIdleTimeout,
	}

	log.Printf("Starting HTTP server on port %s", s.port)
	return srv.ListenAndServe()
}

// handleHealth handles health check requests
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	log.Printf("Health check request received from %s", r.RemoteAddr)
	w.WriteHeader(http.StatusOK)
}

// handleReady handles readiness check requests
func (s *Server) handleReady(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	log.Printf("Readiness check request received from %s", r.RemoteAddr)

	allReady := true

	pods, err := s.k8sClient.GetVaultPods("vault")
	if err != nil {
		log.Printf("Error getting Vault pods: %v", err)
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}

	for _, podIP := range pods {
		vaultAddr := fmt.Sprintf("http://%s:8200", podIP)
		vaultClient := vault.NewClient(vaultAddr)

		status, err := vaultClient.CheckStatus()
		if err != nil {
			log.Printf("Error checking Vault status for %s: %v", vaultAddr, err)
			allReady = false
			continue
		}

		if status.Sealed {
			allReady = false
		}
	}

	if !allReady {
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}

	w.WriteHeader(http.StatusOK)
}
