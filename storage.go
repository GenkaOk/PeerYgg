package main

import (
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

	src, err := ParsePeerSourceJSON(b)
	if err != nil {
		return nil, err
	}
	return ExtractPeerStrings(FlattenPeerSource(src)), nil
}

func SaveLocal(path string, raw []byte) error {
	return ioutil.WriteFile(path, raw, 0644)
}

func FetchURL(url string, cfg *Config) ([]byte, error) {
	return fetchURLInternal(url, cfg)
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
