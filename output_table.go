package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

func writeResultsTable(results []Result) {
	headers := []string{"№", "Latency", "Region", "Country", "Peer URI"}
	rows := make([][]string, 0, len(results))

	for i, r := range results {
		rows = append(rows, []string{
			strconv.Itoa(i + 1),
			fmt.Sprintf("%d ms", r.Latency.Milliseconds()),
			r.Region,
			r.Country,
			r.Peer,
		})
	}

	writeAlignedTable(headers, rows)
}

func writeGroupsTable(groups []ServerGroup) {
	headers := []string{"№", "Latency", "Region", "Country", "Host", "Connections", "Best Peer"}
	rows := make([][]string, 0, len(groups))

	for i, g := range groups {
		bestPeer := ""
		if len(g.Connections) > 0 {
			bestPeer = g.Connections[0].Peer
		}

		rows = append(rows, []string{
			strconv.Itoa(i + 1),
			fmt.Sprintf("%d ms", g.BestLatency.Milliseconds()),
			g.Region,
			g.Country,
			g.Host,
			strconv.Itoa(len(g.Connections)),
			bestPeer,
		})
	}

	writeAlignedTable(headers, rows)
}

func writeAlignedTable(headers []string, rows [][]string) {
	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = len(h)
	}

	for _, row := range rows {
		for i := 0; i < len(headers) && i < len(row); i++ {
			if len(row[i]) > widths[i] {
				widths[i] = len(row[i])
			}
		}
	}

	printSeparate(widths, "=")
	printAlignedRow(headers, widths, false)
	printSeparate(widths, "-")

	for i := len(rows) - 1; i >= 0; i-- {
		printAlignedRow(rows[i], widths, false)
	}

	printSeparate(widths, "-")
	printAlignedRow(headers, widths, false)
	printSeparate(widths, "=")
}

func printSeparate(widths []int, sep string) {
	if sep == "" {
		sep = "-"
	}

	totalWidth := 0
	for i, w := range widths {
		totalWidth += w
		if i > 0 {
			totalWidth += 3
		}
	}

	fmt.Fprintln(os.Stdout, strings.Repeat(sep, totalWidth))
}

func printAlignedRow(cols []string, widths []int, center bool) {
	for i := range widths {
		val := ""
		if i < len(cols) {
			val = cols[i]
		}

		if i > 0 {
			fmt.Fprint(os.Stdout, " | ")
		}

		if center {
			fmt.Fprint(os.Stdout, centerText(val, widths[i]))
			continue
		}

		fmt.Fprintf(os.Stdout, "%-*s", widths[i], val)
	}
	fmt.Fprintln(os.Stdout)
}

func centerText(s string, width int) string {
	if len(s) >= width {
		return s
	}

	padding := width - len(s)
	left := padding / 2
	right := padding - left

	return strings.Repeat(" ", left) + s + strings.Repeat(" ", right)
}
