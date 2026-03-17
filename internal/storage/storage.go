package storage

import (
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
	client := &http.Client{
		Timeout: time.Duration(config.HTTPTimeoutSecs) * time.Second,
	}

	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
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
