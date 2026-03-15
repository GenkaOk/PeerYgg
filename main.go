package main

import (
	"encoding/json"
	"fmt"
	"os"
)

func main() {
	cfg := LoadConfig()

	// Load peers from stdin or URL
	allPeers, err := loadPeersSource(cfg)
	if err != nil {
		fmt.Fprintln(os.Stderr, "failed load peers:", err)
		os.Exit(1)
	}

	// Handle local storage and diff
	if err := handleLocalStorage(cfg, allPeers); err != nil {
		fmt.Fprintln(os.Stderr, "failed handle storage:", err)
		os.Exit(1)
	}

	// Measure latencies
	fmt.Fprintf(os.Stderr, "Measuring %d peers with concurrency %d\n", len(allPeers), cfg.Concurrency)
	results := MeasureAll(allPeers, cfg.Concurrency, cfg.TimeoutSec)

	if len(results) == 0 {
		fmt.Fprintln(os.Stderr, "No reachable peers")
		_ = SaveLocal(cfg.Store, allPeers)
		os.Exit(1)
	}

	// Group by host if enabled
	if cfg.GroupByHost {
		printGroupedResults(results, cfg.TopN)
	} else {
		printResults(results, cfg.TopN)
	}

	// Save full list
	if err := SaveLocal(cfg.Store, allPeers); err != nil {
		fmt.Fprintln(os.Stderr, "failed save local:", err)
	}
}

func loadPeersSource(cfg *Config) ([]string, error) {
	stdinData, _ := ReadStdin()
	if stdinData != "" {
		return ParsePeersFromJSON([]byte(stdinData))
	}

	b, err := FetchURL(cfg.URL)
	if err != nil {
		return nil, err
	}
	return ParsePeersFromJSON(b)
}

func handleLocalStorage(cfg *Config, allPeers []string) error {
	localPeers, err := LoadLocal(cfg.Store)
	if err != nil {
		return err
	}

	toAdd, toRemove := DiffPeers(localPeers, allPeers)
	if len(toAdd) > 0 || len(toRemove) > 0 {
		fmt.Fprintln(os.Stderr, "To add:", toAdd)
		fmt.Fprintln(os.Stderr, "To remove:", toRemove)

		if cfg.DryRun {
			for _, p := range toAdd {
				fmt.Fprintf(os.Stderr, "[DRY] add: %s\n", p)
			}
			for _, p := range toRemove {
				fmt.Fprintf(os.Stderr, "[DRY] remove: %s\n", p)
			}
		}
	}
	return nil
}

func printResults(results []Result, topN int) {
	top := topN
	if top > len(results) {
		top = len(results)
	}
	topResults := results[:top]

	fmt.Fprintln(os.Stderr, "Top peers:")
	for i, r := range topResults {
		fmt.Fprintf(os.Stderr, "%2d. %s (%s) — %d ms\n", i+1, r.Peer, r.Scheme, r.Latency.Milliseconds())
	}

	outPeers := make([]string, 0, len(topResults))
	for _, r := range topResults {
		outPeers = append(outPeers, r.Peer)
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(outPeers)
}

func printGroupedResults(results []Result, topN int) {
	serverGroups := GroupByHost(results)

	top := topN
	if top > len(serverGroups) {
		top = len(serverGroups)
	}
	topGroups := serverGroups[:top]

	fmt.Fprintln(os.Stderr, "Top servers (grouped by host):")
	for i, group := range topGroups {
		fmt.Fprintf(os.Stderr, "%2d. %s — %d ms\n", i+1, group.Host, group.BestLatency.Milliseconds())
		for j, conn := range group.Connections {
			fmt.Fprintf(os.Stderr, "    [%d] %s (%s) — %d ms\n",
				j+1, conn.Peer, conn.Scheme, conn.Latency.Milliseconds())
		}
	}

	// Output JSON с лучшими пирами для каждого сервера
	outPeers := GetBestPeersPerServer(topGroups, topN)

	fmt.Fprintln(os.Stderr, "\nBest peers per server (output):")
	for i, p := range outPeers {
		fmt.Fprintf(os.Stderr, "%2d. %s\n", i+1, p)
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(outPeers)
}
