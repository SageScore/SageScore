package crawl

import (
	"testing"
	"time"

	"github.com/iserter/sagescore/pkg/scorer/parse"
)

func TestBuildPlan_FindsThreeArticlesFromSitemap(t *testing.T) {
	sm := &parse.SitemapSummary{
		Reachable: true,
		URLs: []parse.SitemapURL{
			{Loc: "https://x.com/", LastMod: time.Now()},
			{Loc: "https://x.com/blog/one", LastMod: time.Now()},
			{Loc: "https://x.com/blog/two", LastMod: time.Now()},
			{Loc: "https://x.com/articles/three", LastMod: time.Now()},
			{Loc: "https://x.com/blog/four", LastMod: time.Now()},
		},
	}
	plan := BuildPlan("https://x.com", sm, nil)
	if got := len(plan.Articles); got != 3 {
		t.Fatalf("expected 3 articles, got %d: %v", got, plan.Articles)
	}
}

func TestBuildPlan_FallsBackToAnchors(t *testing.T) {
	anchors := []parse.Anchor{
		{Href: "/posts/foo", Internal: true},
		{Href: "/posts/bar", Internal: true},
		{Href: "/posts/baz", Internal: true},
	}
	plan := BuildPlan("https://y.com", nil, anchors)
	if got := len(plan.Articles); got != 3 {
		t.Fatalf("expected 3 articles from anchors, got %d: %v", got, plan.Articles)
	}
}

func TestBuildPlan_DeterministicOrder(t *testing.T) {
	anchors := []parse.Anchor{
		{Href: "/blog/zeta", Internal: true},
		{Href: "/blog/alpha", Internal: true},
		{Href: "/blog/beta", Internal: true},
	}
	a := BuildPlan("https://z.com", nil, anchors)
	b := BuildPlan("https://z.com", nil, anchors)
	if len(a.Articles) != len(b.Articles) {
		t.Fatal("plan length differs across runs")
	}
	for i := range a.Articles {
		if a.Articles[i] != b.Articles[i] {
			t.Fatalf("non-deterministic order at %d: %q vs %q", i, a.Articles[i], b.Articles[i])
		}
	}
}

func TestBuildPlan_EnforcesMaxPages(t *testing.T) {
	sm := &parse.SitemapSummary{Reachable: true}
	for i := 0; i < 50; i++ {
		sm.URLs = append(sm.URLs, parse.SitemapURL{Loc: "https://big.com/page/" + itoa(i)})
	}
	plan := BuildPlan("https://big.com", sm, nil)
	if plan.Total > MaxPages {
		t.Fatalf("plan exceeded MaxPages: %d", plan.Total)
	}
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	s := ""
	for i > 0 {
		s = string(rune('0'+i%10)) + s
		i /= 10
	}
	return s
}
