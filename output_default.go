package main

import (
	"encoding/json"
	"fmt"
	"os"
)

func writeResultsCurrent(results []Result) {
	fmt.Fprintln(os.Stderr, "=== Top Peers ===")
	for i, r := range results {
		hopsInfo := ""
		if r.Hops > 0 {
			hopsInfo = fmt.Sprintf(" (%d hops)", r.Hops)
		}
		fmt.Fprintf(
			os.Stderr,
			"%2d. %s (%s) — %d ms%s [%s, %s]\n",
			i+1,
			r.Peer,
			r.Scheme,
			r.Latency.Milliseconds(),
			hopsInfo,
			r.Region,
			r.Country,
		)
	}
	fmt.Fprintln(os.Stderr)

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(extractPeersFromResults(results))
}

func writeGroupsCurrent(groups []ServerGroup, topN int) {
	fmt.Fprintln(os.Stderr, "=== Top Servers (Grouped by Host) ===")
	for i, group := range groups {
		fmt.Fprintf(
			os.Stderr,
			"%2d. %s — %d ms (%d connections) [%s, %s]\n",
			i+1,
			group.Host,
			group.BestLatency.Milliseconds(),
			len(group.Connections),
			group.Region,
			group.Country,
		)
		for j, conn := range group.Connections {
			hopsInfo := ""
			if conn.Hops > 0 {
				hopsInfo = fmt.Sprintf(" (%d hops)", conn.Hops)
			}
			fmt.Fprintf(
				os.Stderr,
				"    [%d] %s (%s) — %d ms%s [%s, %s]\n",
				j+1,
				conn.Peer,
				conn.Scheme,
				conn.Latency.Milliseconds(),
				hopsInfo,
				conn.Region,
				conn.Country,
			)
		}
	}
	fmt.Fprintln(os.Stderr)

	outPeers := GetBestPeersPerServer(groups, topN)

	fmt.Fprintln(os.Stderr, "=== Best Peer per Server (Output) ===")
	for i, p := range outPeers {
		fmt.Fprintf(os.Stderr, "%2d. %s\n", i+1, p)
	}
	fmt.Fprintln(os.Stderr)

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(outPeers)
}
