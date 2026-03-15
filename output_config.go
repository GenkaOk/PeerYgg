package main

import (
	"encoding/json"
	"fmt"
	"os"
)

func writeConfigPeers(peers []string) {
	b, err := json.Marshal(peers)
	if err != nil {
		fmt.Fprintln(os.Stderr, "failed encode config peers:", err)
		return
	}
	fmt.Fprintf(os.Stdout, "Peers: %s\n", string(b))
}
