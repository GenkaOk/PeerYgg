package network

import (
	"testing"
	"time"
)

func result(peer, host, country, region string, ms int) Result {
	return Result{
		Peer:    peer,
		Host:    host,
		Country: country,
		Region:  region,
		Latency: time.Duration(ms) * time.Millisecond,
	}
}

func peers(results []Result) []string {
	out := make([]string, 0, len(results))
	for _, r := range results {
		out = append(out, r.Peer)
	}
	return out
}

func TestSelectPreferredWithoutCountry(t *testing.T) {
	all := []Result{
		result("a:1", "a", "netherlands", "europe", 10),
		result("b:1", "b", "germany", "europe", 20),
		result("c:1", "c", "japan", "asia", 30),
	}

	selected, stats := SelectPreferred(all, 2, 2, "", "", false)
	if len(selected) != 2 {
		t.Fatalf("len(selected) = %d, want 2", len(selected))
	}
	if got := peers(selected); got[0] != "a:1" || got[1] != "b:1" {
		t.Errorf("selected peers = %v, want [a:1 b:1]", got)
	}
	if stats.TargetSelected != 0 || stats.NearbyAdded != 0 {
		t.Errorf("stats = %+v, want zero counts", stats)
	}
}

func TestSelectPreferredGuaranteeAlreadyMet(t *testing.T) {
	all := []Result{
		result("a:1", "a", "netherlands", "europe", 10),
		result("b:1", "b", "netherlands", "europe", 20),
		result("c:1", "c", "germany", "europe", 30),
	}

	selected, stats := SelectPreferred(all, 2, 2, "netherlands", "europe", false)
	if len(selected) != 2 {
		t.Fatalf("len(selected) = %d, want 2", len(selected))
	}
	for _, r := range selected {
		if !r.Preferred {
			t.Errorf("peer %s should be marked preferred", r.Peer)
		}
	}
	if stats.TargetSelected != 2 || stats.NearbyAdded != 0 {
		t.Errorf("stats = %+v, want 2 target and 0 nearby", stats)
	}
}

func TestSelectPreferredAddsTargetPeers(t *testing.T) {
	all := []Result{
		result("a:1", "a", "germany", "europe", 10),
		result("b:1", "b", "germany", "europe", 20),
		result("c:1", "c", "netherlands", "europe", 30),
		result("d:1", "d", "netherlands", "europe", 40),
		result("e:1", "e", "germany", "europe", 50),
	}

	selected, stats := SelectPreferred(all, 2, 2, "netherlands", "europe", false)
	want := []string{"a:1", "b:1", "c:1", "d:1"}
	if got := peers(selected); len(got) != len(want) || got[0] != want[0] || got[1] != want[1] || got[2] != want[2] || got[3] != want[3] {
		t.Fatalf("selected peers = %v, want %v", got, want)
	}
	if !selected[2].Preferred || !selected[3].Preferred {
		t.Error("added target peers should be marked preferred")
	}
	if selected[0].Preferred || selected[1].Preferred {
		t.Error("unrelated fastest peers should not be marked preferred")
	}
	if !all[2].Preferred || !all[3].Preferred {
		t.Error("marks should also be applied to the input slice")
	}
	if stats.TargetSelected != 2 || stats.NearbyAdded != 0 {
		t.Errorf("stats = %+v, want 2 target and 0 nearby", stats)
	}
	if len(stats.AddedPeers) != 2 || stats.AddedPeers[0] != "c:1" || stats.AddedPeers[1] != "d:1" {
		t.Errorf("AddedPeers = %v, want [c:1 d:1]", stats.AddedPeers)
	}
}

func TestSelectPreferredRegionFallback(t *testing.T) {
	all := []Result{
		result("a:1", "a", "netherlands", "europe", 10),
		result("b:1", "b", "germany", "europe", 20),
		result("c:1", "c", "france", "europe", 30),
		result("d:1", "d", "japan", "asia", 40),
	}

	selected, stats := SelectPreferred(all, 1, 3, "netherlands", "europe", false)
	want := []string{"a:1", "b:1", "c:1"}
	if got := peers(selected); len(got) != len(want) {
		t.Fatalf("selected peers = %v, want %v", got, want)
	}
	for i, r := range selected {
		if r.Peer != want[i] {
			t.Errorf("selected[%d] = %s, want %s", i, r.Peer, want[i])
		}
	}
	if selected[0].Preferred != true || selected[0].Nearby {
		t.Error("target peer should be preferred, not nearby")
	}
	if !selected[1].Nearby || !selected[2].Nearby {
		t.Error("fallback peers should be marked nearby")
	}
	if stats.TargetSelected != 1 || stats.NearbyAdded != 2 {
		t.Errorf("stats = %+v, want 1 target and 2 nearby", stats)
	}
	if len(stats.AddedPeers) != 2 || stats.AddedPeers[0] != "b:1" || stats.AddedPeers[1] != "c:1" {
		t.Errorf("AddedPeers = %v, want [b:1 c:1]", stats.AddedPeers)
	}
	if stats.TargetLive != 1 {
		t.Errorf("TargetLive = %d, want 1", stats.TargetLive)
	}
}

func TestSelectPreferredWithoutRegionFallback(t *testing.T) {
	all := []Result{
		result("a:1", "a", "netherlands", "europe", 10),
		result("b:1", "b", "germany", "europe", 20),
	}

	selected, stats := SelectPreferred(all, 1, 2, "netherlands", "", false)
	if len(selected) != 1 {
		t.Fatalf("len(selected) = %d, want 1", len(selected))
	}
	if stats.NearbyAdded != 0 {
		t.Errorf("NearbyAdded = %d, want 0", stats.NearbyAdded)
	}
}

func TestSelectPreferredGroupByHost(t *testing.T) {
	all := []Result{
		result("x:1", "host-1", "netherlands", "europe", 10),
		result("y:1", "host-1", "netherlands", "europe", 20),
		result("z:1", "host-2", "germany", "europe", 30),
		result("w:1", "host-3", "netherlands", "europe", 40),
	}

	selected, stats := SelectPreferred(all, 1, 2, "netherlands", "europe", true)
	want := []string{"x:1", "w:1"}
	if got := peers(selected); len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("selected peers = %v, want %v", got, want)
	}
	if stats.TargetSelected != 2 {
		t.Errorf("TargetSelected = %d, want 2", stats.TargetSelected)
	}
}

func TestSelectPreferredZeroCount(t *testing.T) {
	all := []Result{
		result("a:1", "a", "netherlands", "europe", 10),
		result("b:1", "b", "netherlands", "europe", 20),
	}

	selected, _ := SelectPreferred(all, 2, 0, "netherlands", "europe", false)
	if len(selected) != 2 {
		t.Fatalf("len(selected) = %d, want 2", len(selected))
	}
	for _, r := range selected {
		if r.Preferred || r.Nearby {
			t.Errorf("peer %s should not be marked when count is zero", r.Peer)
		}
	}
}

func TestSelectPreferredClampsTopN(t *testing.T) {
	all := []Result{
		result("a:1", "a", "netherlands", "europe", 10),
		result("b:1", "b", "germany", "europe", 20),
	}

	selected, _ := SelectPreferred(all, 10, 2, "", "", false)
	if len(selected) != len(all) {
		t.Fatalf("len(selected) = %d, want %d", len(selected), len(all))
	}

	selected, _ = SelectPreferred(all, -1, 2, "", "", false)
	if len(selected) != 0 {
		t.Fatalf("len(selected) = %d, want 0", len(selected))
	}
}
