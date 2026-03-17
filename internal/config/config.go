package config

import (
	"flag"
	"os"
	"strings"
)

const (
	DefaultURL          = "https://raw.githubusercontent.com/GenkaOk/public-peers/refs/heads/master/nodes.json"
	LocalStore          = "peers.json"
	HTTPTimeoutSecs     = 1
	DefaultConcurrency  = 30
	DefaultTimeoutSec   = 1
	DefaultTraceCount   = 5
	DefaultTraceMaxHops = 20
	DefaultTraceTimeout = 30
)

type ProgressType int

const (
	WithoutProgress ProgressType = iota
	SimpleProgress
	FullProgress
)

type OutputFormat string

const (
	OutputCurrent OutputFormat = "current"
	OutputJSON    OutputFormat = "json"
	OutputTable   OutputFormat = "table"
	OutputConfig  OutputFormat = "config"
)

type Config struct {
	URL          string
	Store        string
	AddCmd       string
	RemoveCmd    string
	DryRun       bool
	Concurrency  int
	TimeoutSec   int
	TopN         int
	GroupByHost  bool
	ProgressType ProgressType
	OutputFormat OutputFormat
	InsecureSSL  bool

	TraceCount   int
	TraceMaxHops int
	TraceTimeout int
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func Load() *Config {
	n := flag.Int("n", 5, "number of fastest peers/servers to output")
	concurrency := flag.Int("c", DefaultConcurrency, "concurrency for pings")
	timeout := flag.Int("t", DefaultTimeoutSec, "timeout per ping in seconds")
	groupByHost := flag.Bool("group", false, "group peers by host and select best connection per server")
	progressMode := flag.String("progress", "full", "progress mode: [n]one|[s]imple|[f]ull")
	outputFormat := flag.String("output", "current", "output format: current|json|table|config")

	traceCount := flag.Int("trace-count", DefaultTraceCount, "tracing count peers to calculate hops, 0 for disable trace calculate")
	traceHops := flag.Int("trace-max-hops", DefaultTraceMaxHops, "max hops count for calculate")
	traceTimeout := flag.Int("trace-timeout", DefaultTraceTimeout, "timeout in seconds for tracing all peers")

	insecureSsl := flag.Bool("insecure", false, "allow skip SSL verification")

	flag.Parse()

	pType := parseProgressMode(*progressMode)
	format := parseOutputFormat(*outputFormat)

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
		ProgressType: pType,
		OutputFormat: format,
		InsecureSSL:  *insecureSsl,

		TraceCount:   *traceCount,
		TraceMaxHops: *traceHops,
		TraceTimeout: *traceTimeout,
	}
}

func parseProgressMode(mode string) ProgressType {
	mode = strings.ToLower(mode)

	switch mode {
	case "none":
		return WithoutProgress
	case "f", "fu", "ful", "full":
		return FullProgress
	case "s", "si", "sim", "simp", "simpl", "simple":
		return SimpleProgress
	default:
		return SimpleProgress
	}
}

func parseOutputFormat(format string) OutputFormat {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "json":
		return OutputJSON
	case "table":
		return OutputTable
	case "config":
		return OutputConfig
	case "current":
		fallthrough
	default:
		return OutputCurrent
	}
}
