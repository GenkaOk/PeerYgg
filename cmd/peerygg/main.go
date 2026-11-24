package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/genkaok/PeerYgg/internal/config"
	"github.com/genkaok/PeerYgg/internal/network"
	"github.com/genkaok/PeerYgg/internal/output"
	"github.com/genkaok/PeerYgg/internal/peer"
	"github.com/genkaok/PeerYgg/internal/storage"
	"github.com/genkaok/PeerYgg/internal/yggconf"
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

	// The results are already printed, so a failed install can exit without
	// invalidating anything the user has seen.
	if cfg.Install {
		if err := installPeers(cfg, selected); err != nil {
			fmt.Fprintln(os.Stderr, "failed install peers:", err)
			os.Exit(1)
		}
	}
}

// selectedPeers returns the full peer URIs of selected results, in order.
func selectedPeers(selected []network.Result) []string {
	peers := make([]string, 0, len(selected))
	for _, r := range selected {
		peers = append(peers, r.Peer)
	}
	return peers
}

// installPeers writes the selected peers into the yggdrasil config named by
// cfg and reports the result on stderr.
func installPeers(cfg *config.Config, selected []network.Result) error {
	peers := selectedPeers(selected)
	// Clearing the peer list is recoverable but disruptive, so a real write
	// refuses it. A dry run is the one place that change is harmless: it lets
	// Install report the diff without touching anything.
	if len(peers) == 0 && !cfg.DryRun {
		return fmt.Errorf("no peers selected, refusing to empty the yggdrasil Peers list")
	}

	path, err := yggconf.Detect(cfg.ConfPath)
	if err != nil {
		return err
	}

	rep, err := yggconf.Install(yggconf.Options{
		Path:   path,
		Peers:  peers,
		DryRun: cfg.DryRun,
	})
	if err != nil {
		// A failed install can still have made the backup; tell the user where
		// it is before reporting the error.
		printInstallFailure(os.Stderr, rep)
		return err
	}

	printInstallReport(rep)
	return nil
}

// printInstallReport prints what installPeers did, or would do on a dry run.
func printInstallReport(rep *yggconf.Report) {
	writeInstallReport(os.Stderr, rep)
}

// writeInstallReport renders a successful install report. A dry run and an
// unchanged no-op never wrote anything, so neither prints a restart command.
func writeInstallReport(w io.Writer, rep *yggconf.Report) {
	fmt.Fprintf(w, "yggdrasil config: %s\n", rep.Path)
	if rep.BackupPath != "" {
		fmt.Fprintf(w, "backup: %s\n", rep.BackupPath)
	}
	if rep.Unchanged {
		fmt.Fprintln(w, "already up to date")
		return
	}
	fmt.Fprintf(w, "+%d / -%d peers\n", len(rep.Added), len(rep.Removed))
	if rep.DryRun {
		fmt.Fprintln(w, "dry run: nothing written")
		return
	}
	// Install only skips validation when yggdrasil is not in PATH; an
	// unchanged config never reaches the validation step at all.
	if !rep.Validated {
		fmt.Fprintln(w, "not validated (yggdrasil not found)")
	}
	if cmd, known := yggconf.RestartCommand(runtime.GOOS); known {
		fmt.Fprintf(w, "restart: %s\n", cmd)
	} else {
		fmt.Fprintln(w, cmd)
	}
}

// printInstallFailure names the pre-install backup left on disk when an install
// fails after making it. It is silent when there is no backup to report.
func printInstallFailure(w io.Writer, rep *yggconf.Report) {
	if rep != nil && rep.BackupPath != "" {
		fmt.Fprintf(w, "backup: %s\n", rep.BackupPath)
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
