package progress

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/genkaok/PeerYgg/internal/config"
)

type Tracker struct {
	Total        int
	Completed    int
	Successful   int
	Failed       int
	startTime    time.Time
	progressType config.ProgressType
}

func NewTracker(total int, progressType config.ProgressType) *Tracker {
	return &Tracker{
		Total:        total,
		Completed:    0,
		Successful:   0,
		Failed:       0,
		startTime:    time.Now(),
		progressType: progressType,
	}
}

func (pt *Tracker) Increment(success bool) {
	pt.Completed++
	if success {
		pt.Successful++
	} else {
		pt.Failed++
	}
	pt.Print()
}

func (pt *Tracker) Print() {
	elapsed := time.Since(pt.startTime)
	percentage := float64(pt.Completed) / float64(pt.Total) * 100

	var eta string
	if pt.Completed > 0 {
		avgTime := elapsed / time.Duration(pt.Completed)
		remaining := time.Duration(pt.Total-pt.Completed) * avgTime
		eta = formatDuration(remaining)
	} else {
		eta = "calculating..."
	}

	if pt.progressType == config.FullProgress {
		barLength := 30
		filledLength := int(float64(barLength) * float64(pt.Completed) / float64(pt.Total))

		bar := strings.Repeat("█", filledLength) + strings.Repeat("░", barLength-filledLength)

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
	} else if pt.progressType == config.SimpleProgress {
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

func (pt *Tracker) Finish() {
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
