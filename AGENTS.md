# AGENTS.md

Authoritative context for anyone — human or AI agent — working in this repository.
Read this before making changes. When a decision here changes, update this file in the
same change. If reality and this document disagree, treat it as a bug.

---

## 1. Overview

**fire-lookout** is a **local status page**. It periodically fetches RSS/Atom feeds from
remote sources (e.g. third-party service status pages) and presents their latest status
updates in one place. It runs entirely on a local machine — there is **no hosted
deployment**.

The project is a **single repository** with a strict separation into three pillars:

| Pillar          | Tech                          | Responsibility                                             |
| --------------- | ----------------------------- | ---------------------------------------------------------- |
| `backend/`      | Go, hexagonal architecture    | Fetch & parse feeds, persist state, expose an HTTP API     |
| `frontend/`     | Vue 3 + TypeScript + Vite     | Render the status page. **No business logic.**             |
| `infra/`        | Docker (single container)     | Build & run backend + frontend as one artifact            |

The backend and frontend communicate **only** through an HTTP contract defined once in
OpenAPI. Server stubs and the typed client are **generated** from that contract — never
hand-written.

---

## 2. Repository layout

```
/
├── AGENTS.md                      # This file — the single source of truth.
├── README.md                      # Short pointer to this file.
├── VERSION                        # Single semver version for the whole tool.
├── .gitignore
├── docker-compose.yml             # Local dev convenience (one service).
│
├── api/
│   └── openapi.yaml               # THE contract. Single source of truth for BE↔FE.
│
├── backend/                       # Go module. Hexagonal architecture.
│   ├── go.mod / go.sum
│   ├── cmd/
│   │   └── status-page/           # main(): wires adapters, embeds FE assets, serves.
│   └── internal/
│       ├── domain/                # Entities + PORTS (interfaces). No external deps.
│       ├── application/           # Use cases (fetch feeds, expose status). Orchestrates ports.
│       └── adapters/              # Implementations of ports (the "hexagon" edges).
│           ├── httpapi/           # DRIVING adapter: oapi-codegen stdlib net/http handlers.
│           ├── feed/              # DRIVEN adapter: gofeed-backed feed fetcher.
│           └── storage/           # DRIVEN adapter: SQLite repository.
│               └── migrations/    # Embedded *.sql schema migrations (goose).
│
├── frontend/                      # Vue 3 + Vite + TS. Managed with Yarn.
│   └── src/
│       ├── components/            # DESIGN SYSTEM. One folder per component + story + test.
│       ├── pages/                 # Compose components. No logic, no direct fetch.
│       ├── composables/          # Shared reactive UI state (e.g. the status-banner queue).
│       ├── utils/                 # Pure presentation helpers (e.g. timestamp formatting).
│       ├── styles/                # Global CSS custom properties + element defaults.
│       └── api/                   # schema.gen.ts is GENERATED; client.ts/status.ts are the
│                                  # thin hand-written transport seam (see §5).
│
├── infra/
│   └── Dockerfile                 # Multi-stage → single runtime container.
│
└── .github/
    └── workflows/
        └── ci.yml                 # Lint + test (BE & FE) + OpenAPI-drift check + docker build.
```

---

## 3. Golden rules

1. **The OpenAPI spec is the only contract.** `api/openapi.yaml` is the single source of
   truth for every backend↔frontend interaction. Change the contract there first, then
   regenerate both sides. Never hand-edit generated code.
2. **The frontend contains no business logic.** It renders data and calls the generated
   API client. Any decision, transformation, or rule lives in the backend.
3. **Minimal backend dependencies.** Prefer the Go standard library. A new third-party
   dependency needs a justification recorded in the PR. Current sanctioned deps:
   `gofeed` (feed parsing), `oapi-codegen` (codegen, build-time),
   `modernc.org/sqlite` (pure-Go, cgo-free SQLite driver — persistence),
   `goose` (SQL schema migrations), and `go-cmp` (test-only, optional).
4. **TDD by default.** Write the failing test first, then the implementation. The one
   sanctioned exception is UI visual behavior (see §8).
5. **`VERSION` is the single version source.** Everything derives from it (see §7).
6. **Keep the pillars separate.** No Go code reaches into `frontend/` sources and no
   frontend code assumes backend internals — the HTTP contract is the only seam.
7. **Update this file** whenever an architectural decision, command, or convention changes.

---

## 4. Backend conventions (`backend/`)

