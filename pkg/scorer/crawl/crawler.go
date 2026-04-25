package crawl

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/iserter/sagescore/pkg/scorer/fetch"
	"github.com/iserter/sagescore/pkg/scorer/parse"
)

// ErrOptedOut is returned by Run when the domain's robots.txt
// disallows SageScoreBot. Callers turn this into the "Owner has opted
// out" rendering (PRD §6 robots-compliance).
var ErrOptedOut = errors.New("crawl: SageScoreBot disallowed by robots.txt")

// HostDelay is the minimum wall-clock gap between same-host requests
// (Technical Plan §3.4 and PRD §6). Exposed as a var for tests.
var HostDelay = 5 * time.Second

// Result bundles every fetch for one domain.
type Result struct {
	Origin   string
	Robots   *parse.RobotsSummary
	Sitemap  *parse.SitemapSummary
	Fetches  []FetchedURL
	Articles []string // URLs classified as articles
	Errors   []string
}

// FetchedURL pairs a Response with its classification.
type FetchedURL struct {
	URL      string
	Response *fetch.Response
	Page     *parse.ParsedPage // nil for non-HTML responses
	Kind     PageKind
	Source   ArticleSource // how this URL entered the plan
}

// PageKind classifies a sampled URL.
type PageKind string

const (
	PageHomepage PageKind = "homepage"
	PageAbout    PageKind = "about"
	PageArticle  PageKind = "article"
	PageOther    PageKind = "other"
	PageRobots   PageKind = "robots"
	PageSitemap  PageKind = "sitemap"
	PageLLMSTxt  PageKind = "llms"
)

// ArticleSource records which discovery path introduced a URL.
type ArticleSource string

const (
	SourceAlwaysFetch ArticleSource = "always"
	SourceArticle     ArticleSource = "article"
	SourceExtra       ArticleSource = "extra"
)

// Run executes the full crawl sequence for a domain: robots → sitemap
// → homepage → plan → remaining pages. Respects rate limits and caps.
func Run(ctx context.Context, origin string, client *fetch.Client) (*Result, error) {
	origin = strings.TrimRight(origin, "/")
	res := &Result{Origin: origin}

	// 1. robots.txt first.
	robotsURL := origin + "/robots.txt"
	robotsResp, _ := client.Fetch(ctx, robotsURL)
	var robotsBody []byte
	if robotsResp != nil && robotsResp.StatusCode == 200 {
		robotsBody = robotsResp.Body
	}
	res.Robots = parse.ParseRobots(robotsBody)

	// Hard opt-out if SageScoreBot is disallowed at "/".
	if !res.Robots.AllowedForSageScoreBot("/") {
		return res, ErrOptedOut
	}

	sleep(ctx)

	// 2. sitemap.xml (we keep the raw summary for the sampler).
	var sitemap *parse.SitemapSummary
	smURL := origin + "/sitemap.xml"
	smResp, _ := client.Fetch(ctx, smURL)
	if smResp != nil && smResp.StatusCode == 200 {
		s, subs := parse.ParseSitemap(smResp.Body)
		sitemap = s
		// Resolve one level of index.
		if s.IsIndex && len(subs) > 0 {
			for _, sub := range subs {
				if len(sitemap.URLs) >= 1000 {
					break
				}
				sleep(ctx)
				subResp, _ := client.Fetch(ctx, sub)
				if subResp == nil || subResp.StatusCode != 200 {
					continue
				}
				s2, _ := parse.ParseSitemap(subResp.Body)
				if s2 != nil {
					sitemap.URLs = append(sitemap.URLs, s2.URLs...)
				}
			}
		}
	}
	res.Sitemap = sitemap

	// 3. llms.txt (boolean presence only).
	sleep(ctx)
	llmsResp, _ := client.Fetch(ctx, origin+"/llms.txt")
	res.Robots.HasLLMSTxt = llmsResp != nil && llmsResp.StatusCode == 200 && len(llmsResp.Body) > 0

	sleep(ctx)
	llmsFullResp, _ := client.Fetch(ctx, origin+"/llms-full.txt")
	res.Robots.HasLLMSFullTxt = llmsFullResp != nil && llmsFullResp.StatusCode == 200 && len(llmsFullResp.Body) > 0

	// 4. Homepage (always).
	sleep(ctx)
	homeResp, _ := client.Fetch(ctx, origin+"/")
	var homeAnchors []parse.Anchor
	if homeResp != nil && homeResp.StatusCode == 200 && homeResp.IsHTML() {
		page, err := parse.ParsePage(origin+"/", homeResp.Body)
		if err == nil {
			homeAnchors = page.Anchors
			res.Fetches = append(res.Fetches, FetchedURL{
				URL:      origin + "/",
				Response: homeResp,
				Page:     page,
				Kind:     PageHomepage,
				Source:   SourceAlwaysFetch,
			})
		}
	}

	// 5. Build plan from sitemap + anchors.
	plan := BuildPlan(origin, sitemap, homeAnchors)

	// 6. Fetch the rest of the plan (everything except homepage, which is done).
	for _, u := range plan.All() {
		if u == origin+"/" {
			continue
		}
		if u == robotsURL {
			// Already fetched; fold robots into the record list.
			if robotsResp != nil {
				res.Fetches = append(res.Fetches, FetchedURL{
					URL: u, Response: robotsResp, Kind: PageRobots,
					Source: SourceAlwaysFetch,
				})
			}
			continue
		}
		if u == smURL {
			if smResp != nil {
				res.Fetches = append(res.Fetches, FetchedURL{
					URL: u, Response: smResp, Kind: PageSitemap,
					Source: SourceAlwaysFetch,
				})
			}
			continue
		}
		if u == origin+"/llms.txt" {
			if llmsResp != nil {
				res.Fetches = append(res.Fetches, FetchedURL{
					URL: u, Response: llmsResp, Kind: PageLLMSTxt,
					Source: SourceAlwaysFetch,
				})
			}
			continue
		}
		if !res.Robots.AllowedForSageScoreBot(urlPath(u)) {
			res.Errors = append(res.Errors, "robots-disallowed: "+u)
			continue
		}
		sleep(ctx)
		resp, err := client.Fetch(ctx, u)
		if err != nil || resp == nil {
			if err != nil {
				res.Errors = append(res.Errors, u+": "+err.Error())
			}
			continue
		}
		if resp.StatusCode != 200 || !resp.IsHTML() {
			res.Fetches = append(res.Fetches, FetchedURL{
				URL: u, Response: resp,
				Kind: PageOther, Source: sourceFor(u, plan),
			})
			continue
		}
		page, err := parse.ParsePage(u, resp.Body)
		if err != nil {
			res.Errors = append(res.Errors, u+": parse: "+err.Error())
			continue
		}
		kind := classify(u, page, plan)
		f := FetchedURL{
			URL: u, Response: resp, Page: page, Kind: kind, Source: sourceFor(u, plan),
		}
		res.Fetches = append(res.Fetches, f)
		if kind == PageArticle {
			res.Articles = append(res.Articles, u)
		}
	}

	return res, nil
}

