package main

import (
	"flag"
	"os"
)

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func LoadConfig() *Config {
	n := flag.Int("n", 10, "number of fastest peers/servers to output")
	concurrency := flag.Int("c", DefaultConcurrency, "concurrency for pings")
	timeout := flag.Int("t", DefaultTimeoutSec, "timeout per ping in seconds")
	groupByHost := flag.Bool("group", false, "group peers by host and select best connection per server")
	flag.Parse()

	return &Config{
		URL:         getEnv("PEERS_URL", DefaultURL),
		Store:       getEnv("PEERS_STORE", LocalStore),
		AddCmd:      getEnv("PEERS_ADD_CMD", ""),
		RemoveCmd:   getEnv("PEERS_REMOVE_CMD", ""),
		DryRun:      getEnv("DRY_RUN", "") == "1",
		Concurrency: *concurrency,
		TimeoutSec:  *timeout,
		TopN:        *n,
		GroupByHost: *groupByHost,
	}
}
