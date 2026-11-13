package peer

import (
	"strings"
	"testing"
)

func TestNormalizeCountry(t *testing.T) {
	cases := map[string]string{
		"NL":              "netherlands",
		" nl ":            "netherlands",
		"Netherlands":     "netherlands",
		"holland":         "netherlands",
		"United States":   "united-states",
		"united_states":   "united-states",
		"usa":             "united-states",
		"UK":              "united-kingdom",
		"czech republic":  "czechia",
		"czech-republic":  "czechia",
		"":                "",
		"atlantis":        "atlantis",
		"New Caledonia":   "new-caledonia",
		"  saudi arabia ": "saudi-arabia",
	}

	for input, want := range cases {
		if got := NormalizeCountry(input); got != want {
			t.Errorf("NormalizeCountry(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestResolveCountry(t *testing.T) {
	src := &Source{Servers: []Server{
		{Region: "europe", Country: "netherlands"},
		{Region: "europe", Country: "germany"},
		{Region: "asia", Country: "japan"},
	}}

	country, region, err := ResolveCountry(src, "NL")
	if err != nil {
		t.Fatalf("ResolveCountry() error = %v", err)
	}
	if country != "netherlands" {
		t.Errorf("country = %q, want netherlands", country)
	}
	if region != "europe" {
		t.Errorf("region = %q, want europe", region)
	}

	if _, _, err := ResolveCountry(src, "zz"); err == nil {
		t.Fatal("ResolveCountry() for unknown country returned no error")
	} else if !strings.Contains(err.Error(), "netherlands") {
		t.Errorf("error should list available countries, got %v", err)
	}

	if _, _, err := ResolveCountry(nil, "nl"); err == nil {
		t.Fatal("ResolveCountry() for nil source returned no error")
	}

	if _, _, err := ResolveCountry(src, ""); err == nil {
		t.Fatal("ResolveCountry() for empty input returned no error")
	}
}
