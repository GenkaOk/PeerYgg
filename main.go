package main

import (
	"context"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"time"
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

	// Выполняем трассировку для топ-10 результатов
	traceCount := cfg.TraceCount
	if len(results) < traceCount {
		traceCount = len(results)
	}
	if traceCount > 0 {
		fmt.Fprintf(os.Stderr, "Tracing top %d peers to calculate hops...\n", traceCount)
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second) // Уменьшаем общий таймаут, так как теперь параллельно
		defer cancel()

		var wg sync.WaitGroup
		var completed int32

		for i := range traceCount {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				results[idx].Hops = TraceHops(ctx, results[idx].Host)
				count := atomic.AddInt32(&completed, 1)
				fmt.Fprintf(os.Stderr, "  [%d/%d] %s: %d hops\n", count, traceCount, results[idx].Host, results[idx].Hops)
			}(i)
		}
		wg.Wait()
		fmt.Fprintln(os.Stderr)
	}

	if cfg.GroupByHost {
		printGroupedResults(results, cfg.TopN, cfg.OutputFormat)
	} else {
		printResults(results, cfg.TopN, cfg.OutputFormat)
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
