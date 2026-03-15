package main

import (
	"encoding/json"
	"os"
)

func writeResultsJSON(results []Result) {
	type jsonResult struct {
		Peer      string `json:"peer"`
		Host      string `json:"host"`
		Scheme    string `json:"scheme"`
		LatencyMS int64  `json:"latency_ms"`
		Region    string `json:"region"`
		Country   string `json:"country"`
	}

	out := make([]jsonResult, 0, len(results))
	for _, r := range results {
		out = append(out, jsonResult{
			Peer:      r.Peer,
			Host:      r.Host,
			Scheme:    r.Scheme,
			LatencyMS: r.Latency.Milliseconds(),
			Region:    r.Region,
			Country:   r.Country,
		})
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(out)
}

func writeGroupsJSON(groups []ServerGroup) {
	type jsonConnection struct {
		Peer      string `json:"peer"`
		Scheme    string `json:"scheme"`
		LatencyMS int64  `json:"latency_ms"`
		Region    string `json:"region"`
		Country   string `json:"country"`
	}
	type jsonGroup struct {
		Host          string           `json:"host"`
		BestLatencyMS int64            `json:"best_latency_ms"`
		Region        string           `json:"region"`
		Country       string           `json:"country"`
		Connections   []jsonConnection `json:"connections"`
	}

	out := make([]jsonGroup, 0, len(groups))
	for _, g := range groups {
		connections := make([]jsonConnection, 0, len(g.Connections))
		for _, c := range g.Connections {
			connections = append(connections, jsonConnection{
				Peer:      c.Peer,
				Scheme:    c.Scheme,
				LatencyMS: c.Latency.Milliseconds(),
				Region:    c.Region,
				Country:   c.Country,
			})
		}

		out = append(out, jsonGroup{
			Host:          g.Host,
			BestLatencyMS: g.BestLatency.Milliseconds(),
			Region:        g.Region,
			Country:       g.Country,
			Connections:   connections,
		})
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(out)
}
