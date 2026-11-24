package main

import (
	"reflect"
	"testing"

	"github.com/genkaok/PeerYgg/internal/network"
)

func TestSelectedPeers(t *testing.T) {
	t.Run("preserves_order", func(t *testing.T) {
		results := []network.Result{
			{Peer: "tcp://first.example:443"},
			{Peer: "tcp://second.example:443"},
		}
		want := []string{"tcp://first.example:443", "tcp://second.example:443"}

		got := selectedPeers(results)
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("selectedPeers(%v) = %v, want %v", results, got, want)
		}
	})

	t.Run("empty", func(t *testing.T) {
		got := selectedPeers(nil)
		if len(got) != 0 {
			t.Fatalf("selectedPeers(nil) = %v, want empty", got)
		}
	})
}
