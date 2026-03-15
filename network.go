package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

func fetchURLInternal(url string) ([]byte, error) {
	client := &http.Client{Timeout: time.Second * HTTPTimeoutSecs}
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	return ioutil.ReadAll(resp.Body)
}

// ParseAddress парсит адрес и возвращает хост, порт и схему
func ParseAddress(addr string) (host string, port string, scheme string, err error) {
	// Извлечь схему
	if idx := strings.Index(addr, "://"); idx != -1 {
		scheme = addr[:idx]
		addr = addr[idx+3:]
	} else {
		scheme = "tcp"
	}

	// Удалить query params
	if idx := strings.Index(addr, "?"); idx != -1 {
		addr = addr[:idx]
	}

	// Удалить path
	if idx := strings.Index(addr, "/"); idx != -1 {
		addr = addr[:idx]
	}

	// Парсить host:port
	host, port, err = net.SplitHostPort(addr)
	if err != nil {
		return "", "", "", fmt.Errorf("invalid host:port (%s): %v", addr, err)
	}

	// Если это hostname, резолвим его в IP
	host = normalizeHost(host)

	return host, port, scheme, nil
}

func normalizeHost(host string) string {
	// Попытаться резолвить hostname в IP
	if ip := net.ParseIP(host); ip != nil {
		return ip.String()
	}

	// Если это hostname, резолвим
	ips, err := net.LookupIP(host)
	if err == nil && len(ips) > 0 {
		return ips[0].String()
	}

	// Если не удалось резолвить, возвращаем как есть
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
	_ = conn.Close()
	return lat, host, scheme, nil
}

func MeasureAll(peers []string, concurrency int, timeoutSec int) []Result {
	var wg sync.WaitGroup
	inCh := make(chan string, concurrency)
	outCh := make(chan Result, concurrency)
	sem := make(chan struct{}, concurrency)

	ctx := context.Background()

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for p := range inCh {
				sem <- struct{}{}
				lat, host, scheme, err := DialPeer(ctx, p, time.Duration(timeoutSec)*time.Second)
				if err == nil {
					outCh <- Result{
						Peer:    p,
						Latency: lat,
						Host:    host,
						Scheme:  scheme,
					}
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
	sort.Slice(res, func(i, j int) bool {
		return res[i].Latency < res[j].Latency
	})
	return res
}

// GroupByHost группирует результаты по хостам, выбирая лучшую задержку
func GroupByHost(results []Result) []ServerGroup {
	groups := make(map[string]*ServerGroup)

	for _, r := range results {
		if _, exists := groups[r.Host]; !exists {
			groups[r.Host] = &ServerGroup{
				Host:        r.Host,
				BestLatency: r.Latency,
				Connections: []Connection{},
			}
		}

		group := groups[r.Host]
		group.Connections = append(group.Connections, Connection{
			Peer:    r.Peer,
			Scheme:  r.Scheme,
			Latency: r.Latency,
		})

		// Обновить лучшую задержку
		if r.Latency < group.BestLatency {
			group.BestLatency = r.Latency
		}
	}

	// Конвертировать в слайс
	var serverGroups []ServerGroup
	for _, g := range groups {
		// Сортировать подключения по задержке
		sort.Slice(g.Connections, func(i, j int) bool {
			return g.Connections[i].Latency < g.Connections[j].Latency
		})
		serverGroups = append(serverGroups, *g)
	}

	// Сортировать группы по лучшей задержке
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
		// Возвращаем пир с лучшей задержкой
		if len(group.Connections) > 0 {
			result = append(result, group.Connections[0].Peer)
		}
	}
	return result
}
