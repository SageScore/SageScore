package scorer_test

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/iserter/sagescore/pkg/scorer"
	"github.com/iserter/sagescore/pkg/scorer/crawl"
	"github.com/iserter/sagescore/pkg/scorer/fetch"
)

// TestRun_PrintsRealAudit is a smoke check that prints a full audit
// against the synthetic site. Runs only with SAGE_SMOKE=1 so it's not
// noisy in normal `go test ./...`.
func TestRun_PrintsRealAudit(t *testing.T) {
	if os.Getenv("SAGE_SMOKE") == "" {
		t.Skip("set SAGE_SMOKE=1 to run")
	}
	srv := httptest.NewServer(syntheticMux())
	defer srv.Close()

	orig := crawl.HostDelay
	crawl.HostDelay = 0
	defer func() { crawl.HostDelay = orig }()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	audit, err := scorer.Run(ctx, srv.URL, scorer.Config{
		HTTPClient: fetch.NewClient(),
		Now:        time.Date(2026, 4, 25, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}
	b, _ := json.MarshalIndent(audit, "", "  ")
	t.Logf("\n%s\n", b)
}
