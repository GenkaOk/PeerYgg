package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

func fetchURLInternal(url string, cfg *Config) ([]byte, error) {
	client := &http.Client{
		Timeout: time.Second * HTTPTimeoutSecs,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: cfg.InsecureSSL,
			},
		},
	}
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

// ParseAddress парсит адрес и возвращает хост, порт и схему
func ParseAddress(addr string) (host string, port string, scheme string, err error) {
	if idx := strings.Index(addr, "://"); idx != -1 {
		scheme = addr[:idx]
		addr = addr[idx+3:]
	} else {
		scheme = "tcp"
	}

	if idx := strings.Index(addr, "?"); idx != -1 {
		addr = addr[:idx]
	}

	if idx := strings.Index(addr, "/"); idx != -1 {
		addr = addr[:idx]
	}

	host, port, err = net.SplitHostPort(addr)
	if err != nil {
		return "", "", "", fmt.Errorf("invalid host:port (%s): %v", addr, err)
	}

	host = normalizeHost(host)

	return host, port, scheme, nil
}

func normalizeHost(host string) string {
	if ip := net.ParseIP(host); ip != nil {
		return ip.String()
	}

	ips, err := net.LookupIP(host)
	if err == nil && len(ips) > 0 {
		return ips[0].String()
	}

	return host
}

func DialPeer(ctx context.Context, addr string, timeout time.Duration) (time.Duration, string, string, error) {
	host, port, scheme, err := ParseAddress(addr)
	if err != nil {
		return 0, "", "", err
	}

	dialAddr := net.JoinHostPort(host, port)
	d := net.Dialer{Timeout: timeout}
	start := time.Now()
	conn, err := d.DialContext(ctx, "tcp", dialAddr)
	lat := time.Since(start)
	if err != nil {
		return 0, "", "", err
	}
	defer conn.Close()

	return lat, host, scheme, nil
}

func MeasureAll(peers []PeerInfo, concurrency int, timeoutSec int, progressType ProgressType) []Result {
	var wg sync.WaitGroup
	inCh := make(chan PeerInfo, concurrency)
	outCh := make(chan Result, concurrency)
	sem := make(chan struct{}, concurrency)

	var progressMutex sync.Mutex
	var progress *ProgressTracker
	showProgress := false
	if progressType != WithoutProgress {
		progress = NewProgressTracker(len(peers), progressType)
		showProgress = true
	}

	ctx := context.Background()

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for p := range inCh {
				sem <- struct{}{}
				lat, host, scheme, err := DialPeer(ctx, p.Peer, time.Duration(timeoutSec)*time.Second)

				success := err == nil
				if success {
					outCh <- Result{
						Peer:    p.Peer,
						Latency: lat,
						Host:    host,
						Scheme:  scheme,
						Region:  p.Region,
						Country: p.Country,
					}
				}

				if showProgress {
					progressMutex.Lock()
					progress.Increment(success)
					progressMutex.Unlock()
				}

				<-sem
			}
		}()
	}

	go func() {
		for _, p := range peers {
			inCh <- p
		}
		close(inCh)
	}()

	go func() {
		wg.Wait()
		close(outCh)
	}()

	var res []Result
	for r := range outCh {
		res = append(res, r)
	}

	if showProgress && progress != nil {
		progress.Finish()
	}

	sort.Slice(res, func(i, j int) bool {
		return res[i].Latency < res[j].Latency
	})
	return res
}

// GroupByHost возвращает лучший Result для каждого host.
func GroupByHost(results []Result) []Result {
	bestByHost := make(map[string]Result)

	for _, r := range results {
		best, exists := bestByHost[r.Host]
		if !exists || r.Latency < best.Latency {
			bestByHost[r.Host] = r
		}
	}

	grouped := make([]Result, 0, len(bestByHost))
	for _, r := range bestByHost {
		grouped = append(grouped, r)
	}

	sort.Slice(grouped, func(i, j int) bool {
		return grouped[i].Latency < grouped[j].Latency
	})

	return grouped
}

// BuildServerGroups строит подробные группы для расширенного вывода.
func BuildServerGroups(results []Result) []ServerGroup {
	groups := make(map[string]*ServerGroup)

	// Обновляем Hops в соединениях групп, если они изменились
	for i := range results {
		r := results[i]
		if _, exists := groups[r.Host]; !exists {
			groups[r.Host] = &ServerGroup{
				Host:        r.Host,
				BestLatency: r.Latency,
				Region:      r.Region,
				Country:     r.Country,
				Connections: []Connection{},
			}
		}

		group := groups[r.Host]
		group.Connections = append(group.Connections, Connection{
			Peer:    r.Peer,
			Scheme:  r.Scheme,
			Latency: r.Latency,
			Hops:    r.Hops,
			Region:  r.Region,
			Country: r.Country,
		})

		if r.Latency < group.BestLatency {
			group.BestLatency = r.Latency
			group.Region = r.Region
			group.Country = r.Country
		}
	}

	var serverGroups []ServerGroup
	for _, g := range groups {
		sort.Slice(g.Connections, func(i, j int) bool {
			return g.Connections[i].Latency < g.Connections[j].Latency
		})
		serverGroups = append(serverGroups, *g)
	}

	sort.Slice(serverGroups, func(i, j int) bool {
		return serverGroups[i].BestLatency < serverGroups[j].BestLatency
	})

	return serverGroups
}

// GetBestPeersPerServer возвращает лучший пир для каждого сервера
func GetBestPeersPerServer(serverGroups []ServerGroup, topN int) []string {
	result := make([]string, 0, topN)
	for i, group := range serverGroups {
		if i >= topN {
			break
		}
		if len(group.Connections) > 0 {
			result = append(result, group.Connections[0].Peer)
		}
	}
	return result
}

// TraceHops выполняет traceroute до хоста и возвращает количество хопов.
func TraceHops(ctx context.Context, host string, maxHops int) int {
	// Используем -m 20 для ограничения максимального количества хопов (быстрее)
	// -w 1 для таймаута ожидания ответа 1 сек
	// -q 1 для отправки только одного пакета на каждый хоп
	cmd := exec.CommandContext(ctx, "traceroute", "-m", strconv.Itoa(maxHops), "-w", "1", "-q", "1", host)
	out, err := cmd.Output()
	if err != nil {
		return 0
	}

	lines := strings.Split(string(out), "\n")
	lastHop := 0
	// Регулярное выражение для поиска номера хопа в начале строки
	re := regexp.MustCompile(`^\s*(\d+)\s+`)

	for _, line := range lines {
		matches := re.FindStringSubmatch(line)
		if len(matches) > 1 {
			hop, _ := fmt.Sscanf(matches[1], "%d", &lastHop)
			if hop > 0 {
				// Продолжаем, чтобы найти последний номер
			}
		}
	}

	return lastHop
}
