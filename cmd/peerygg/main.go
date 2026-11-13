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

	preferredCountry, preferredRegion := "", ""
	if cfg.Country != "" {
		var err error
		preferredCountry, preferredRegion, err = peer.ResolveCountry(source, cfg.Country)
		if err != nil {
			fmt.Fprintln(os.Stderr, "failed resolve country:", err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "Preferred country: %s (%s), extra peers: %d\n", preferredCountry, preferredRegion, cfg.CountryCount)
	}

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

	selected, stats := network.SelectPreferred(
		results,
		cfg.TopN,
		cfg.CountryCount,
		preferredCountry,
		preferredRegion,
		cfg.GroupByHost,
	)
	if cfg.Country != "" {
		fmt.Fprintf(os.Stderr, "Preferred %s: %d of %d live peers selected", stats.Country, stats.TargetSelected, stats.TargetLive)
		if stats.NearbyAdded > 0 {
			fmt.Fprintf(os.Stderr, " (+%d nearby from %s)", stats.NearbyAdded, stats.Region)
		}
		fmt.Fprintln(os.Stderr)
	}

	traceCount := cfg.TraceCount
	if traceCount < 0 {
		traceCount = 0
	}
	if len(results) < traceCount {
		traceCount = len(results)
	}

	var traceIndexes []int
	if traceCount > 0 {
		traceExtra := make(map[string]struct{}, len(stats.AddedPeers))
		for _, p := range stats.AddedPeers {
			traceExtra[p] = struct{}{}
		}
		traceIndexes = make([]int, 0, traceCount+len(traceExtra))
		for i := range results {
			if i < traceCount {
				traceIndexes = append(traceIndexes, i)
				continue
			}
			if _, ok := traceExtra[results[i].Peer]; ok {
				traceIndexes = append(traceIndexes, i)
			}
		}
	}

	if len(traceIndexes) > 0 {
		if len(traceIndexes) == traceCount {
			fmt.Fprintf(os.Stderr, "Tracing top %d peers to calculate hops...\n", traceCount)
		} else {
			fmt.Fprintf(os.Stderr, "Tracing %d peers to calculate hops (including preferred)...\n", len(traceIndexes))
		}
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.TraceTimeout)*time.Second)
		defer cancel()

		var wg sync.WaitGroup
		var completed atomic.Int32

		for _, idx := range traceIndexes {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				results[idx].Hops = network.TraceHops(ctx, results[idx].Host, cfg.TraceMaxHops)
				count := completed.Add(1)
				fmt.Fprintf(os.Stderr, "  [%d/%d] %s: %d hops\n", count, len(traceIndexes), results[idx].Host, results[idx].Hops)
			}(idx)
		}
		wg.Wait()
		fmt.Fprintln(os.Stderr)

		byPeer := make(map[string]int, len(results))
		for i := range results {
			byPeer[results[i].Peer] = i
		}
		for i := range selected {
			if idx, ok := byPeer[selected[i].Peer]; ok {
				selected[i].Hops = results[idx].Hops
			}
		}
	}

	if cfg.GroupByHost {
		output.PrintGroupedResults(results, selected, cfg.OutputFormat)
	} else {
		output.PrintResults(results, selected, cfg.OutputFormat)
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
