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

func PrintResults(all, selected []network.Result, format config.OutputFormat) {
	switch format {
	case config.OutputJSON:
		writeResultsJSON(all)
	case config.OutputTable:
		writeResultsTable(all)
	case config.OutputConfig:
		writeConfigPeers(selected)
	default:
		writeResultsCurrent(selected)
	}
}

func PrintGroupedResults(all, selected []network.Result, format config.OutputFormat) {
	groupedAll := network.GroupByHost(all)
	groupedSelected := network.GroupByHost(selected)

	switch format {
	case config.OutputJSON:
		writeResultsJSON(groupedAll)
	case config.OutputTable:
		writeResultsTable(groupedAll)
	case config.OutputConfig:
		writeConfigPeers(groupedSelected)
	default:
		writeGroupsCurrent(network.BuildServerGroups(groupedSelected))
	}
}

func writeResultsJSON(results any) {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	_ = encoder.Encode(results)
}

func resultMark(preferred, nearby bool) string {
	switch {
	case preferred:
		return "*"
	case nearby:
		return "~"
	default:
		return ""
	}
}

func writeResultsTable(results []network.Result) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "PEER\tLATENCY\tPREF\tREGION\tCOUNTRY")
	for _, r := range results {
		fmt.Fprintf(w, "%s\t%v\t%s\t%s\t%s\n", r.Peer, r.Latency, resultMark(r.Preferred, r.Nearby), r.Region, r.Country)
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
		fmt.Printf("%d. %s (%v) [%s, %s]%s\n", i+1, r.Peer, r.Latency, r.Region, r.Country, markSuffix(resultMark(r.Preferred, r.Nearby)))
	}
}

func writeGroupsCurrent(groups []network.ServerGroup) {
	for i, g := range groups {
		fmt.Printf("%d. %s (best: %v) [%s, %s]%s\n", i+1, g.Host, g.BestLatency, g.Region, g.Country, markSuffix(resultMark(g.Preferred, g.Nearby)))
		for _, conn := range g.Connections {
			fmt.Printf("   - %s (%v)%s\n", conn.Peer, conn.Latency, markSuffix(resultMark(conn.Preferred, conn.Nearby)))
		}
	}
}

func markSuffix(mark string) string {
	if mark == "" {
		return ""
	}
	return " " + mark
}
