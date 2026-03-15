package main

import (
	"fmt"
	"os"
	"strings"
)

func writeResultsTable(results []Result) {
	fmt.Fprintln(os.Stdout, "N | LATENCY_MS | SCHEME | REGION | COUNTRY | HOST | PEER")
	fmt.Fprintln(os.Stdout, strings.Repeat("-", 120))
	for i, r := range results {
		fmt.Fprintf(
			os.Stdout,
			"%d | %d | %s | %s | %s | %s | %s\n",
			i+1,
			r.Latency.Milliseconds(),
			r.Scheme,
			r.Region,
			r.Country,
			r.Host,
			r.Peer,
		)
	}
	fmt.Fprintln(os.Stdout, "N | LATENCY_MS | SCHEME | REGION | COUNTRY | HOST | PEER")
}

func writeGroupsTable(groups []ServerGroup) {
	fmt.Fprintln(os.Stdout, "N | LATENCY_MS | REGION | COUNTRY | HOST | CONNECTIONS | BEST_PEER")
	fmt.Fprintln(os.Stdout, strings.Repeat("-", 120))
	for i, g := range groups {
		bestPeer := ""
		if len(g.Connections) > 0 {
			bestPeer = g.Connections[0].Peer
		}
		fmt.Fprintf(
			os.Stdout,
			"%d | %d | %s | %s | %s | %d | %s\n",
			i+1,
			g.BestLatency.Milliseconds(),
			g.Region,
			g.Country,
			g.Host,
			len(g.Connections),
			bestPeer,
		)
	}
}
