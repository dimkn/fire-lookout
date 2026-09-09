# Security hardening backlog

Findings from a review of the containerized setup (the `infra/` pillar, landed in `0.10.0`).
Each section is an independently shippable slice — pick one, do it, tick it off.

**These were verified against a running container**, not read off the Dockerfile. Where a
finding has a measurement, the reproduction command is included so nobody has to re-derive
it. Nothing here is known to be exploited; this is a backlog, not an incident.

Conventions that apply to every slice below: TDD (AGENTS.md §3 rule 4), bump `VERSION` for
anything observable (§7), and update AGENTS.md in the same change when a decision moves (§3
rule 7).

| # | Severity | Finding | Origin |
| --- | --- | --- | --- |
| [1](#1--api-published-on-every-interface-without-auth) | High | API published on every interface, no auth | Introduced by `0.10.0` |
| [2](#2--ssrf-the-feed-fetcher-dials-any-address) | High | SSRF — fetcher dials any address | Pre-existing, amplified by `0.10.0` |
| [3](#3--no-container-hardening-flags) | Medium | No container hardening flags | Gap |
| [4](#4--content_html-is-stored-and-served-verbatim) | Medium | `content_html` stored and served verbatim | Pre-existing, latent |
| [5](#5--no-security-response-headers) | Low | No security response headers | Gap |
| [6](#6--floating-base-image-tags) | Low | Floating base image tags | Introduced by `0.10.0` |
| [7](#7--goembed-alldist-also-embeds-dotfiles) | Info | `go:embed all:dist` embeds dotfiles | Introduced by `0.10.0` |

Suggested order: **1 → 3 → 5 → 7** are small and contained (compose file + the `webui`
handler) and can go in one pass. **2** and **4** change backend behavior and each deserve
their own slice with tests. **6** is best done together with CI.

---

## 1 — API published on every interface, without auth

**Severity: High.** `docker-compose.yml` publishes `0.0.0.0:8080->8080/tcp`, and
`api/openapi.yaml` declares no security schemes, so the API is fully open — list, subscribe,
update, delete — to anyone who can route to the host. On a café or office network that is
everyone. This also turns finding 2 from a local-only issue into a remotely triggerable one.

It also contradicts AGENTS.md §1: *"It runs entirely on a local machine — there is no hosted
deployment."* The compose file should match that claim.

Reproduce:

```bash
docker inspect rss-reader-rss-reader-1 --format '{{json .NetworkSettings.Ports}}'
```

**Fix** — bind the published port to loopback in [docker-compose.yml](../docker-compose.yml):

```yaml
ports:
  - "127.0.0.1:${RSS_READER_PORT:-8080}:8080"
```

Keep the container listening on `:8080` internally (`RSS_READER_ADDR`) — it is the *host*
side of the mapping that needs narrowing, and hardcoding the container side to loopback
would break the port mapping entirely.

**Done when:** `curl` from another machine on the LAN cannot reach it, `curl localhost:8080`
still can, and AGENTS.md §9 "Infra" notes that exposing it beyond loopback needs auth first.

---

## 2 — SSRF: the feed fetcher dials any address

**Severity: High.** `validateFeedURL` at
[backend/internal/domain/input.go:69](../backend/internal/domain/input.go#L69) checks only
that the scheme is `http`/`https` and the host is non-empty. Anything else — `127.0.0.1`,
`192.168.x.x`, `169.254.169.254`, an internal hostname — is accepted and fetched.

Containerizing did not create this, but it does amplify it: the service now sits on a Docker
bridge with routes to the host and the LAN, and `restart: unless-stopped` keeps it running
around the clock.

**The API itself leaks nothing** — every failure maps to the same generic
`feed_unreachable` body, which is good handler design. Two channels remain:

*Timing.* Verified against the running container:

| Target | Result |
| --- | --- |
| `http://127.0.0.1:9999/` (refused) | 0.003 s |
| `http://127.0.0.1:8080/` (open) | 0.003 s |
| `http://192.0.2.1:80/` (filtered, RFC 5737) | 10.004 s |

```bash
curl -s -o /dev/null -w "%{time_total}s\n" -X POST localhost:8080/api/feeds \
  -H 'content-type: application/json' -d '{"url":"http://192.0.2.1:80/","title":"probe"}'
```

A 3000× gap maps which hosts are alive. It does not reliably separate open from closed ports
on a live host, but it does enumerate the network.

*Exfiltration.* Any internal endpoint serving valid RSS, Atom, **or JSON Feed** (gofeed
accepts all three and is deliberately lenient) is parsed, stored, and readable afterwards
through `GET /api/feeds/{feedId}/items`.

**Fix — do it at the dial, not at validation.** Two traps make the obvious fix wrong:

- Validating the hostname then resolving it later is a TOCTOU (DNS rebinding).
- `feed.Fetcher` uses a default `http.Client`, which follows up to 10 redirects, so a
  validated public URL can redirect straight to `127.0.0.1`.

Both close with a single hook in
[backend/internal/adapters/feed/fetcher.go](../backend/internal/adapters/feed/fetcher.go):
give the client a transport whose `net.Dialer` sets `Control`. It runs *after* DNS
resolution and *before* connect, on every hop including redirects, and receives the actual
IP — so one check covers rebinding and redirects with no per-hop logic:

```go
dialer := &net.Dialer{
    Control: func(network, address string, _ syscall.RawConn) error {
        // reject loopback, private, link-local, unique-local, CGNAT, unspecified
    },
}
```

`netip.Addr` has most of the predicates built in (`IsLoopback`, `IsPrivate`,
`IsLinkLocalUnicast`, `IsUnspecified`); CGNAT (`100.64.0.0/10`) needs an explicit prefix.
Note this stays out of `domain/`, which may import only stdlib and must not do I/O — the
existing syntactic check there is fine as-is.

**Escape hatch.** Polling a status page on your own LAN is a legitimate use of this tool, so
this must be opt-out: `RSS_READER_ALLOW_PRIVATE_IPS=true`, defaulting to deny. Document it
in AGENTS.md §9 alongside the other env vars.

**Tests:** table-driven over the address predicate (loopback / private / link-local / CGNAT /
public); an `httptest` server that 302s to `127.0.0.1` must be refused; the allow-flag path
must permit a private address. The predicate should be a pure function so most of this needs
no network.

**Done when:** subscribing to `http://127.0.0.1:*` is refused with a clear validation error
rather than a generic `feed_unreachable`, a redirect to a private address is refused
mid-chain, and the opt-out flag re-enables both.

---

## 3 — No container hardening flags

**Severity: Medium.** The container already runs as uid 65532 with a correctly owned volume,
but keeps a writable root filesystem, the full default capability set, and no resource
limits — so a memory-exhausting feed or an RCE has more room than it needs.

**Verified working.** All of the following were applied to the running container and the app
stayed fully functional: SPA `200`, `/api/overview` `200`, and a real DB write (`PATCH
/api/feeds/1` advanced `updated_at`). **No `tmpfs` was needed** — SQLite writes only beside
the database, and the WAL/SHM sidecars live on the `/data` volume.

```yaml
read_only: true
cap_drop: [ALL]
security_opt: ["no-new-privileges:true"]
mem_limit: 256m
pids_limit: 128
```

**Done when:** the flags are in [docker-compose.yml](../docker-compose.yml),
`docker inspect` reports `ReadonlyRootfs=true` and a non-empty `CapDrop`, and the
subscribe → restart → still-there check from AGENTS.md §9 still passes.

---

## 4 — `content_html` is stored and served verbatim

**Severity: Medium — latent, not currently exploitable.** `api/openapi.yaml` describes
`content_html` as *"Raw entry body (all updates), preserved verbatim"*, and it is stored and
returned unsanitized. The content is fully attacker-controlled: whoever runs the feed a user
subscribed to writes it.

**Not exploitable today.** There is no `v-html`, `innerHTML`, or equivalent sink anywhere in
`frontend/src/` — confirmed by grep. The risk is purely that the first component to render
rich content turns this into stored XSS, and that component's author will reasonably assume
the backend already sanitized.

```bash
grep -rn "v-html\|innerHTML" frontend/src/   # must stay empty, or #4 must be done first
```

**Fix**, in rough order of preference:

1. Sanitize on ingest in the `feed` adapter (allowlist of tags/attributes), storing only
   what is safe to render. Adding a sanitizer is a new backend dependency and so needs the
   justification AGENTS.md §3 rule 3 requires — note it in the PR.
2. Or keep the raw value but rename the field to make the hazard obvious and put an explicit
   "must be sanitized before rendering" warning in the schema description.

Either way, `content_text` (already stripped via
[striptags.go](../backend/internal/adapters/feed/striptags.go)) stays the safe default for
any UI that only needs a preview.

**Done when:** either the stored value is safe to render, or the contract says in so many
words that it is not — and a test covers a `<script>`-bearing feed entry end to end.

---

## 5 — No security response headers

**Severity: Low.** The container returns none of Content-Security-Policy,
X-Content-Type-Options, X-Frame-Options, or Referrer-Policy (0 of 4).

```bash
curl -s -D- -o /dev/null localhost:8080/ | grep -iE "content-security-policy|x-content-type|x-frame|referrer"
```

**Fix** in [backend/internal/adapters/webui/webui.go](../backend/internal/adapters/webui/webui.go),
where the SPA response headers are already set. A strict CSP is realistic here: the app has
no inline scripts, no external origins, and no CDN — `default-src 'self'` should hold as-is.
Verify in the browser console after applying; Vite injects a module script tag but no inline
code. `X-Content-Type-Options: nosniff` is worth having regardless, and pairs with the
extension-aware 404 behavior the handler already implements.

This also meaningfully blunts finding 4.

**Done when:** the four headers are present on SPA responses, the app still renders with a
clean console, and a `webui` test asserts them.

---

## 6 — Floating base image tags

**Severity: Low.** [infra/Dockerfile](../infra/Dockerfile) uses `node:22-alpine`,
`golang:1.26-alpine`, and `gcr.io/distroless/static-debian12:nonroot` — all mutable. Builds
are not reproducible, and a compromised upstream tag lands silently.

**Fix:** pin by digest (`node:22-alpine@sha256:…`). This is deliberately *not* done yet,
because digest pins rot without something to bump them — pair it with the CI workflow
(AGENTS.md §2 still lists `.github/workflows/ci.yml` as unwritten) and a bump job, or accept
the floating tags as a conscious trade-off and record that in AGENTS.md.

**Done when:** either all three are digest-pinned with a documented bump path, or AGENTS.md
records the decision not to.

---

## 7 — `go:embed all:dist` also embeds dotfiles

**Severity: Info.** The `all:` prefix in
[backend/internal/adapters/webui/webui.go](../backend/internal/adapters/webui/webui.go) is
required — it is what lets `go:embed` accept a `dist/` directory containing only `.gitkeep`
on a clean checkout. The side effect is that *every* dotfile in `dist/` is compiled into the
binary and served.

Vite emits no dotfiles today (verified against `frontend/dist`), so nothing leaks now. But a
stray `.env` in the build output would be baked into the binary and served at `/.env`.

```bash
find frontend/dist -name ".*" -type f   # must stay empty
```

**Fix:** refuse to serve any path with a dot-prefixed segment in the `webui` handler, before
the file lookup. Cheap, and removes the whole class rather than relying on Vite's behavior
staying the same.

**Done when:** a `fstest.MapFS` test proves `/.env` and `/assets/.secret` are refused while
normal assets still serve.

---

## Checked and found clean

Recorded so this does not get re-audited:

- **No SQL injection surface** — the storage adapter builds no SQL by string concatenation;
  `gosec` G201/G202 are enabled and passing.
- **No XSS sink in the frontend** — no `v-html` / `innerHTML` anywhere in `frontend/src/`.
- **Fetch is bounded** — 5 MiB body cap and a 10 s timeout in `feed/fetcher.go`, applied to
  both the parse path and the drain-on-close path.
- **Container identity and storage** — runs as uid 65532; `/data` and the SQLite files are
  owned by 65532:65532, so the volume needs no runtime `chown`.
- **Build context** — no secrets, no `.git`, no `node_modules` reach the image
  (see [.dockerignore](../.dockerignore)).
- **Path traversal** — the `webui` handler cleans paths and is covered by a test that proves
  it cannot escape the embedded root.
