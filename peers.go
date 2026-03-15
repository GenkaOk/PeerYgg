package main

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

func ParsePeersFromJSON(b []byte) ([]string, error) {
	var root interface{}
	if err := json.Unmarshal(b, &root); err != nil {
		return nil, err
	}
	var raw []string
	extractStringsFromInterface(root, &raw)
	if len(raw) == 0 {
		return nil, fmt.Errorf("no peer strings found in JSON")
	}
	return UniqueSorted(raw), nil
}

func extractStringsFromInterface(v interface{}, out *[]string) {
	switch t := v.(type) {
	case string:
		s := strings.TrimSpace(t)
		if s != "" {
			*out = append(*out, s)
		}
	case []interface{}:
		for _, it := range t {
			extractStringsFromInterface(it, out)
		}
	case map[string]interface{}:
		for _, it := range t {
			extractStringsFromInterface(it, out)
		}
	}
}

func UniqueSorted(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
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
