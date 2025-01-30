package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"log"
	"math/big"
	"net/http"
	"os"
	"time"
)

// autogenerateClientCaIfNeeded will create a client CA if a path is
// specified for the CA cert and key, and the files do not exist.
// AutogenerateClientCA must be set.
func autogenerateClientCaIfNeeded() {
	if !config.AutogenerateClientCA || config.ClientCACert == "" || config.ClientCAKey == "" {
		log.Println("Client CA generation is disabled or paths are not set.")
		return
	}

	// Check if the certificate and key files already exist
	if fileExists(config.ClientCACert) && fileExists(config.ClientCAKey) {
		log.Println("Client CA already exists, skipping generation.")
		return
	}

	log.Println("Generating new Client CA certificate and key...")

	// Generate a new RSA private key
	privateKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		log.Fatalf("Failed to generate private key: %v", err)
	}

	// Create a self-signed certificate template
	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		log.Fatalf("Failed to generate serial number: %v", err)
	}

	certTemplate := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			CommonName:   "Client CA",
			Organization: []string{"My IoT Exporter"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(200 * 365 * 24 * time.Hour), // Valid for 200 years
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLen:            2,
	}

	// Self-sign the certificate
	certDER, err := x509.CreateCertificate(rand.Reader, &certTemplate, &certTemplate, &privateKey.PublicKey, privateKey)
	if err != nil {
		log.Fatalf("Failed to create certificate: %v", err)
	}

	// Convert certificate to PEM format
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	if err := os.WriteFile(config.ClientCACert, certPEM, 0600); err != nil {
		log.Fatalf("Failed to save CA certificate: %v", err)
	}

	// Convert private key to PEM format
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(privateKey)})
	if err := os.WriteFile(config.ClientCAKey, keyPEM, 0600); err != nil {
		log.Fatalf("Failed to save CA private key: %v", err)
	}

	log.Println("Client CA certificate and key successfully generated.")
}

// fileExists checks if a file exists and is not a directory.
func fileExists(filename string) bool {
	info, err := os.Stat(filename)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

// getClientKeyEndpoint generates a client key and a signed certificate using the CA key
// and returns them in PEM format appended as a plain text response.
// EnableClientKeyGeneration must be set.
func getClientKeyEndpoint(w http.ResponseWriter, r *http.Request) {
	if !config.EnableClientKeyGeneration {
		http.Error(w, "Client key generation is disabled", http.StatusForbidden)
		return
	}

	// Read CA certificate
	caCertPEM, err := os.ReadFile(config.ClientCACert)
	if err != nil {
		http.Error(w, "Failed to read CA certificate", http.StatusInternalServerError)
		log.Printf("Error reading CA certificate: %v", err)
		return
	}

	// Decode CA certificate
	caBlock, _ := pem.Decode(caCertPEM)
	if caBlock == nil {
		http.Error(w, "Invalid CA certificate format", http.StatusInternalServerError)
		log.Println("Invalid CA certificate format")
		return
	}

	caCert, err := x509.ParseCertificate(caBlock.Bytes)
	if err != nil {
		http.Error(w, "Failed to parse CA certificate", http.StatusInternalServerError)
		log.Printf("Error parsing CA certificate: %v", err)
		return
	}

	// Read CA private key
	caKeyPEM, err := os.ReadFile(config.ClientCAKey)
	if err != nil {
		http.Error(w, "Failed to read CA private key", http.StatusInternalServerError)
		log.Printf("Error reading CA private key: %v", err)
		return
	}

	// Decode CA private key
	caKeyBlock, _ := pem.Decode(caKeyPEM)
	if caKeyBlock == nil {
		http.Error(w, "Invalid CA private key format", http.StatusInternalServerError)
		log.Println("Invalid CA private key format")
		return
	}

	caKey, err := x509.ParsePKCS1PrivateKey(caKeyBlock.Bytes)
	if err != nil {
		http.Error(w, "Failed to parse CA private key", http.StatusInternalServerError)
		log.Printf("Error parsing CA private key: %v", err)
		return
	}

	// Generate a new RSA private key for the client
	clientKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		http.Error(w, "Failed to generate client key", http.StatusInternalServerError)
		log.Printf("Error generating client key: %v", err)
		return
	}

	// Create a certificate signing request (CSR) template
	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		http.Error(w, "Failed to generate serial number", http.StatusInternalServerError)
		log.Printf("Error generating serial number: %v", err)
		return
	}

	clientCertTemplate := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			CommonName:   "IoT Client",
			Organization: []string{"My IoT Exporter"},
		},
		NotBefore:   time.Now(),
		NotAfter:    time.Now().Add(100 * 365 * 24 * time.Hour), // Valid for 100 years
		KeyUsage:    x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}

	// Sign the client certificate with the CA
	clientCertDER, err := x509.CreateCertificate(rand.Reader, &clientCertTemplate, caCert, &clientKey.PublicKey, caKey)
	if err != nil {
		http.Error(w, "Failed to sign client certificate", http.StatusInternalServerError)
		log.Printf("Error signing client certificate: %v", err)
		return
	}

	// Convert client certificate to PEM format
	clientCertPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: clientCertDER})

	// Convert client private key to PEM format
	clientKeyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(clientKey)})

	// Send the client key and certificate in PEM format as plain text
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	w.Write(clientKeyPEM)
	w.Write(clientCertPEM)

	log.Println("Generated and returned new client key and certificate.")
}
