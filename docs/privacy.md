# Privacy Policy — SageScore

**Effective date:** 2026-06-30 (public beta).
**Last updated:** 2026-04-25.
**Controller:** Ilyas Serter, operating SageScore (`sagescore.org`).
**Contact:** `contact@sagescore.org`.

---

## 1. Summary in plain English

SageScore is a public website auditor. We fetch publicly available HTML, robots.txt, and sitemap files from websites, analyse them for AI-search readiness, and publish the result on a page like `sagescore.org/<domain>`. We do **not** collect personal data from third-party sites. The only personal data we hold belongs to people who interact with us directly: site owners requesting removal of their audit, or visitors of our own site whose privacy-respecting analytics events are recorded.

If you are a site owner and want your audit removed, use [`/remove`](/remove). It takes effect within seconds.

---

## 2. What data we process

### 2.1 Data from audited websites

We fetch and store **only public, machine-readable, factual signals** from third-party websites:

- HTML, headers, response status codes from public URLs.
- `robots.txt`, `sitemap.xml`, `llms.txt`, JSON-LD blocks, meta tags, anchors.
- Derived facts: word counts, presence/absence of schema, link counts.

We do **not** collect:
- User-generated content from third-party sites (comments, profiles, forum posts).
- Personal data of users of third-party sites.
- Anything behind authentication.
- Anything served only to authenticated or cookied sessions.

If a third-party site's HTML happens to contain an email address or phone number (e.g. on a contact page), we record this as a structural finding ("NAP completeness") but do not retain the email or phone number itself in any form linkable to a person.

### 2.2 Data from removal requests (`/remove`)

When you submit a removal request:

- **Email address** you supply: used to send a verification link.
- **IP address** of the submission: hashed (SHA-256 with a daily-rotated salt) and stored to enforce per-IP abuse limits. The raw IP is not retained.
- **Timestamp** of submission and confirmation.

### 2.3 Data from visitors to `sagescore.org`

We use **self-hosted Plausible Analytics** (EU). Plausible is configured cookie-less. It records:

- Pseudonymous, aggregated page-view counts.
- Referrer, country (derived from IP, IP itself not stored), browser family.
- Outbound link clicks on conversion CTAs.

No cookies are set. No fingerprinting. No advertising trackers. No data leaves our infrastructure.

---

## 3. Lawful basis (UK GDPR / EU GDPR)

| Processing | Lawful basis |
|---|---|
| Public-HTML fetching and audit publishing | **Legitimate interest** (Art. 6(1)(f)). Documented in our internal Legitimate-Interest Assessment. The interest is making AI-search readiness signals discoverable; the data is non-personal; site owners have a one-click removal path. |
| Removal-request email + IP-hash | **Compliance with legal obligation** (Art. 6(1)(c)) for fulfilling data-subject rights, and **legitimate interest** for abuse prevention. |
| Self-hosted Plausible analytics | **Legitimate interest** (Art. 6(1)(f)) for understanding aggregate site usage. No consent banner required because no cookies or device-identifiers are used. |

---

## 4. Retention

| Data | Retention |
|---|---|
| Audit content (HTML-derived findings, scores) | Until removal-confirmed + 30 days hard-delete SLA. Re-fetched every 30 days while live. |
| Removal-request email + IP-hash | 30 days after the audit row is hard-deleted, then purged. |
| Plausible analytics events | 12 months, then aggregated. |
| Server access logs | 14 days, then purged. |

---

## 5. Your rights

Under UK / EU GDPR you have rights to:

- **Access** the personal data we hold about you.
- **Rectification** of inaccurate data.
- **Erasure** (the "right to be forgotten").
- **Restriction** of processing.
- **Data portability**.
- **Object** to processing based on legitimate interest.
- **Withdraw consent** where we rely on consent.
- **Complain** to your local data-protection authority.

For audit-page erasure, the fastest path is the self-serve [`/remove`](/remove) form. For any other request, email `contact@sagescore.org`. We respond within 30 days.

---

## 6. International transfers

Our infrastructure (VPS, object storage, analytics) is in the EU. Email confirmation is sent via Resend's EU region. Cloudflare's EU edge handles caching. We do not transfer personal data outside the EEA / UK in the ordinary course of operation.

---

## 7. Sub-processors

| Sub-processor | Purpose | Region |
|---|---|---|
| Hetzner Online GmbH | VPS + object storage | EU (Germany / Finland) |
| Cloudflare, Inc. | CDN, DDoS protection (free tier) | Global edge; EU edge for EU users |
| Resend, Inc. | Removal-confirmation email | EU region selected |

We do not use Google Analytics, Facebook Pixel, advertising networks, or any tracker that sells data.

---

## 8. Security

- TLS everywhere; HTTPS-only with HSTS.
- SQLite encrypted at rest (LUKS on the VPS volume); replicated to encrypted object storage via litestream.
- Removal-request tokens are HMAC-signed and single-use.
- Rate limits and IP-hash abuse controls on `/remove` and `/audit`.

---

## 9. Children

SageScore is not directed at children under 16. We do not knowingly process children's personal data.

---

## 10. Changes to this policy

We will post material changes here with the new effective date and at least 14 days' notice for substantive changes affecting personal data.

---

## 11. Contact

`contact@sagescore.org`. For removal of an audit page, please use [`/remove`](/remove) — it is faster than email.
