package parse

import (
	"encoding/xml"
	"strconv"
	"time"
)

// SitemapURL is one <url> entry.
type SitemapURL struct {
	Loc      string
	LastMod  time.Time
	Priority float64
}

// SitemapSummary captures domain-level sitemap facts.
type SitemapSummary struct {
	Reachable bool
	URLs      []SitemapURL
	IsIndex   bool
	FetchedAt time.Time
}

// urlSetXML / sitemapIndexXML mirror the relevant subset of
// sitemaps.org's schema.

type sitemapLoc struct {
	XMLName xml.Name `xml:"url"`
	Loc     string   `xml:"loc"`
	LastMod string   `xml:"lastmod"`
	Prio    string   `xml:"priority"`
}

type urlSetXML struct {
	XMLName xml.Name     `xml:"urlset"`
	URLs    []sitemapLoc `xml:"url"`
}

type sitemapIndexEntry struct {
	Loc     string `xml:"loc"`
	LastMod string `xml:"lastmod"`
}

type sitemapIndexXML struct {
	XMLName  xml.Name            `xml:"sitemapindex"`
	Sitemaps []sitemapIndexEntry `xml:"sitemap"`
}

// ParseSitemap returns a SitemapSummary and a slice of sub-sitemap
// URLs if the input is a sitemap-index. Caller is responsible for
// fetching sub-sitemaps and merging results. Capped at 1000 URLs.
func ParseSitemap(body []byte) (*SitemapSummary, []string) {
	sum := &SitemapSummary{Reachable: true, FetchedAt: time.Now()}
	if len(body) == 0 {
		sum.Reachable = false
		return sum, nil
	}

	// Try urlset first.
	var uset urlSetXML
	if err := xml.Unmarshal(body, &uset); err == nil && len(uset.URLs) > 0 {
		for i, u := range uset.URLs {
			if i >= 1000 {
				break
			}
			entry := SitemapURL{Loc: u.Loc}
			if t, err := parseSitemapTime(u.LastMod); err == nil {
				entry.LastMod = t
			}
			if f, err := strconv.ParseFloat(u.Prio, 64); err == nil {
				entry.Priority = f
			}
			sum.URLs = append(sum.URLs, entry)
		}
		return sum, nil
	}

	// Otherwise try sitemapindex.
	var idx sitemapIndexXML
	if err := xml.Unmarshal(body, &idx); err == nil && len(idx.Sitemaps) > 0 {
		sum.IsIndex = true
		subs := make([]string, 0, len(idx.Sitemaps))
		for _, s := range idx.Sitemaps {
			subs = append(subs, s.Loc)
		}
		return sum, subs
	}

	sum.Reachable = false
	return sum, nil
}

// parseSitemapTime accepts both RFC3339 and plain YYYY-MM-DD.
func parseSitemapTime(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, errEmptyTime
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}
	return time.Parse("2006-01-02", s)
}

var errEmptyTime = stringError("empty time")

type stringError string

func (e stringError) Error() string { return string(e) }
