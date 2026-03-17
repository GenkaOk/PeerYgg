package peer

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

type Source struct {
	Servers []Server `json:"servers"`
	RawJSON json.RawMessage
}

type Server struct {
	Region  string   `json:"region"`
	Country string   `json:"country"`
	Peers   []string `json:"peers"`
}

type Info struct {
	Peer     string `json:"peer"`
	Region   string `json:"region"`
	Country  string `json:"country"`
	ServerID string `json:"server_id"`
}

func ParseSourceJSON(b []byte) (*Source, error) {
	var raw map[string]map[string][]string
	if err := json.Unmarshal(b, &raw); err != nil {
		return nil, fmt.Errorf("unsupported peers JSON format: %w", err)
	}

	src := &Source{
		Servers: make([]Server, 0),
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

			src.Servers = append(src.Servers, Server{
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

func FlattenSource(src *Source) []Info {
	if src == nil {
		return nil
	}

	out := make([]Info, 0)
	seen := make(map[string]struct{})

	for idx, server := range src.Servers {
		serverID := fmt.Sprintf("%s|%s|%d", server.Region, server.Country, idx)
		for _, p := range server.Peers {
			p = strings.TrimSpace(p)
			if p == "" {
				continue
			}

			key := serverID + "|" + p
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}

			out = append(out, Info{
				Peer:     p,
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

func ExtractPeerStrings(peers []Info) []string {
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

func Diff(oldList, newList []string) (toAdd, toRemove []string) {
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
