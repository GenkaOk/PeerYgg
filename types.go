package main

import "time"

const (
	DefaultURL         = "https://raw.githubusercontent.com/GenkaOk/public-peers/refs/heads/master/nodes.json"
	LocalStore         = "peers.json"
	HTTPTimeoutSecs    = 1
	DefaultConcurrency = 30
	DefaultTimeoutSec  = 1
)

type Result struct {
	Peer    string        `json:"peer"`
	Latency time.Duration `json:"latency_ms"`
	Host    string        `json:"host"` // IP или hostname
	Scheme  string        `json:"scheme"`
}

type ServerGroup struct {
	Host        string        `json:"host"`
	BestLatency time.Duration `json:"best_latency_ms"`
	Connections []Connection  `json:"connections"`
}

type Connection struct {
	Peer    string        `json:"peer"`
	Scheme  string        `json:"scheme"`
	Latency time.Duration `json:"latency_ms"`
}

type Config struct {
	URL         string
	Store       string
	AddCmd      string
	RemoveCmd   string
	DryRun      bool
	Concurrency int
	TimeoutSec  int
	TopN        int
	GroupByHost bool // Группировать по хосту
}
