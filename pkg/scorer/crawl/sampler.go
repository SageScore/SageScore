// Package crawl orchestrates fetching the per-domain page sample
// (Technical Plan §3.4, methodology.md §"Page sampling"). The sampler
// is a pure function so it's trivially unit-testable.
package crawl

import (
	"regexp"
	"sort"
	"strings"

	"github.com/iserter/sagescore/pkg/scorer/parse"
)

// ArticlePathPattern matches path segments where an article commonly lives.
var ArticlePathPattern = regexp.MustCompile(
	`(?i)/(blog|articles|posts|news|insights|guides|resources|knowledge)/[^/]+`,
)

// Plan is the ordered set of URLs we will fetch for a domain.
type Plan struct {
	AlwaysFetch []string // homepage, /about, /robots.txt, /sitemap.xml, /llms.txt
	Articles    []string // up to 3
	Extras      []string // fill up to MaxPages
	Total       int
}

// MaxPages is the per-domain fetch cap (Technical Plan §3.4).
const MaxPages = 10

// MinArticles is the minimum article count before we emit
// ARTICLES_INSUFFICIENT_SAMPLE (docs/methodology.md).
const MinArticles = 3

// BuildPlan computes a deterministic fetch plan for the domain.
//
// Inputs:
//   - scheme+host (e.g. "https://example.com")
//   - sitemap summary (may be nil/empty)
//   - anchors discovered on the homepage (may be nil/empty)
//
// Output: a Plan where AlwaysFetch comes first, then Articles (sorted
// for determinism), then Extras to pad up to MaxPages.
func BuildPlan(origin string, sitemap *parse.SitemapSummary, homeAnchors []parse.Anchor) Plan {
	origin = strings.TrimRight(origin, "/")

	plan := Plan{
		AlwaysFetch: []string{
			origin + "/",
			origin + "/about",
			origin + "/robots.txt",
			origin + "/sitemap.xml",
			origin + "/llms.txt",
		},
	}

	seen := map[string]struct{}{}
	for _, u := range plan.AlwaysFetch {
		seen[u] = struct{}{}
	}

	host := hostOf(origin)
	articles := discoverArticles(origin, host, sitemap, homeAnchors)
	for _, u := range articles {
		if _, dup := seen[u]; dup {
			continue
		}
		if len(plan.Articles) >= MinArticles {
			break
		}
		plan.Articles = append(plan.Articles, u)
		seen[u] = struct{}{}
	}

	// Fill extras from sitemap diversity picks.
	if sitemap != nil {
		for _, u := range sitemap.URLs {
			if plan.total() >= MaxPages {
				break
			}
			if hostOf(u.Loc) != host {
				continue
			}
			if _, dup := seen[u.Loc]; dup {
				continue
			}
			plan.Extras = append(plan.Extras, u.Loc)
			seen[u.Loc] = struct{}{}
		}
	}

	plan.Total = plan.total()
	return plan
}

// All returns the AlwaysFetch + Articles + Extras URLs in order.
func (p Plan) All() []string {
	out := make([]string, 0, p.total())
	out = append(out, p.AlwaysFetch...)
	out = append(out, p.Articles...)
	out = append(out, p.Extras...)
	return out
}

func (p Plan) total() int {
	return len(p.AlwaysFetch) + len(p.Articles) + len(p.Extras)
}

// discoverArticles runs the priority-ordered article discovery pipeline
// (Technical Plan §3.4 URL sampling). Results are returned in a stable
// deterministic order (sitemap order first, then lex order for
// anchor-derived URLs).
func discoverArticles(origin, host string, sitemap *parse.SitemapSummary, anchors []parse.Anchor) []string {
	var fromSitemap []string
	var fromSitemapLastMod []string
	var fromAnchors []string

	if sitemap != nil {
		for _, u := range sitemap.URLs {
			if hostOf(u.Loc) != host {
				continue
			}
			path := pathOf(u.Loc)
			if ArticlePathPattern.MatchString(path) {
				fromSitemap = append(fromSitemap, u.Loc)
				continue
			}
			if !u.LastMod.IsZero() && pathDepth(path) >= 2 {
				fromSitemapLastMod = append(fromSitemapLastMod, u.Loc)
			}
		}
	}
	// Sitemap order is meaningful (site-provided); no sort.

	for _, a := range anchors {
		if !a.Internal {
			continue
		}
		href := a.Href
		if !strings.HasPrefix(href, "http") {
			href = origin + "/" + strings.TrimLeft(href, "/")
		}
		path := pathOf(href)
		if ArticlePathPattern.MatchString(path) {
			fromAnchors = append(fromAnchors, href)
		}
	}
	sort.Strings(fromAnchors)

	out := []string{}
	seen := map[string]struct{}{}
	for _, batch := range [][]string{fromSitemap, fromSitemapLastMod, fromAnchors} {
		for _, u := range batch {
			if _, ok := seen[u]; ok {
				continue
			}
			out = append(out, u)
			seen[u] = struct{}{}
			if len(out) >= MinArticles {
				return out
			}
		}
	}
	return out
}

func hostOf(u string) string {
	i := strings.Index(u, "://")
	if i < 0 {
		return ""
	}
	rest := u[i+3:]
	end := len(rest)
	for j, r := range rest {
		if r == '/' || r == '?' || r == '#' {
			end = j
			break
		}
	}
	host := rest[:end]
	if at := strings.LastIndex(host, "@"); at >= 0 {
		host = host[at+1:]
	}
	if colon := strings.Index(host, ":"); colon >= 0 {
		host = host[:colon]
	}
	return strings.ToLower(host)
}

func pathOf(u string) string {
	i := strings.Index(u, "://")
	if i < 0 {
		return u
	}
	rest := u[i+3:]
	if slash := strings.Index(rest, "/"); slash >= 0 {
		p := rest[slash:]
		// strip query/fragment
		if q := strings.Index(p, "?"); q >= 0 {
			p = p[:q]
		}
		if h := strings.Index(p, "#"); h >= 0 {
			p = p[:h]
		}
		return p
	}
	return "/"
}

func pathDepth(p string) int {
	p = strings.Trim(p, "/")
	if p == "" {
		return 0
	}
	return strings.Count(p, "/") + 1
}
