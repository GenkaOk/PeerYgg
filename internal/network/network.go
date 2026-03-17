package network

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/genkaok/PeerYgg/internal/config"
	"github.com/genkaok/PeerYgg/internal/peer"
	"github.com/genkaok/PeerYgg/internal/progress"
)

type Result struct {
	Peer    string        `json:"peer"`
	Latency time.Duration `json:"latency_ms"`
	Hops    int           `json:"hops,omitzero"`
	Host    string        `json:"host"`
	Scheme  string        `json:"scheme"`
	Region  string        `json:"region"`
	Country string        `json:"country"`
}

type ServerGroup struct {
	Host        string        `json:"host"`
	BestLatency time.Duration `json:"best_latency_ms"`
	Region      string        `json:"region"`
	Country     string        `json:"country"`
	Connections []Connection  `json:"connections"`
}

type Connection struct {
	Peer    string        `json:"peer"`
	Scheme  string        `json:"scheme"`
	Latency time.Duration `json:"latency_ms"`
	Hops    int           `json:"hops,omitzero"`
	Region  string        `json:"region"`
	Country string        `json:"country"`
}

func ParseAddress(addr string) (host, port, scheme string, err error) {
	u, err := url.Parse(addr)
	if err != nil {
		return "", "", "", err
	}
	host, port, _ = net.SplitHostPort(u.Host)
	if host == "" {
		host = u.Host
	}
	return host, port, u.Scheme, nil
}

func normalizeHost(host string) string {
	if strings.Contains(host, ":") && !strings.HasPrefix(host, "[") {
		return "[" + host + "]"
	}
	return host
}

func DialPeer(ctx context.Context, addr string, timeout time.Duration) (time.Duration, string, string, error) {
	host, port, scheme, err := ParseAddress(addr)
	if err != nil {
		return 0, "", "", err
	}

	dialer := net.Dialer{Timeout: timeout}
	start := time.Now()
	conn, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort(normalizeHost(host), port))
	if err != nil {
		return 0, host, scheme, err
	}
	defer conn.Close()
	return time.Since(start), host, scheme, nil
}

func MeasureAll(peers []peer.Info, concurrency int, timeoutSec int, pType config.ProgressType) []Result {
	var results []Result
	var mu sync.Mutex

	tracker := progress.NewTracker(len(peers), pType)
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup

	for _, p := range peers {
		wg.Add(1)
		go func(p peer.Info) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSec)*time.Second)
			defer cancel()

			latency, host, scheme, err := DialPeer(ctx, p.Peer, time.Duration(timeoutSec)*time.Second)
			if err == nil {
				mu.Lock()
				results = append(results, Result{
					Peer:    p.Peer,
					Latency: latency,
					Host:    host,
					Scheme:  scheme,
					Region:  p.Region,
					Country: p.Country,
				})
				mu.Unlock()
				tracker.Increment(true)
			} else {
				tracker.Increment(false)
			}
		}(p)
	}

	wg.Wait()
	tracker.Finish()

	sort.Slice(results, func(i, j int) bool {
		return results[i].Latency < results[j].Latency
	})

	return results
}

func GroupByHost(results []Result) []Result {
	groups := make(map[string]Result)
	var order []string

	for _, r := range results {
		if _, ok := groups[r.Host]; !ok {
			groups[r.Host] = r
			order = append(order, r.Host)
		}
	}

	out := make([]Result, 0, len(order))
	for _, host := range order {
		out = append(out, groups[host])
	}
	return out
}

func BuildServerGroups(results []Result) []ServerGroup {
	groupsMap := make(map[string]*ServerGroup)
	var order []string

	for _, r := range results {
		if _, ok := groupsMap[r.Host]; !ok {
			groupsMap[r.Host] = &ServerGroup{
				Host:        r.Host,
				BestLatency: r.Latency,
				Region:      r.Region,
				Country:     r.Country,
				Connections: []Connection{},
			}
			order = append(order, r.Host)
		}
		groupsMap[r.Host].Connections = append(groupsMap[r.Host].Connections, Connection{
			Peer:    r.Peer,
			Scheme:  r.Scheme,
			Latency: r.Latency,
			Hops:    r.Hops,
			Region:  r.Region,
			Country: r.Country,
		})
	}

	out := make([]ServerGroup, 0, len(order))
	for _, host := range order {
		group := groupsMap[host]
		sort.Slice(group.Connections, func(i, j int) bool {
			return group.Connections[i].Latency < group.Connections[j].Latency
		})
		out = append(out, *group)
	}
	return out
}

func TraceHops(ctx context.Context, host string, maxHops int) int {
	cmd := exec.CommandContext(ctx, "traceroute", "-m", strconv.Itoa(maxHops), "-w", "1", "-q", "1", host)
	out, err := cmd.Output()
	if err != nil {
		return 0
	}

	lines := strings.Split(string(out), "\n")
	lastHop := 0
	re := regexp.MustCompile(`^\s*(\d+)\s+`)

	for _, line := range lines {
		matches := re.FindStringSubmatch(line)
		if len(matches) > 1 {
			var h int
			_, _ = fmt.Sscanf(matches[1], "%d", &h)
			if h > lastHop {
				lastHop = h
			}
		}
	}

	return lastHop
}