- **Architecture: hexagonal (ports & adapters).**
  - `domain/` — plain entities and **port interfaces**. Imports nothing outside the stdlib.
    Example ports: `FeedFetcher`, `StatusRepository`.
  - `application/` — use cases that orchestrate ports (e.g. "refresh all feeds",
    "get current status"). Depends on `domain/` only, never on adapters.
  - `adapters/` — concrete implementations that plug into ports:
    - `httpapi/` — **driving** side. Generated `net/http` server (Go 1.22+ `ServeMux`)
      from OpenAPI via `oapi-codegen`; thin handlers delegate to application use cases.
    - `feed/` — **driven** side. Wraps `gofeed` behind the `FeedFetcher` port.
    - `storage/` — **driven** side. Implements `StatusRepository` on a local SQLite
      database via stdlib `database/sql` with the pure-Go `modernc.org/sqlite` driver
      (cgo-free). Schema evolves through embedded `goose` migrations (see **Storage** below).
- **Dependency direction:** adapters → application → domain. Never the reverse. Wiring
  happens only in `cmd/status-page/main.go`.
- **HTTP server:** stdlib `net/http` only. No web framework (no gin/echo/chi).
- **Storage:** a single SQLite database file under a configurable data directory
  (defaults to `./data/fire-lookout.db`, gitignored), opened in **WAL mode** with **foreign
  keys enforced** — both pinned in the DSN, which FK `ON DELETE` actions depend on:
  `file:./data/fire-lookout.db?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)`. Accessed
  through stdlib `database/sql` with the `modernc.org/sqlite` driver — **no ORM**. SQLite
  (WAL) plus the `database/sql` pool handle concurrency, so no manual serialization is needed.
  Schema is managed by versioned SQL migrations under `internal/adapters/storage/migrations/`
  (embedded via `embed.FS`) applied on startup with `goose`. All reads/writes go through the
  repository adapter — nothing else touches the database.
- **Data model.** The authoritative schema is the goose migration in
  `internal/adapters/storage/migrations/` (starting at `00001_init_schema.sql`); the bullets
  below only summarize intent. Three tables:
  - `feed_group` — arbitrary user buckets. Row **`id = 0` is a seeded, undeletable
    `Ungrouped` sentinel** (a `BEFORE DELETE` trigger guards it).
  - `feed` — one row per subscribed RSS/Atom endpoint (`url` UNIQUE, and `title` unique
    **case-insensitively** via `idx_feed_title_unique` in migration `00002` — two systems may
    not share a display name). `group_id` is
    `NOT NULL DEFAULT 0` referencing `feed_group(id)` with `ON DELETE SET DEFAULT`, so
    deleting a group re-parents its feeds to `Ungrouped` rather than orphaning or deleting
    them. Health bookkeeping is a distinct trio: `last_fetched_at` (attempt),
    `last_success_at` (success), `last_error`.
  - `feed_item` — one row per incident/entry, keyed `UNIQUE(feed_id, guid)` for idempotent
    upserts (`ON CONFLICT(feed_id, guid) DO UPDATE`) across polls. Stores raw `content_html`
    as the source of truth plus a best-effort, nullable `current_status`; per-update
    timelines are intentionally **not** parsed (yet).
  - **Timestamps:** every time column is TEXT, RFC3339, **UTC**, second-precision; normalize
    on write (`t.UTC().Format(time.RFC3339)`). `group_id` is a plain `int64` in Go (`0` =
    ungrouped), never a nullable pointer.
  - **Retention:** after each successful poll, prune `feed_item` older than a configurable
    window (default ~7 days); `feed`/`feed_group` rows are never pruned.
  - **Fetching:** the poller always re-fetches and re-parses — no HTTP conditional-GET
    (ETag/Last-Modified) bookkeeping yet; add it if a provider rate-limits.
- **Subscribing is validate-then-write.** `POST /feeds` sanitises the input (every string
  trimmed, empties rejected, http(s) URLs only), rejects a duplicate URL or name, and only
  then fetches the endpoint once through the `FeedFetcher` port to prove it really is a feed.
  Cheapest checks first: a duplicate never costs a network round trip, and nothing is written
  unless every check passes. The validation fetch is **not** a poll — it stores no incidents
  and leaves the health columns null, so a new system shows an unknown (grey) light until the
  poller reads it. The `feed/` adapter therefore exists for validation only today; polling is
  still a later slice.
- **Read model / traffic-light.** `GET /overview` backs the main page: one row per feed, no
  parameters, and no incidents (a card lazily fetches its own from `GET /feeds/{feedId}/items`).
  The `Indicator` (green/yellow/red/grey) is derived in `domain` — never in the frontend — from
  the newest item: `investigating|identified` → outage, `monitoring|maintenance` → degraded,
  `resolved` → operational, anything unrecognized → unknown. A feed with no items is
  `operational` if it has ever polled successfully, otherwise `unknown`. A failed *latest* poll
  (`last_error`) does **not** repaint a known-good system; it travels as feed data for the
  expanded card. See `domain.NewSystemOverview`.
