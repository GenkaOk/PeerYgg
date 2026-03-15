package main

import (
	"encoding/json"
	"time"
)

const (
	DefaultURL         = "https://raw.githubusercontent.com/GenkaOk/public-peers/refs/heads/master/nodes.json"
	LocalStore         = "peers.json"
	HTTPTimeoutSecs    = 1
	DefaultConcurrency = 30
	DefaultTimeoutSec  = 1
)

type ProgressType int

const (
	WithoutProgress ProgressType = iota
	SimpleProgress
	FullProgress
)

type PeerSource struct {
	Servers []PeerServer `json:"servers"`
	RawJSON json.RawMessage
}

type PeerServer struct {
	Region  string   `json:"region"`
	Country string   `json:"country"`
	Peers   []string `json:"peers"`
}

type PeerInfo struct {
	Peer     string `json:"peer"`
	Region   string `json:"region"`
	Country  string `json:"country"`
	ServerID string `json:"server_id"`
}

type Result struct {
	Peer    string        `json:"peer"`
	Latency time.Duration `json:"latency_ms"`
	Host    string        `json:"host"`
	Scheme  string        `json:"scheme"`
	Region  string        `json:"region"`
	Country string        `json:"country"`
}

type ServerGroup struct {
	Host        string        `json:"host"`
	BestLatency time.Duration `json:"best_latency_ms"`
	Region      string        `json:"region"`
	Country     string        `json:"country"`
	Connections []Connection  `json:"connections"`
}

type Connection struct {
	Peer    string        `json:"peer"`
	Scheme  string        `json:"scheme"`
	Latency time.Duration `json:"latency_ms"`
	Region  string        `json:"region"`
	Country string        `json:"country"`
}

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
}

type ProgressTracker struct {
	Total        int
	Completed    int
	Successful   int
	Failed       int
	startTime    time.Time
	progressType ProgressType
}
