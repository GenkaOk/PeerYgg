package storage

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/genkaok/PeerYgg/internal/config"
)

func certPEM(server *httptest.Server) []byte {
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw})
}

func assertPoolTrusts(t *testing.T, pool *x509.CertPool, server *httptest.Server) {
	t.Helper()
	conn, err := tls.Dial("tcp", server.Listener.Addr().String(), &tls.Config{RootCAs: pool})
	if err != nil {
		t.Fatalf("tls.Dial with merged roots: %v", err)
	}
	_ = conn.Close()
}

func TestLoadRootCAsIncludesExtraFile(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer server.Close()

	path := filepath.Join(t.TempDir(), "ca.pem")
	if err := os.WriteFile(path, certPEM(server), 0600); err != nil {
		t.Fatal(err)
	}

	pool := loadRootCAs([]string{path}, nil)
	assertPoolTrusts(t, pool, server)
}

func TestLoadRootCAsIncludesExtraDir(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer server.Close()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "ca.pem"), certPEM(server), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("not a certificate"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "subdir"), 0700); err != nil {
		t.Fatal(err)
	}

	pool := loadRootCAs(nil, []string{dir})
	assertPoolTrusts(t, pool, server)
}

func TestRootCAsHonorsSSLCertFile(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer server.Close()

	path := filepath.Join(t.TempDir(), "ca.pem")
	if err := os.WriteFile(path, certPEM(server), 0600); err != nil {
		t.Fatal(err)
	}

	t.Setenv("SSL_CERT_FILE", path)
	t.Setenv("SSL_CERT_DIR", "")

	pool := rootCAs()
	if pool == nil {
		t.Fatal("rootCAs returned nil for SSL_CERT_FILE override")
	}
	assertPoolTrusts(t, pool, server)
}

func TestRootCAsHonorsSSLCertDir(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer server.Close()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "ca.pem"), certPEM(server), 0600); err != nil {
		t.Fatal(err)
	}

	t.Setenv("SSL_CERT_FILE", "")
	t.Setenv("SSL_CERT_DIR", dir)

	pool := rootCAs()
	if pool == nil {
		t.Fatal("rootCAs returned nil for SSL_CERT_DIR override")
	}
	assertPoolTrusts(t, pool, server)
}

func TestLoadRootCAsIgnoresMissingAndBroken(t *testing.T) {
	dir := t.TempDir()
	bad := filepath.Join(dir, "bad.pem")
	if err := os.WriteFile(bad, []byte("-----BEGIN CERTIFICATE-----\nnope\n-----END CERTIFICATE-----"), 0600); err != nil {
		t.Fatal(err)
	}

	pool := loadRootCAs(
		[]string{filepath.Join(dir, "missing.pem"), bad},
		[]string{filepath.Join(dir, "missing-dir")},
	)
	if pool == nil {
		t.Fatal("loadRootCAs returned nil pool")
	}
}

func TestAppendPEMDeduplicates(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer server.Close()

	pool := x509.NewCertPool()
	seen := make(map[[32]byte]struct{})
	data := certPEM(server)

	if added := appendPEM(pool, data, seen); added != 1 {
		t.Fatalf("first append added %d certs, want 1", added)
	}
	if added := appendPEM(pool, data, seen); added != 0 {
		t.Fatalf("second append added %d certs, want 0", added)
	}
	if got := len(pool.Subjects()); got != 1 {
		t.Fatalf("pool contains %d certs, want 1", got)
	}
}

func TestFetchURLUnknownAuthorityHint(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer server.Close()

	_, err := FetchURL(server.URL, &config.Config{})
	if err == nil {
		t.Fatal("FetchURL without InsecureSSL should reject a self-signed certificate")
	}
	if !strings.Contains(err.Error(), "SSL_CERT_FILE") {
		t.Fatalf("error should hint at SSL_CERT_FILE, got %v", err)
	}
}
