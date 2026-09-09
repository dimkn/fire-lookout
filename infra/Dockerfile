# syntax=docker/dockerfile:1

# The whole tool as one container: a static Go binary with the Vue SPA compiled into it,
# serving the API under /api and the UI under everything else (AGENTS.md §1).
#
# Build from the REPO ROOT, not from infra/ — frontend/vite.config.ts reads ../VERSION:
#
#	docker build -t rss-reader:$(cat VERSION) -f infra/Dockerfile .

# ---------------------------------------------------------------------------------------
# Stage 1 — build the Vue SPA.
# Node 22 LTS: still bundles Corepack (25 dropped it) and satisfies vite@7's engine range.
# ---------------------------------------------------------------------------------------
FROM node:22-alpine AS frontend

ENV COREPACK_ENABLE_DOWNLOAD_PROMPT=0
WORKDIR /src

RUN corepack enable

# VERSION must sit one level above frontend/, where vite.config.ts looks for it.
COPY VERSION ./VERSION

# Manifests first: dependencies only reinstall when these change, not on every source edit.
COPY frontend/package.json frontend/yarn.lock frontend/.yarnrc.yml ./frontend/
WORKDIR /src/frontend
RUN corepack prepare yarn@4.9.2 --activate
RUN --mount=type=cache,target=/root/.yarn/berry/cache \
    yarn install --immutable

COPY frontend/ ./
RUN yarn build

# ---------------------------------------------------------------------------------------
# Stage 2 — build the Go binary with the SPA embedded.
# CGO_ENABLED=0 gives a fully static binary because the SQLite driver is pure Go
# (modernc.org/sqlite, AGENTS.md §3) — that is what makes a distroless/static runtime work.
# GOTOOLCHAIN=local fails loudly on a version mismatch instead of downloading a toolchain.
# ---------------------------------------------------------------------------------------
FROM golang:1.26-alpine AS backend

ENV CGO_ENABLED=0 GOTOOLCHAIN=local
WORKDIR /src

COPY backend/go.mod backend/go.sum ./backend/
WORKDIR /src/backend
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

WORKDIR /src
COPY VERSION ./VERSION
COPY backend/ ./backend/

# go:embed cannot reach outside the module, so the built assets are dropped into the webui
# adapter's dist/ directory here rather than referenced across the pillar boundary.
COPY --from=frontend /src/frontend/dist/ ./backend/internal/adapters/webui/dist/

WORKDIR /src/backend
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go build -trimpath \
      -ldflags "-s -w -X main.version=$(cat /src/VERSION)" \
      -o /out/status-page ./cmd/status-page

# Docker seeds a fresh named volume from the image's content *and ownership* at the mount
# point. Creating /data here, owned by the runtime uid, is what stops the unprivileged
# process from hitting a root-owned volume it cannot write the SQLite WAL into.
RUN mkdir -p /out/data

# ---------------------------------------------------------------------------------------
# Stage 3 — runtime. distroless/static supplies the CA certificates the feed poller needs
# for outbound HTTPS, plus tzdata, /etc/nsswitch.conf and an unprivileged uid, in ~2 MB.
# ---------------------------------------------------------------------------------------
FROM gcr.io/distroless/static-debian12:nonroot

# Numeric rather than nonroot:nonroot so this does not depend on name lookup in the target
# image; 65532 is distroless's nonroot user.
COPY --from=backend --chown=65532:65532 /out/data /data
COPY --from=backend /out/status-page /status-page

ENV RSS_READER_ADDR=:8080 \
    RSS_READER_DB=/data/rss-reader.db

EXPOSE 8080
USER 65532:65532

# No VOLUME instruction: it would silently create throwaway anonymous volumes on a bare
# `docker run`. Mount /data explicitly — docker-compose.yml does.
ENTRYPOINT ["/status-page"]
