package main

import (
	"flag"
	"os"
	"strings"
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
	progressMode := flag.String("progress", "full", "progress mode: [n]one|[s]imple|[f]ull")

	flag.Parse()

	progressType := parseProgressMode(*progressMode)

	return &Config{
		URL:          getEnv("PEERS_URL", DefaultURL),
		Store:        getEnv("PEERS_STORE", LocalStore),
		AddCmd:       getEnv("PEERS_ADD_CMD", ""),
		RemoveCmd:    getEnv("PEERS_REMOVE_CMD", ""),
		DryRun:       getEnv("DRY_RUN", "") == "1",
		Concurrency:  *concurrency,
		TimeoutSec:   *timeout,
		TopN:         *n,
		GroupByHost:  *groupByHost,
		ProgressType: progressType,
	}
}

func parseProgressMode(mode string) ProgressType {
	mode = strings.ToLower(mode)

	switch mode {
	case "none":
		return WithoutProgress
	case "f":
	case "fu":
	case "ful":
	case "full":
		return FullProgress
	case "s":
	case "si":
	case "sim":
	case "simp":
	case "simpl":
	case "simple":
		return SimpleProgress
	default:
		// Значение по умолчанию, если передано неправильное значение
		return SimpleProgress
	}

	return FullProgress
}
