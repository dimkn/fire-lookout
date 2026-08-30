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
│       └── api/                   # GENERATED types + openapi-fetch client. Do not hand-edit.
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
  (defaults to `./data/fire-lookout.db`, gitignored), opened in **WAL mode**. Accessed through
  stdlib `database/sql` with the `modernc.org/sqlite` driver — **no ORM**. SQLite (WAL) plus
  the `database/sql` pool handle concurrency, so no manual serialization is needed. Schema is
  managed by versioned SQL migrations under `internal/adapters/storage/migrations/` (embedded
  via `embed.FS`) applied on startup with `goose`. All reads/writes go through the repository
  adapter — nothing else touches the database.
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
- **API access only via the generated layer.** `src/api/` holds `openapi-typescript` types
  and an `openapi-fetch` client generated from `api/openapi.yaml`. It is generated output —
  do not hand-edit (see §6).
- **Vue-native tooling** (from `yarn create vue`): Vite, ESLint + `eslint-plugin-vue`,
  Prettier, Vitest, Playwright. Prefer these over third-party equivalents.

---

## 6. OpenAPI contract workflow

`api/openapi.yaml` is the single contract. Any change to the BE↔FE interface starts there.

1. Edit `api/openapi.yaml`.
2. Regenerate the **backend** server stubs (`oapi-codegen` → `backend/internal/adapters/httpapi/*.gen.go`).
3. Regenerate the **frontend** types + client (`openapi-typescript` + `openapi-fetch` → `frontend/src/api/`).
4. Implement/adjust code against the regenerated interfaces (TDD).

**Drift is a CI failure.** CI regenerates from the spec and fails if the committed
generated code differs. Commit regenerated code alongside the spec change.

> Exact generator commands are wired up when each pillar is scaffolded and recorded in §9.

---

## 7. Versioning workflow

- The whole tool has **one semver version**, stored in the root **`VERSION`** file
  (currently `0.0.0`).
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
  rather than strictly test-first.
- Backend targets meaningful coverage of domain and application layers; adapters are tested
  through their ports with fakes and `httptest`.

---

## 9. Commands

> Placeholders until each pillar is scaffolded. Fill in exact invocations as they land,
> and keep them accurate — agents rely on this section.

**Repo-wide**
- Current version: `cat VERSION`

**Backend** (`cd backend`)
- Build: `go build -ldflags "-X main.version=$(cat ../VERSION)" -o bin/status-page ./cmd/status-page` _(TBD)_
- Test: `go test ./...` _(TBD)_
- Vet/format: `go vet ./...` / `gofmt -l .` _(TBD)_
- Generate server from OpenAPI: `oapi-codegen ...` _(TBD)_
- Apply migrations: `goose -dir internal/adapters/storage/migrations sqlite ./data/fire-lookout.db up` _(TBD)_

**Frontend** (`cd frontend`)
- Install: `yarn install` _(TBD)_
- Dev server: `yarn dev` _(TBD)_
- Test: `yarn test:unit` _(TBD)_
- Type-check: `yarn type-check` _(TBD)_
- Lint: `yarn lint` _(TBD)_
- Storybook: `yarn storybook` _(TBD)_
- Visual tests: `yarn test:e2e` (Playwright) _(TBD)_
- Generate API client from OpenAPI: `yarn gen:api` _(TBD)_
- Build static: `yarn build` _(TBD)_

**Infra**
- Build image: `docker build -f infra/Dockerfile -t fire-lookout:$(cat VERSION) .` _(TBD)_
- Run locally: `docker compose up --build` _(TBD)_

---

## 10. What not to touch

- **Generated code** — `backend/internal/adapters/httpapi/*.gen.go` and everything in
  `frontend/src/api/`. Change `api/openapi.yaml` and regenerate instead.
- **Build output** — `backend/bin/`, `frontend/dist/`, `frontend/storybook-static/`.
- **Local runtime data** — `./data/` (gitignored SQLite database + `-wal`/`-shm` sidecar files).
