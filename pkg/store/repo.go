// Package store is the persistence layer for SageScore.
//
// Phase 0 status: interfaces only. GORM-backed implementations land in
// Phase 2. The interfaces here are the public contract; the rest of the
// app depends on these, never on GORM directly.
//
// Driver swap path: SQLite today, Postgres tomorrow, via env-var-selected
// driver in Open() (Phase 2).
package store

import (
	"context"
	"errors"
	"time"
)

// ErrNotFound is returned when a lookup by primary key (or unique key)
// finds no row. Repositories MUST return this exact error so callers can
// branch on it.
var ErrNotFound = errors.New("store: not found")

// AuditRepo persists scoring engine results. Audits are keyed by domain.
type AuditRepo interface {
	// Get returns the stored audit for a domain, or ErrNotFound.
	Get(ctx context.Context, domain string) (*Audit, error)

	// Upsert writes an audit and its associated PageAudits in one
	// transaction. Existing PageAudits for the domain are replaced.
	Upsert(ctx context.Context, a *Audit) error

	// MarkNoindex flips the domain to noindex without deleting content.
	// Used immediately on POST /remove (the deindex happens before
	// email verification).
	MarkNoindex(ctx context.Context, domain string) error

	// Delete hard-deletes the audit and its PageAudits. Called by the
	// reaper after the 30-day post-confirmation window.
	Delete(ctx context.Context, domain string) error

	// ListSitemap returns audit rows eligible for inclusion in the public
	// sitemap (noindex=false, opt_out=false, word_count>=800).
	ListSitemap(ctx context.Context, limit, offset int) ([]Audit, error)

	// ListSimilar returns up to `limit` rows for the "similar audits"
	// block, filtered by score band and TLD heuristics.
	ListSimilar(ctx context.Context, domain string, limit int) ([]Audit, error)
}

// RemovalRepo persists removal requests. Used to satisfy the GDPR
// erasure flow documented in docs/removal-flow.md.
type RemovalRepo interface {
	// Create inserts a new removal request with a fresh token.
	Create(ctx context.Context, r *RemovalRequest) error

	// ConfirmByToken marks the request as confirmed (single-use).
	ConfirmByToken(ctx context.Context, token string) (*RemovalRequest, error)

	// ListPendingDeletes returns confirmed requests older than 30 days
	// that haven't been deleted yet. Drives the reaper job.
	ListPendingDeletes(ctx context.Context, now time.Time) ([]RemovalRequest, error)

	// MarkDeleted records that the audit row has been hard-deleted.
	MarkDeleted(ctx context.Context, id uint64, when time.Time) error
}

// RecrawlRepo persists owner-signed re-audit tokens.
type RecrawlRepo interface {
	Create(ctx context.Context, t *RecrawlToken) error
	ConsumeByToken(ctx context.Context, token string) (*RecrawlToken, error)
}

// RateLimitRepo gates abuse on /audit, /remove, etc. Bucket strings:
// "audit", "remove", "api".
type RateLimitRepo interface {
	// Check increments the counter for (ipHash, bucket) within the
	// current window and returns whether the limit was exceeded.
	Check(ctx context.Context, ipHash, bucket string, window time.Duration, limit int) (allowed bool, err error)
}

// --- DTOs (the row-shaped Go structs) -------------------------------------
//
// These mirror the GORM models that will be defined in Phase 2. Keeping
// the DTOs separate from gorm.io/gorm tags lets the rest of the app stay
// decoupled from the ORM.

// Audit is the persisted form of a scorer.Audit.
type Audit struct {
	Domain        string
	Score         int
	ResultJSON    string
	FetchedAt     time.Time
	ScorerVersion string
	Noindex       bool
	OptOut        bool
	WordCount     int
	Pages         []PageAudit
}

// PageAudit is the persisted form of a scorer.PageAudit.
type PageAudit struct {
	ID            uint64
	Domain        string
	URL           string
	URLHash       string
	Kind          string
	Score         int
	SubscoresJSON string
	WordCount     int
	StatusCode    int
	FetchedAt     time.Time
	ScorerVersion string
}

// RemovalRequest is one row in the removal-flow audit log.
type RemovalRequest struct {
	ID          uint64
	Domain      string
	Email       string
	Token       string
	RequestedAt time.Time
	ConfirmedAt *time.Time
	DeletedAt   *time.Time
	IPHash      string
}

// RecrawlToken is a single-use re-audit grant.
type RecrawlToken struct {
	Token     string
	Domain    string
	ExpiresAt time.Time
}
