package main

import (
	"encoding/json"
	"fmt"
	"os"
)

func main() {
	cfg := LoadConfig()

	source, allPeers, err := loadPeersSource(cfg)
	if err != nil {
		fmt.Fprintln(os.Stderr, "failed load peers:", err)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "Loaded %d peers\n", len(allPeers))

	// Handle local storage and diff
	//if err := handleLocalStorage(cfg, ExtractPeerStrings(allPeers)); err != nil {
	//	fmt.Fprintln(os.Stderr, "failed handle storage:", err)
	//	os.Exit(1)
	//}

	fmt.Fprintf(os.Stderr, "Starting scan with concurrency %d, timeout %ds\n", cfg.Concurrency, cfg.TimeoutSec)
	results := MeasureAll(
		allPeers,
		cfg.Concurrency,
		cfg.TimeoutSec,
		cfg.ProgressType,
	)

	if len(results) == 0 {
		fmt.Fprintln(os.Stderr, "No reachable peers")
		if source != nil && len(source.RawJSON) > 0 {
			_ = SaveLocal(cfg.Store, source.RawJSON)
		}
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "Found %d reachable peers\n\n", len(results))

	if cfg.GroupByHost {
		printGroupedResults(results, cfg.TopN)
	} else {
		printResults(results, cfg.TopN)
	}

	if source != nil && len(source.RawJSON) > 0 {
		if err := SaveLocal(cfg.Store, source.RawJSON); err != nil {
			fmt.Fprintln(os.Stderr, "failed save local:", err)
		}
	}
}

func loadPeersSource(cfg *Config) (*PeerSource, []PeerInfo, error) {
	stdinData, _ := ReadStdin()
	if stdinData != "" {
		src, err := ParsePeerSourceJSON([]byte(stdinData))
		if err != nil {
			return nil, nil, err
		}
		return src, FlattenPeerSource(src), nil
	}

	fmt.Fprintln(os.Stderr, "Fetching peers from URL...")
	b, err := FetchURL(cfg.URL)
	if err != nil {
		return nil, nil, err
	}

	src, err := ParsePeerSourceJSON(b)
	if err != nil {
		return nil, nil, err
	}
	return src, FlattenPeerSource(src), nil
}

func handleLocalStorage(cfg *Config, allPeers []string) error {
	localPeers, err := LoadLocal(cfg.Store)
	if err != nil {
		return err
	}

	toAdd, toRemove := DiffPeers(localPeers, allPeers)
	if len(toAdd) > 0 || len(toRemove) > 0 {
		fmt.Fprintln(os.Stderr, "Changes detected:")
		fmt.Fprintf(os.Stderr, "  To add: %d peers\n", len(toAdd))
		fmt.Fprintf(os.Stderr, "  To remove: %d peers\n", len(toRemove))

		if cfg.DryRun {
			fmt.Fprintln(os.Stderr, "[DRY RUN MODE]")
			for _, p := range toAdd {
				fmt.Fprintf(os.Stderr, "  [+] %s\n", p)
			}
			for _, p := range toRemove {
				fmt.Fprintf(os.Stderr, "  [-] %s\n", p)
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

	fmt.Fprintln(os.Stderr, "=== Top Peers ===")
	for i, r := range topResults {
		fmt.Fprintf(
			os.Stderr,
			"%2d. %s (%s) — %d ms [%s, %s]\n",
			i+1,
			r.Peer,
			r.Scheme,
			r.Latency.Milliseconds(),
			r.Region,
			r.Country,
		)
	}
	fmt.Fprintln(os.Stderr)

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

	fmt.Fprintln(os.Stderr, "=== Top Servers (Grouped by Host) ===")
	for i, group := range topGroups {
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
			fmt.Fprintf(
				os.Stderr,
				"    [%d] %s (%s) — %d ms [%s, %s]\n",
				j+1,
				conn.Peer,
				conn.Scheme,
				conn.Latency.Milliseconds(),
				conn.Region,
				conn.Country,
			)
		}
	}
	fmt.Fprintln(os.Stderr)

	outPeers := GetBestPeersPerServer(topGroups, topN)

	fmt.Fprintln(os.Stderr, "=== Best Peer per Server (Output) ===")
	for i, p := range outPeers {
		fmt.Fprintf(os.Stderr, "%2d. %s\n", i+1, p)
	}
	fmt.Fprintln(os.Stderr)

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(outPeers)
}
