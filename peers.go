package main

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

func ParsePeerSourceJSON(b []byte) (*PeerSource, error) {
	var raw map[string]map[string][]string
	if err := json.Unmarshal(b, &raw); err != nil {
		return nil, fmt.Errorf("unsupported peers JSON format: %w", err)
	}

	src := &PeerSource{
		Servers: make([]PeerServer, 0),
		RawJSON: append(json.RawMessage(nil), b...),
	}

	regions := make([]string, 0, len(raw))
	for region := range raw {
		regions = append(regions, region)
	}
	sort.Strings(regions)

	for _, region := range regions {
		countriesMap := raw[region]

		countries := make([]string, 0, len(countriesMap))
		for country := range countriesMap {
			countries = append(countries, country)
		}
		sort.Strings(countries)

		for _, country := range countries {
			peers := UniqueSorted(countriesMap[country])
			if len(peers) == 0 {
				continue
			}

			src.Servers = append(src.Servers, PeerServer{
				Region:  strings.TrimSpace(region),
				Country: strings.TrimSpace(country),
				Peers:   peers,
			})
		}
	}

	if len(src.Servers) == 0 {
		return nil, fmt.Errorf("no peers found in JSON")
	}

	return src, nil
}

func FlattenPeerSource(src *PeerSource) []PeerInfo {
	if src == nil {
		return nil
	}

	out := make([]PeerInfo, 0)
	seen := make(map[string]struct{})

	for idx, server := range src.Servers {
		serverID := fmt.Sprintf("%s|%s|%d", server.Region, server.Country, idx)
		for _, peer := range server.Peers {
			peer = strings.TrimSpace(peer)
			if peer == "" {
				continue
			}

			key := serverID + "|" + peer
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}

			out = append(out, PeerInfo{
				Peer:     peer,
				Region:   server.Region,
				Country:  server.Country,
				ServerID: serverID,
			})
		}
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].Region != out[j].Region {
			return out[i].Region < out[j].Region
		}
		if out[i].Country != out[j].Country {
			return out[i].Country < out[j].Country
		}
		return out[i].Peer < out[j].Peer
	})

	return out
}

func ExtractPeerStrings(peers []PeerInfo) []string {
	out := make([]string, 0, len(peers))
	for _, p := range peers {
		out = append(out, p.Peer)
	}
	return out
}

func UniqueSorted(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if _, ok := seen[s]; !ok {
			seen[s] = struct{}{}
			out = append(out, s)
		}
	}
	sort.Strings(out)
	return out
}

func DiffPeers(oldList, newList []string) (toAdd, toRemove []string) {
	oldSet := make(map[string]struct{}, len(oldList))
	for _, s := range oldList {
		oldSet[s] = struct{}{}
	}
	newSet := make(map[string]struct{}, len(newList))
	for _, s := range newList {
		newSet[s] = struct{}{}
	}
	for s := range newSet {
		if _, ok := oldSet[s]; !ok {
			toAdd = append(toAdd, s)
		}
	}
	for s := range oldSet {
		if _, ok := newSet[s]; !ok {
			toRemove = append(toRemove, s)
		}
	}
	sort.Strings(toAdd)
	sort.Strings(toRemove)
	return
}
