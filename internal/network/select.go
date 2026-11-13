package network

import "strings"

// SelectStats describes how the preferred country affected the selection.
type SelectStats struct {
	Country        string
	Region         string
	TargetLive     int
	TargetSelected int
	NearbyAdded    int
	AddedPeers     []string
}

// SelectPreferred builds the output list from latency-sorted results.
//
// It keeps the topN fastest peers and, when a country is requested, appends the
// fastest remaining peers from that country until countryCount peers from the
// country are present. If the country has too few reachable peers, the
// shortfall is filled with the fastest peers from the same region. The total is
// capped at topN+countryCount.
//
// Added peers are marked in both the returned slice and in all, so callers can
// render the complete measurement set with selection flags. In group mode the
// guarantee is applied to host representatives rather than individual peers.
func SelectPreferred(all []Result, topN, countryCount int, country, region string, groupByHost bool) ([]Result, SelectStats) {
	stats := SelectStats{Country: country, Region: region}

	candidates := all
	if groupByHost {
		candidates = GroupByHost(all)
	}

	if topN < 0 {
		topN = 0
	}
	if topN > len(candidates) {
		topN = len(candidates)
	}

	selected := make([]Result, topN)
	copy(selected, candidates[:topN])

	stats.TargetLive = countResults(all, func(r Result) bool { return sameCountry(r.Country, country) })
	stats.TargetSelected = countResults(selected, func(r Result) bool { return sameCountry(r.Country, country) })

	if country == "" || countryCount <= 0 {
		return selected, stats
	}

	index := make(map[string]int, len(all))
	for i := range all {
		index[all[i].Peer] = i
	}

	markPreferred := func(i int) {
		selected[i].Preferred = true
		if j, ok := index[selected[i].Peer]; ok {
			all[j].Preferred = true
		}
	}
	for i := range selected {
		if sameCountry(selected[i].Country, country) {
			markPreferred(i)
		}
	}

	need := countryCount - stats.TargetSelected
	if need <= 0 {
		return selected, stats
	}

	taken := make(map[string]struct{}, len(selected))
	for _, r := range selected {
		taken[r.Peer] = struct{}{}
	}

	appendPeer := func(r Result) {
		selected = append(selected, r)
		taken[r.Peer] = struct{}{}
		stats.AddedPeers = append(stats.AddedPeers, r.Peer)
		if j, ok := index[r.Peer]; ok {
			all[j].Preferred = r.Preferred
			all[j].Nearby = r.Nearby
		}
	}

	for _, r := range candidates {
		if need == 0 {
			break
		}
		if _, ok := taken[r.Peer]; ok {
			continue
		}
		if !sameCountry(r.Country, country) {
			continue
		}

		r.Preferred = true
		r.Nearby = false
		appendPeer(r)
		stats.TargetSelected++
		need--
	}

	if need > 0 && region != "" {
		for _, r := range candidates {
			if need == 0 {
				break
			}
			if _, ok := taken[r.Peer]; ok {
				continue
			}
			if !sameCountry(r.Region, region) || sameCountry(r.Country, country) {
				continue
			}

			r.Nearby = true
			r.Preferred = false
			appendPeer(r)
			stats.NearbyAdded++
			need--
		}
	}

	return selected, stats
}

func sameCountry(a, b string) bool {
	if b == "" {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(a), strings.TrimSpace(b))
}

func countResults(results []Result, match func(Result) bool) int {
	n := 0
	for _, r := range results {
		if match(r) {
			n++
		}
	}
	return n
}
