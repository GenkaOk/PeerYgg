package main

func printResults(results []Result, topN int, format OutputFormat) {
	topResults := limitResults(results, topN)

	switch format {
	case OutputJSON:
		writeResultsJSON(results) // Выводим все записи
	case OutputTable:
		writeResultsTable(results) // Выводим все записи
	case OutputConfig:
		writeConfigPeers(topResults)
	default:
		writeResultsCurrent(topResults)
	}
}

func printGroupedResults(results []Result, topN int, format OutputFormat) {
	groupedResults := GroupByHost(results)
	topGroupedResults := limitResults(groupedResults, topN)

	switch format {
	case OutputJSON:
		writeResultsJSON(groupedResults) // Выводим все записи
	case OutputTable:
		writeResultsTable(groupedResults) // Выводим все записи
	case OutputConfig:
		writeConfigPeers(topGroupedResults)
	default:
		writeGroupsCurrent(BuildServerGroups(topGroupedResults), topN)
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
