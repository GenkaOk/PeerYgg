package main

import (
	"context"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/genkaok/PeerYgg/internal/config"
	"github.com/genkaok/PeerYgg/internal/network"
	"github.com/genkaok/PeerYgg/internal/output"
	"github.com/genkaok/PeerYgg/internal/peer"
	"github.com/genkaok/PeerYgg/internal/storage"
)

func main() {
	cfg := config.Load()

	source, allPeers, err := loadPeersSource(cfg)
	if err != nil {
		fmt.Fprintln(os.Stderr, "failed load peers:", err)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "Loaded %d peers\n", len(allPeers))

	fmt.Fprintf(os.Stderr, "Starting scan with concurrency %d, timeout %ds\n", cfg.Concurrency, cfg.TimeoutSec)
	results := network.MeasureAll(
		allPeers,
		cfg.Concurrency,
		cfg.TimeoutSec,
		cfg.ProgressType,
	)

	if len(results) == 0 {
		fmt.Fprintln(os.Stderr, "No reachable peers")
		if source != nil && len(source.RawJSON) > 0 {
			_ = storage.SaveLocal(cfg.Store, source.RawJSON)
		}
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "Found %d reachable peers\n\n", len(results))

	traceCount := cfg.TraceCount
	if len(results) < traceCount {
		traceCount = len(results)
	}
	if traceCount > 0 {
		fmt.Fprintf(os.Stderr, "Tracing top %d peers to calculate hops...\n", traceCount)
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.TraceTimeout)*time.Second)
		defer cancel()

		var wg sync.WaitGroup
		var completed atomic.Int32

		for i := 0; i < traceCount; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				results[idx].Hops = network.TraceHops(ctx, results[idx].Host, cfg.TraceMaxHops)
				count := completed.Add(1)
				fmt.Fprintf(os.Stderr, "  [%d/%d] %s: %d hops\n", count, traceCount, results[idx].Host, results[idx].Hops)
			}(i)
		}
		wg.Wait()
		fmt.Fprintln(os.Stderr)
	}

	if cfg.GroupByHost {
		output.PrintGroupedResults(results, cfg.TopN, cfg.OutputFormat)
	} else {
		output.PrintResults(results, cfg.TopN, cfg.OutputFormat)
	}

	if source != nil && len(source.RawJSON) > 0 {
		if err := storage.SaveLocal(cfg.Store, source.RawJSON); err != nil {
			fmt.Fprintln(os.Stderr, "failed save local:", err)
		}
	}
}

func loadPeersSource(cfg *config.Config) (*peer.Source, []peer.Info, error) {
	stdinData, _ := storage.ReadStdin()
	if stdinData != "" {
		src, err := peer.ParseSourceJSON([]byte(stdinData))
		if err != nil {
			return nil, nil, err
		}
		return src, peer.FlattenSource(src), nil
	}

	fmt.Fprintln(os.Stderr, "Fetching peers from URL...")
	b, err := storage.FetchURL(cfg.URL, cfg)
	if err != nil {
		return nil, nil, err
	}

	src, err := peer.ParseSourceJSON(b)
	if err != nil {
		return nil, nil, err
	}
	return src, peer.FlattenSource(src), nil
}
