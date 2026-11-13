package storage

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/genkaok/PeerYgg/internal/config"
)

func TestFetchURLInsecure(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}))
	defer server.Close()

	if _, err := FetchURL(server.URL, &config.Config{}); err == nil {
		t.Fatal("FetchURL without InsecureSSL should reject a self-signed certificate")
	}

	body, err := FetchURL(server.URL, &config.Config{InsecureSSL: true})
	if err != nil {
		t.Fatalf("FetchURL with InsecureSSL = true: %v", err)
	}
	if string(body) != "ok" {
		t.Fatalf("body = %q, want ok", body)
	}
}