- **Errors:** return wrapped errors (`fmt.Errorf("...: %w", err)`); the httpapi adapter
  maps domain errors to HTTP status codes.
- **Generated code:** lives beside its adapter and is named `*.gen.go`. Regenerated, never
  edited by hand (see §6).
- **Testing:** stdlib `testing`, table-driven; `net/http/httptest` for the httpapi adapter;
  fakes/in-memory implementations of ports for application tests. The storage adapter is
  tested against a throwaway SQLite database (a temp-file DB, or `:memory:` with a shared
  cache) with migrations applied first, keeping adapter tests hermetic. `go-cmp` allowed for
  readable diffs. Aim to test domain and application thoroughly; adapters via their ports.

---

## 5. Frontend conventions (`frontend/`)

- **TypeScript only.** No plain `.js` in `src/`. `vue-tsc` type-checking must pass.
- **Package manager: Yarn**, pinned via Corepack and the `packageManager` field in
  `package.json`. Run all scripts through Yarn (`yarn <script>`).
- **Design-system first.** Every reusable UI element lives in its own folder under
  `src/components/<Component>/` with:
  - `<Component>.vue` — the component,
  - `<Component>.stories.ts` — its Storybook story (developed/reviewed in isolation),
  - `<Component>.spec.ts` — its Vitest + Vue Test Utils tests.
  Pages compose these components; they do not invent one-off markup that belongs in the
  design system.
- **Pages hold no logic.** `src/pages/` compose components and pass data down. Data comes
  from the generated API client; no business rules, no direct `fetch`.
- **API access only via the generated layer.** `src/api/` holds:
  - `schema.gen.ts` — `openapi-typescript` output from `api/openapi.yaml`. **Generated; never
    hand-edited** (see §6), and excluded from ESLint and Prettier.
  - `client.ts` — the `openapi-fetch` client instance (`baseUrl: '/api'`), three lines.
  - `status.ts` — one thin function per operation the UI calls (`fetchOverview`,
    `fetchFeedItems`), plus the schema type aliases everything else imports. Transport only:
    unwrap the response, throw on failure, never reshape or decide anything.
    **A missing body is a failure, never an empty result.** `openapi-fetch` populates `error`
    only when the failure body parsed as the contract's `Error` schema; a dead backend behind
    the dev proxy answers `500` with an empty body, so both `data` and `error` come back
    undefined. Check `response.ok` and `data` — defaulting to `[]` there would render "no
    systems" as though the server had said so, and would blank the page on a failed refresh.
