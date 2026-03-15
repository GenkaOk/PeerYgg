package main

import (
	"encoding/json"
	"io/ioutil"
	"os"
)

func LoadLocal(path string) ([]string, error) {
	b, err := ioutil.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, err
	}
	return ParsePeersFromJSON(b)
}

func SaveLocal(path string, peers []string) error {
	b, err := json.MarshalIndent(peers, "", "  ")
	if err != nil {
		return err
	}
	return ioutil.WriteFile(path, b, 0644)
}

func FetchURL(url string) ([]byte, error) {
	return fetchURLInternal(url)
}

func ReadStdin() (string, error) {
	stat, err := os.Stdin.Stat()
	if err != nil {
		return "", err
	}
	if (stat.Mode() & os.ModeCharDevice) == 0 {
		b, err := ioutil.ReadAll(os.Stdin)
		return string(b), err
	}
	return "", nil
}
