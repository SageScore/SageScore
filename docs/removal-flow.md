# Removal Flow Specification — `/remove`

**Version:** 1.0
**Last updated:** 2026-04-25
**Owner:** Ilyas Serter
**Status:** Locked for v0.1 implementation in Phase 3.

This document is the source of truth for the removal flow. The Phase 3 implementation must match this spec; any deviation needs a doc update first.

---

## Goals

1. **Frictionless for legitimate owners.** One form, one field, no account, takes effect in seconds.
2. **Resistant to abuse.** A random visitor cannot deindex a third party's audit.
3. **GDPR-compliant.** 30-day hard-delete SLA, audit trail of who-requested-what.
4. **Transparent.** Owner sees clear feedback at each step.

---

## User journey (happy path)

```
[Owner views sagescore.org/example.com]
              │
              ▼  click "Remove this audit"
[GET /remove?domain=example.com]
              │
              ▼  enter email, submit
[POST /remove]
              │
              ├─ noindex set immediately on audits.example.com
              ├─ Cloudflare cache purged
              ├─ Verification email sent
              │
              ▼  
[Pending page: "Check your inbox"]
              │
              ▼  owner clicks email link
[GET /remove/confirm?token=...]
              │
              ▼
[Confirmed page: "Your audit is removed; data deletes within 30 days"]
              │
              ▼  (30 days later, nightly reaper)
[Hard delete of audit row + cascaded page audits]
```

---

## Endpoint specifications

### `GET /remove`

**Purpose:** Render the removal form.
**Query params:** `?domain=<domain>` optional pre-fill.
**Response:** HTML form with one email field, hidden `domain` field, CSRF token, submit button.
**Cache headers:** `Cache-Control: no-store`.
**Indexability:** `<meta name="robots" content="noindex,nofollow">`.

### `POST /remove`

**Purpose:** Accept a removal request, immediately deindex the audit, send a verification email.
**Body:** `domain` (required, matches an existing audit), `email` (required, RFC 5322 valid), `csrf_token` (required).
**Rate limit:** 5 requests per IP-hash per 24 hours.
**Validation:**
- Reject if `domain` does not have an `Audit` row.
- Reject if `email` fails RFC 5322 syntactic validation.
- Reject if rate limit exceeded → HTTP 429 with `Retry-After`.

**Side effects (in order, atomically per database where possible):**
1. Insert `RemovalRequest` row: `(domain, email, token, requested_at, ip_hash)`. `token` is 32 random bytes hex-encoded.
2. Set `audits.noindex = true` for the requested domain.
3. Async: purge Cloudflare cache for `https://sagescore.org/<domain>`.
4. Async: send verification email via Resend with link `https://sagescore.org/remove/confirm?token=<token>`.

The audit row is **already** marked `noindex` after step 2. The verification step exists to confirm intent and to satisfy our audit trail — it is not the trigger for deindexing.

**Response:** HTTP 200 with the "removal pending" page. `Cache-Control: no-store`.

### `GET /remove/confirm?token=<token>`

**Purpose:** Mark a removal request as confirmed.
**Validation:**
- `token` must match an existing `RemovalRequest.token`.
- `RemovalRequest.requested_at` must be within last 7 days.
- `RemovalRequest.confirmed_at` must be `NULL` (single-use confirmation).

**Side effects:**
1. Set `RemovalRequest.confirmed_at = now()`.

The `audits.noindex` flag was already set in `POST /remove`; this step does not change it.

**Response:** HTTP 200 confirmation page. If token invalid/expired/already-used: HTTP 410 Gone with a polite "this link is no longer valid" message.

### Nightly reaper (`cmd/sagescore-web --reap`)

**Schedule:** Daily at 03:00 UTC, via systemd timer.
**Logic:**
```
SELECT * FROM removal_requests
  WHERE confirmed_at IS NOT NULL
    AND deleted_at IS NULL
    AND confirmed_at < NOW() - INTERVAL '30 days'
```
For each row:
1. Hard-delete `Audit` row matching `domain` (cascades to `PageAudit` via FK).
2. Set `RemovalRequest.deleted_at = NOW()`.
3. Log: `removal.reaped domain=... request_id=...`.

The first 7 days of production runs the reaper in **dry-run mode**: it logs what it *would* delete, but doesn't. Promote to live deletion on day 8 after a sanity check.

---

## Email template

