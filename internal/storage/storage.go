package storage

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/genkaok/PeerYgg/internal/config"
	"github.com/genkaok/PeerYgg/internal/peer"
)

func LoadLocal(path string) ([]string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, err
	}

	src, err := peer.ParseSourceJSON(b)
	if err != nil {
		return nil, err
	}
	return peer.ExtractPeerStrings(peer.FlattenSource(src)), nil
}

func SaveLocal(path string, raw []byte) error {
	return os.WriteFile(path, raw, 0644)
}

func FetchURL(url string, cfg *config.Config) ([]byte, error) {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if cfg != nil && cfg.InsecureSSL {
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec // explicit -insecure flag
	} else if roots := rootCAs(); roots != nil {
		transport.TLSClientConfig = &tls.Config{RootCAs: roots}
	}

	client := &http.Client{
		Timeout:   time.Duration(config.HTTPTimeoutSecs) * time.Second,
		Transport: transport,
	}

	resp, err := client.Get(url)
	if err != nil {
		return nil, withCAHint(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}

func withCAHint(err error) error {
	var unknown x509.UnknownAuthorityError
	if errors.As(err, &unknown) {
		return fmt.Errorf("%w (no trusted CA bundle found; install ca-certificates, e.g. \"opkg install ca-certificates\", or set SSL_CERT_FILE)", err)
	}
	return err
}

func ReadStdin() (string, error) {
	stat, err := os.Stdin.Stat()
	if err != nil {
		return "", err
	}
	if (stat.Mode() & os.ModeCharDevice) == 0 {
		b, err := io.ReadAll(os.Stdin)
		return string(b), err
	}
	return "", nil
}