func classify(u string, page *parse.ParsedPage, plan Plan) PageKind {
	low := strings.ToLower(urlPath(u))
	switch {
	case low == "/about" || low == "/about-us" || low == "/about/" || low == "/about-us/":
		return PageAbout
	}
	// Article: must have been discovered as one AND have strong on-page signal.
	inArticlePlan := false
	for _, a := range plan.Articles {
		if a == u {
			inArticlePlan = true
			break
		}
	}
	if inArticlePlan {
		// Need either Article JSON-LD OR <article> with ≥300 words (methodology).
		if hasArticleSchema(page) {
			return PageArticle
		}
		if page.HasArticleTag && page.ArticleWordN >= 300 {
			return PageArticle
		}
	}
	return PageOther
}

func hasArticleSchema(page *parse.ParsedPage) bool {
	for _, obj := range page.JSONLD {
		t, ok := obj["@type"]
		if !ok {
			continue
		}
		switch tv := t.(type) {
		case string:
			if tv == "Article" || tv == "BlogPosting" || tv == "NewsArticle" {
				return true
			}
		case []any:
			for _, x := range tv {
				if s, ok := x.(string); ok {
					if s == "Article" || s == "BlogPosting" || s == "NewsArticle" {
						return true
					}
				}
			}
		}
	}
	return false
}

func sourceFor(u string, plan Plan) ArticleSource {
	for _, a := range plan.AlwaysFetch {
		if a == u {
			return SourceAlwaysFetch
		}
	}
	for _, a := range plan.Articles {
		if a == u {
			return SourceArticle
		}
	}
	return SourceExtra
}

func urlPath(u string) string {
	i := strings.Index(u, "://")
	if i < 0 {
		return u
	}
	rest := u[i+3:]
	if slash := strings.Index(rest, "/"); slash >= 0 {
		p := rest[slash:]
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

func sleep(ctx context.Context) {
	select {
	case <-ctx.Done():
	case <-time.After(HostDelay):
	}
}
