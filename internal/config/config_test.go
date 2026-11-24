package config

import "testing"

func TestResolveConfPath(t *testing.T) {
	cases := []struct {
		name      string
		flagValue string
		env       string
		want      string
	}{
		{name: "flag_wins_over_env", flagValue: "/flag/yggdrasil.conf", env: "/env/yggdrasil.conf", want: "/flag/yggdrasil.conf"},
		{name: "env_when_flag_empty", flagValue: "", env: "/env/yggdrasil.conf", want: "/env/yggdrasil.conf"},
		{name: "empty_when_both_empty", flagValue: "", env: "", want: ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("PEERS_CONFIG", tc.env)

			if got := resolveConfPath(tc.flagValue); got != tc.want {
				t.Fatalf("resolveConfPath(%q) with PEERS_CONFIG=%q = %q, want %q", tc.flagValue, tc.env, got, tc.want)
			}
		})
	}
}
