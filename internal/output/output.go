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
		writeGroupsCurrent(network.BuildServerGroups(groupedSelected), extractPeers(groupedSelected))
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
	showMarks := false
	for _, r := range results {
		if r.Preferred || r.Nearby {
			showMarks = true
			break
		}
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	if showMarks {
		fmt.Fprintln(w, "PEER\tLATENCY\tPREF\tREGION\tCOUNTRY")
	} else {
		fmt.Fprintln(w, "PEER\tLATENCY\tREGION\tCOUNTRY")
	}

	for _, r := range results {
		if showMarks {
			fmt.Fprintf(w, "%s\t%v\t%s\t%s\t%s\n", r.Peer, r.Latency, resultMark(r.Preferred, r.Nearby), r.Region, r.Country)
			continue
		}
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

func extractPeers(results []network.Result) []string {
	out := make([]string, 0, len(results))
	for _, r := range results {
		out = append(out, r.Peer)
	}
	return out
}

func writeResultsCurrent(results []network.Result) {
	fmt.Fprintln(os.Stderr, "=== Top Peers ===")
	for i, r := range results {
		hopsInfo := ""
		if r.Hops > 0 {
			hopsInfo = fmt.Sprintf(" (%d hops)", r.Hops)
		}
		fmt.Fprintf(
			os.Stderr,
			"%2d. %s (%s) — %d ms%s [%s, %s]%s\n",
			i+1,
			r.Peer,
			r.Scheme,
			r.Latency.Milliseconds(),
			hopsInfo,
			r.Region,
			r.Country,
			markSuffix(resultMark(r.Preferred, r.Nearby)),
		)
	}
	fmt.Fprintln(os.Stderr)

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	_ = encoder.Encode(extractPeers(results))
}

func writeGroupsCurrent(groups []network.ServerGroup, outPeers []string) {
	fmt.Fprintln(os.Stderr, "=== Top Servers (Grouped by Host) ===")
	for i, g := range groups {
		fmt.Fprintf(
			os.Stderr,
			"%2d. %s — %d ms (%d connections) [%s, %s]%s\n",
			i+1,
			g.Host,
			g.BestLatency.Milliseconds(),
			len(g.Connections),
			g.Region,
			g.Country,
			markSuffix(resultMark(g.Preferred, g.Nearby)),
		)
		for j, conn := range g.Connections {
			hopsInfo := ""
			if conn.Hops > 0 {
				hopsInfo = fmt.Sprintf(" (%d hops)", conn.Hops)
			}
			fmt.Fprintf(
				os.Stderr,
				"    [%d] %s (%s) — %d ms%s [%s, %s]%s\n",
				j+1,
				conn.Peer,
				conn.Scheme,
				conn.Latency.Milliseconds(),
				hopsInfo,
				conn.Region,
				conn.Country,
				markSuffix(resultMark(conn.Preferred, conn.Nearby)),
			)
		}
	}
	fmt.Fprintln(os.Stderr)

	fmt.Fprintln(os.Stderr, "=== Best Peer per Server (Output) ===")
	for i, p := range outPeers {
		fmt.Fprintf(os.Stderr, "%2d. %s\n", i+1, p)
	}
	fmt.Fprintln(os.Stderr)

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	_ = encoder.Encode(outPeers)
}

func markSuffix(mark string) string {
	if mark == "" {
		return ""
	}
	return " " + mark
}