**Sender:** `SageScore <noreply@sagescore.org>` (Resend, EU region).
**Subject:** `Confirm removal of your SageScore audit for {{domain}}`
**Body (plaintext + HTML versions):**

```
Hi,

You (or someone using this email) requested removal of the SageScore audit
for {{domain}}.

We've already de-listed the audit page from search engines. To complete
the removal — including hard-deletion of all stored data within 30 days —
please confirm by clicking the link below.

→ https://sagescore.org/remove/confirm?token={{token}}

This link is valid for 7 days and can only be used once.

If you didn't request this, you can safely ignore this email.

Questions? Reply to this email or write to contact@sagescore.org.

— SageScore (https://sagescore.org)
```

---

## Anti-abuse controls

| Control | Implementation |
|---|---|
| **Per-IP rate limit** | 5 removal requests per IP-hash per 24 hours, enforced via `RateLimitRepo`. |
| **Per-domain rate limit** | 3 unconfirmed removal requests per domain per 24 hours; further requests get HTTP 429 until one confirms or all expire. |
| **Email verification** | Required before hard delete; deindex happens immediately so an unverified request still respects the owner's wishes provisionally. |
| **Email-domain matching (soft signal)** | Requests where the email's domain matches the audited domain (or common admin variants like `hostmaster@`, `webmaster@`, `postmaster@`) are flagged in logs as "high confidence". Other requests are processed normally but logged for review. |
| **Token security** | 256-bit random token, HMAC-validated, single-use, 7-day TTL. |
| **No domain-bouncing** | The `domain` field cannot be edited in the form — it's bound to the page URL where the form was loaded. |
| **CSRF** | Standard double-submit cookie + form token. |

### What the controls deliberately do NOT do

- We do **not** require the requester to prove WHOIS ownership. Most modern domains have privacy-protected WHOIS, making this both burdensome and ineffective.
- We do **not** require the requester to upload a verification file to their domain root, because that would require server access many marketers don't have. The `noindex`-on-submit + email-verification combination is the proportionate response.
- We do **not** retain a permanent block-list of "removed" domains, because re-auditing should be possible if a site owner later wants to re-include their domain.

---

## Edge cases

| Case | Behaviour |
|---|---|
| User submits `/remove` for a domain that has no audit | HTTP 404; "We don't have an audit for that domain. Nothing to remove." |
| User submits twice in succession before email arrives | Second submission rate-limited per-domain (max 3 unconfirmed). The latest token is the live one. |
| Email link clicked twice | First click confirms; second click renders "This link has already been used." |
| Email link clicked after 7 days | Renders "This link has expired. To re-request removal, please submit the form again." |
| Audited site re-audits itself via `/recrawl` after removal-confirmed | Recrawl is rejected: confirmed-removal rows show "Owner has opted out". The owner must email `contact@sagescore.org` to reverse. |
| Cloudflare purge API fails | Logged at WARN; does not block the user flow. The cached page expires naturally within 24 hours. |
| Resend email-send fails | Logged at ERROR; user shown "We've de-listed the page but the verification email failed to send. Please try again or contact us." The de-list is already in effect. |
| Reaper fails (DB error) | Failure is logged at ERROR; pages a human via Sentry. Reaper retries on the next nightly run. The 30-day SLA has slack (we run nightly; one missed run is still well within 30 days). |
| Owner wants to *un-remove* (re-include) | Out of scope for v0.1 self-serve. Manual process via `contact@sagescore.org`. |

---

## Audit trail

The `removal_requests` table is the audit log. It retains:
- Domain
- Email (kept as-is — submitted by the data subject for the purpose of fulfilling their right)
- IP-hash (not raw IP)
- Timestamps (requested, confirmed, deleted)

This row is itself purged 30 days after `deleted_at` is set, satisfying minimisation while leaving a 30-day window for any post-deletion enquiries.

---

## What a regulator audit would see

If a UK/EU data-protection authority audits us, the documents that together prove compliance:

1. This spec (`docs/removal-flow.md`).
2. `docs/lia.md` (the legitimate-interest assessment).
3. `docs/privacy.md` (the public privacy policy).
4. The `removal_requests` table schema and reaper job.
5. Logs of reaper runs (kept 90 days).
6. Source-of-truth code in `pkg/web/handlers/remove.go` and `cmd/sagescore-web --reap`.

The narrative we'd present: "Our crawler operates under industry-standard manners; we publish only structural facts about HTML; site owners have a one-form, no-account, sub-second deindex path; full data removal within 30 days; documented audit trail."
