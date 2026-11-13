package peer

import (
	"fmt"
	"sort"
	"strings"
)

// countryAliases maps ISO alpha-2 codes and common names to the slugs used in
// the public peers list. Countries not listed here still resolve by their
// normalized slug.
var countryAliases = map[string]string{
	"am":             "armenia",
	"armenia":        "armenia",
	"at":             "austria",
	"austria":        "austria",
	"au":             "australia",
	"australia":      "australia",
	"br":             "brazil",
	"brazil":         "brazil",
	"ca":             "canada",
	"canada":         "canada",
	"ch":             "switzerland",
	"switzerland":    "switzerland",
	"cl":             "chile",
	"chile":          "chile",
	"cz":             "czechia",
	"czechia":        "czechia",
	"czech-republic": "czechia",
	"de":             "germany",
	"germany":        "germany",
	"es":             "spain",
	"spain":          "spain",
	"fi":             "finland",
	"finland":        "finland",
	"fr":             "france",
	"france":         "france",
	"gb":             "united-kingdom",
	"great-britain":  "united-kingdom",
	"uk":             "united-kingdom",
	"united-kingdom": "united-kingdom",
	"hk":             "hong-kong",
	"hong-kong":      "hong-kong",
	"hu":             "hungary",
	"hungary":        "hungary",
	"id":             "indonesia",
	"indonesia":      "indonesia",
	"in":             "india",
	"india":          "india",
	"jp":             "japan",
	"japan":          "japan",
	"lu":             "luxembourg",
	"luxembourg":     "luxembourg",
	"holland":        "netherlands",
	"netherlands":    "netherlands",
	"nl":             "netherlands",
	"no":             "norway",
	"norway":         "norway",
	"pl":             "poland",
	"poland":         "poland",
	"ro":             "romania",
	"romania":        "romania",
	"ru":             "russia",
	"russia":         "russia",
	"sa":             "saudi-arabia",
	"saudi-arabia":   "saudi-arabia",
	"se":             "sweden",
	"sweden":         "sweden",
	"sg":             "singapore",
	"singapore":      "singapore",
	"sk":             "slovakia",
	"slovakia":       "slovakia",
	"tw":             "taiwan",
	"taiwan":         "taiwan",
	"ua":             "ukraine",
	"ukraine":        "ukraine",
	"america":        "united-states",
	"united-states":  "united-states",
	"us":             "united-states",
	"usa":            "united-states",
	"za":             "south-africa",
	"south-africa":   "south-africa",
}

// NormalizeCountry converts user input (ISO code, country name, or slug) into
// the lowercase slug used by the peers source. Unknown values are returned as
// normalized slugs so future countries keep working.
func NormalizeCountry(input string) string {
	slug := strings.ToLower(strings.TrimSpace(input))
	if slug == "" {
		return ""
	}
	slug = strings.ReplaceAll(slug, "_", "-")
	slug = strings.Join(strings.Fields(slug), "-")
	if canonical, ok := countryAliases[slug]; ok {
		return canonical
	}
	return slug
}

// ResolveCountry returns the canonical country slug and its region from the
// peers source, or an error listing the countries that are available.
func ResolveCountry(src *Source, input string) (string, string, error) {
	country := NormalizeCountry(input)
	if country == "" {
		return "", "", fmt.Errorf("preferred country is empty")
	}
	if src == nil {
		return "", "", fmt.Errorf("cannot resolve country %q: peers source is empty", input)
	}

	regions := make(map[string]string)
	for _, server := range src.Servers {
		slug := NormalizeCountry(server.Country)
		if slug == "" {
			continue
		}
		if _, ok := regions[slug]; !ok {
			regions[slug] = strings.TrimSpace(server.Region)
		}
	}

	if region, ok := regions[country]; ok {
		return country, region, nil
	}

	available := make([]string, 0, len(regions))
	for slug := range regions {
		available = append(available, slug)
	}
	sort.Strings(available)

	return "", "", fmt.Errorf("country %q not found in peers list (available: %s)", input, strings.Join(available, ", "))
}