- **Data flows top-down; components never fetch.** A page owns the data and passes it into
  components as props; a component that needs more data *emits* (e.g. `SystemCard` emits
  `expand`, the page fetches that feed's items and passes them back). This keeps component
  tests free of network mocking.
- **Component names are multi-word** (`vue/multi-word-component-names`) — hence `PageToolbar`,
  not `Toolbar`.
- **Outcomes are announced, never swallowed.** A failed request must not leave the user
  guessing — an empty area otherwise reads as "nothing saved yet", and an empty card body as
  "no incidents". Announce via `notifyError` / `notifySuccess` from
  `src/composables/statusBanner.ts`, and keep the `console.error` alongside it: the banner
  carries a human sentence, the console keeps the technical detail. The queue shows one banner
  at a time, bottom-right, above everything (`--z-status-banner`).
- **Loading states have a minimum duration.** The backend is local and answers in single-digit
  milliseconds, so an unfloored spinner flashes for a frame and reads as a glitch. Gate them on
  `MIN_LOADING_MS` from `src/utils/timing.ts` (a floor, not a timeout — a slow request still
  owns the affordance), and reveal fresh data only once the floor has passed, so no frame shows
  new content under a live loader.
- **Deferred, by decision:** Storybook stories (`*.stories.ts`) and Playwright visual tests are
  not set up yet; the first slice ships components with Vitest specs only, and UI appearance was
  verified manually in a browser. Add both when the design system grows — the per-component
  folder layout above already has room for them.
- **Vue-native tooling** (from `yarn create vue`): Vite, ESLint + `eslint-plugin-vue`,
  Prettier, Vitest, Playwright. Prefer these over third-party equivalents.

---

## 6. OpenAPI contract workflow

`api/openapi.yaml` is the single contract. Any change to the BE↔FE interface starts there.

1. Edit `api/openapi.yaml`.
2. Regenerate the **backend** server stubs — from `backend/`:
   `go tool oapi-codegen --config internal/adapters/httpapi/oapi-codegen.yaml ../api/openapi.yaml`
   (config: `backend/internal/adapters/httpapi/oapi-codegen.yaml`; output paths in it are
   resolved against the working directory, so run it from `backend/`).
3. Regenerate the **frontend** types — from `frontend/`: `yarn gen:api`
   (`openapi-typescript ../api/openapi.yaml -o src/api/schema.gen.ts`).
4. Implement/adjust code against the regenerated interfaces (TDD).

`oapi-codegen` is pinned as a Go **tool dependency** in `backend/go.mod` (invoked via
`go tool`), so codegen is reproducible without a separately installed binary.

**Only implemented operations are routed.** `oapi-codegen.yaml` lists
`output-options.include-operation-ids`, so `ServerInterface` covers just the operations that
have handlers; every other documented path 404s instead of half-answering. Add an operation id
there when its handler lands.

**Drift is a CI failure.** CI regenerates from the spec and fails if the committed
generated code differs. Commit regenerated code alongside the spec change. (The workflow in
`.github/workflows/` is not written yet — until it is, run steps 2–3 and check `git diff` by
hand before committing.)

---

## 7. Versioning workflow

- The whole tool has **one semver version**, stored in the root **`VERSION`** file
  (currently `0.0.1`).
- It is the single source of truth:
  - **Backend** — injected at build time via `-ldflags "-X main.version=$(cat VERSION)"`.
  - **Frontend** — injected at build time via a Vite `define` / env var.
  - **Docker image** — tagged with it.
- **Bump `VERSION`** as part of any change that alters observable behavior, following
  semver (MAJOR breaking / MINOR feature / PATCH fix). CI verifies the value is a valid
  semver string.

---

## 8. Testing philosophy

- **TDD is the default:** red → green → refactor, for backend domain/application/adapters
  and for frontend component logic (Vitest + Vue Test Utils).
- **Sanctioned exception — UI visual behavior:** appearance and layout are verified with
  **Playwright screenshot / visual-regression tests**, written after the component exists
  rather than strictly test-first. _(Not wired up yet — see §5. The first UI slice was checked
  by hand in a browser at wide and 480px viewports.)_
- Backend targets meaningful coverage of domain and application layers; adapters are tested
  through their ports with fakes and `httptest`.

---

## 9. Commands

Verified against the repo. Keep them accurate — agents rely on this section.

**Repo-wide**
- Current version: `cat VERSION`

**Backend** (`cd backend`)
- Build: `go build -ldflags "-X main.version=$(cat ../VERSION)" -o bin/status-page ./cmd/status-page`
- Run (API on :8080, database at the repo-root `data/`): `go run ./cmd/status-page -db ../data/fire-lookout.db`
  - Flags: `-addr` (default `:8080`, env `RSS_READER_ADDR`), `-db` (default `./data/fire-lookout.db`,
    env `RSS_READER_DB`).
- Test: `go test ./...`
- Vet: `go vet ./...`
- Lint: `golangci-lint run` (config: `backend/.golangci.yml`; requires golangci-lint ≥ v2;
  subsumes `go vet`. Enforces hexagonal import boundaries via `depguard`.) Not installed in the
  dev image yet: `go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest run`.
- Format: `golangci-lint fmt` — runs `gofumpt` **and** `gci` with the repo's settings. Do not
  run bare `gofumpt`: without the `module-path` setting from `.golangci.yml` its
  "dotless first segment means stdlib" heuristic files `fire-lookout/backend/...` imports into
  the standard-library group, producing files the linter then rejects.
- Generate server from OpenAPI: see §6.
- Migrations are applied automatically on startup (embedded, idempotent); the `goose` CLI is
  not required.

**Frontend** (`cd frontend`)
- Install: `corepack yarn install` (Yarn 4 via the `packageManager` field; a bare `yarn` may be
  a different global install)
- Dev server: `corepack yarn dev` — serves the UI on :5173 and proxies `/api` to :8080, so run
  the backend alongside it.
- Test: `corepack yarn test:unit`
- Type-check: `corepack yarn type-check`
- Lint: `corepack yarn lint`
- Format: `corepack yarn format`
- Generate API types from OpenAPI: `corepack yarn gen:api`
- Build static: `corepack yarn build`
- Storybook / Playwright: not set up yet (see §5).

**Infra**
- Not scaffolded yet: `infra/Dockerfile` and `docker-compose.yml` do not exist, and the backend
  does not embed `frontend/dist` (a `go:embed` of a missing directory would not compile). Until
  they land, run the two dev commands above side by side.

---

## 10. What not to touch

- **Generated code** — `backend/internal/adapters/httpapi/*.gen.go` and
  `frontend/src/api/schema.gen.ts`. Change `api/openapi.yaml` and regenerate instead. (The rest
  of `frontend/src/api/` — `client.ts`, `status.ts` — *is* hand-written; see §5.)
- **Build output** — `backend/bin/`, `frontend/dist/`, `frontend/storybook-static/`.
- **Local runtime data** — `./data/` (gitignored SQLite database + `-wal`/`-shm` sidecar files).
