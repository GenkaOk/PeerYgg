package output

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/genkaok/PeerYgg/internal/config"
	"github.com/genkaok/PeerYgg/internal/network"
)

func PrintResults(results []network.Result, topN int, format config.OutputFormat) {
	topResults := limitResults(results, topN)

	switch format {
	case config.OutputJSON:
		writeResultsJSON(results)
	case config.OutputTable:
		writeResultsTable(results)
	case config.OutputConfig:
		writeConfigPeers(topResults)
	default:
		writeResultsCurrent(topResults)
	}
}

func PrintGroupedResults(results []network.Result, topN int, format config.OutputFormat) {
	groupedResults := network.GroupByHost(results)
	topGroupedResults := limitResults(groupedResults, topN)

	switch format {
	case config.OutputJSON:
		writeResultsJSON(groupedResults)
	case config.OutputTable:
		writeResultsTable(groupedResults)
	case config.OutputConfig:
		writeConfigPeers(topGroupedResults)
	default:
		writeGroupsCurrent(network.BuildServerGroups(topGroupedResults), topN)
	}
}

func limitResults(results []network.Result, topN int) []network.Result {
	top := topN
	if top > len(results) {
		top = len(results)
	}
	return results[:top]
}

func writeResultsJSON(results any) {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	_ = encoder.Encode(results)
}

func writeResultsTable(results []network.Result) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "PEER\tLATENCY\tREGION\tCOUNTRY")
	for _, r := range results {
		fmt.Fprintf(w, "%s\t%v\t%s\t%s\n", r.Peer, r.Latency, r.Region, r.Country)
	}
	w.Flush()
}

func writeConfigPeers(results []network.Result) {
	var peers []string
	for _, r := range results {
		peers = append(peers, r.Peer)
	}
	fmt.Println(strings.Join(peers, ","))
}

func writeResultsCurrent(results []network.Result) {
	for i, r := range results {
		fmt.Printf("%d. %s (%v) [%s, %s]\n", i+1, r.Peer, r.Latency, r.Region, r.Country)
	}
}

func writeGroupsCurrent(groups []network.ServerGroup, topN int) {
	for i, g := range groups {
		if i >= topN {
			break
		}
		fmt.Printf("%d. %s (best: %v) [%s, %s]\n", i+1, g.Host, g.BestLatency, g.Region, g.Country)
		for _, conn := range g.Connections {
			fmt.Printf("   - %s (%v)\n", conn.Peer, conn.Latency)
		}
	}
}
