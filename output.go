package main

func printResults(results []Result, topN int, format OutputFormat) {
	topResults := limitResults(results, topN)

	switch format {
	case OutputJSON:
		writeResultsJSON(results)
	case OutputTable:
		writeResultsTable(results)
	case OutputConfig:
		writeConfigPeers(extractPeersFromResults(topResults))
	default:
		writeResultsCurrent(topResults)
	}
}

func printGroupedResults(results []Result, topN int, format OutputFormat) {
	serverGroups := GroupByHost(results)

	top := topN
	if top > len(serverGroups) {
		top = len(serverGroups)
	}
	topGroups := serverGroups[:top]

	switch format {
	case OutputJSON:
		writeGroupsJSON(topGroups)
	case OutputTable:
		writeGroupsTable(topGroups)
	case OutputConfig:
		writeConfigPeers(GetBestPeersPerServer(topGroups, topN))
	default:
		writeGroupsCurrent(topGroups, topN)
	}
}

func limitResults(results []Result, topN int) []Result {
	top := topN
	if top > len(results) {
		top = len(results)
	}
	return results[:top]
}

func extractPeersFromResults(results []Result) []string {
	out := make([]string, 0, len(results))
	for _, r := range results {
		out = append(out, r.Peer)
	}
	return out
}
