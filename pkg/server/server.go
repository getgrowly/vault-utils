package server

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/getgrowly/vault-utils/pkg/kubernetes"
	"github.com/getgrowly/vault-utils/pkg/vault"
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

	addr := fmt.Sprintf(":%s", s.port)
	log.Printf("Starting HTTP server on %s", addr)

	server := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  30 * time.Second,
	}

	return server.ListenAndServe()
}

// handleHealth handles health check requests
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	log.Printf("Health check request received from %s", r.RemoteAddr)
	w.WriteHeader(http.StatusOK)
}

// handleReady handles readiness check requests
func (s *Server) handleReady(w http.ResponseWriter, r *http.Request) {
	log.Printf("Readiness check request received from %s", r.RemoteAddr)

	pods, err := s.k8sClient.GetVaultPods("vault")
	if err != nil {
		log.Printf("Failed to get Vault pods: %v", err)
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}

	allReady := true
	for _, podIP := range pods {
		vaultAddr := fmt.Sprintf("http://%s:8200", podIP)
		vaultClient := vault.NewClient(vaultAddr)

		status, err := vaultClient.CheckStatus()
		if err != nil {
			log.Printf("Error checking Vault status for %s: %v", vaultAddr, err)
			allReady = false
			continue
		}

		// Consider a pod ready if it's running (can respond to health check)
		// regardless of whether it's sealed or not
		log.Printf("Vault pod %s status: initialized=%v, sealed=%v", vaultAddr, status.Initialized, status.Sealed)
	}

	if allReady {
		log.Printf("All Vault pods are running")
		w.WriteHeader(http.StatusOK)
	} else {
		log.Printf("Some Vault pods are not running")
		w.WriteHeader(http.StatusServiceUnavailable)
	}
}
