package main

import (
	"fmt"
	"os"
	"strings"
	"time"
)

func NewProgressTracker(total int, progressType ProgressType) *ProgressTracker {
	return &ProgressTracker{
		Total:        total,
		Completed:    0,
		Successful:   0,
		Failed:       0,
		startTime:    time.Now(),
		progressType: progressType,
	}
}

func (pt *ProgressTracker) Increment(success bool) {
	pt.Completed++
	if success {
		pt.Successful++
	} else {
		pt.Failed++
	}
	pt.Print()
}

func (pt *ProgressTracker) Print() {
	elapsed := time.Since(pt.startTime)
	percentage := float64(pt.Completed) / float64(pt.Total) * 100

	// Расчет оставшегося времени
	var eta string
	if pt.Completed > 0 {
		avgTime := elapsed / time.Duration(pt.Completed)
		remaining := time.Duration(pt.Total-pt.Completed) * avgTime
		eta = formatDuration(remaining)
	} else {
		eta = "calculating..."
	}

	if pt.progressType == FullProgress {
		barLength := 30
		filledLength := int(float64(barLength) * float64(pt.Completed) / float64(pt.Total))

		// Построить прогресс-бар
		bar := strings.Repeat("█", filledLength) + strings.Repeat("░", barLength-filledLength)

		// Вывод прогресса
		fmt.Fprintf(os.Stderr, "\r[%s] %.1f%% (%d/%d) | ✓ %d ✗ %d | Elapsed: %s | ETA: %s",
			bar,
			percentage,
			pt.Completed,
			pt.Total,
			pt.Successful,
			pt.Failed,
			formatDuration(elapsed),
			eta,
		)
	} else if pt.progressType == SimpleProgress {
		fmt.Fprintf(os.Stderr, "\r[%d/%d] %.1f%% | ✓ %d ✗ %d | Elapsed: %s | ETA: %s",
			pt.Completed,
			pt.Total,
			percentage,
			pt.Successful,
			pt.Failed,
			formatDuration(elapsed),
			eta,
		)
	}
}

func (pt *ProgressTracker) Finish() {
	elapsed := time.Since(pt.startTime)
	fmt.Fprintf(os.Stderr, "\n✓ Scanning complete in %s\n", formatDuration(elapsed))
	fmt.Fprintf(os.Stderr, "Results: %d successful, %d failed out of %d peers\n\n",
		pt.Successful, pt.Failed, pt.Total)
}

func formatDuration(d time.Duration) string {
	d = d.Round(time.Second)
	h := d / time.Hour
	d %= time.Hour
	m := d / time.Minute
	d %= time.Minute
	s := d / time.Second

	if h > 0 {
		return fmt.Sprintf("%dh%dm%ds", h, m, s)
	}
	if m > 0 {
		return fmt.Sprintf("%dm%ds", m, s)
	}
	return fmt.Sprintf("%ds", s)
}
