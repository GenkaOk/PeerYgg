package main

import (
	"encoding/json"
	"fmt"
	"os"
)

func writeConfigPeers(results []Result) {
	b, err := json.Marshal(extractPeersFromResults(results))
	if err != nil {
		fmt.Fprintln(os.Stderr, "failed encode config peers:", err)
		return
	}

	// Need fix for go 1.20
	//minLatency := 999
	//maxLatency := 0

	//for _, row := range results {
	//	minLatency = min(minLatency, int(row.Latency.Milliseconds()))
	//	maxLatency = max(maxLatency, int(row.Latency.Milliseconds()))
	//}

	//fmt.Fprintf(os.Stdout, "Latency: from %d ms to %d ms\n\n", minLatency, maxLatency)
	fmt.Fprintf(os.Stdout, "Peers: %s\n", string(b))
}
