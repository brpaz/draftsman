# syntax=docker/dockerfile:1

# =============================
# --- Global Build Arguments ---
# =============================
ARG GO_VERSION=1.25.4
ARG ALPINE_VERSION=3.22

# ==============================
# --- Dependencies Stage ---
# ==============================
FROM golang:${GO_VERSION}-alpine${ALPINE_VERSION} AS dependencies

WORKDIR /app

COPY go.mod go.sum ./

RUN --mount=type=cache,target=/go/pkg/mod \
  --mount=type=cache,target=/root/.cache/go-build \
  go mod download && \
  go mod verify

# ==============================
# --- Builder Stage ---
# ==============================
FROM golang:${GO_VERSION}-alpine${ALPINE_VERSION} AS builder

ARG TARGETPLATFORM
ARG BUILD_DATE
ARG VCS_REF
ARG VERSION=latest

WORKDIR /app

# Copy source code
COPY . .

# Build the application
# CGO_ENABLED=0 for static linking
# -ldflags options:
#   -w: omit DWARF symbol table
#   -s: omit symbol table and debug information
#   -X: set version variables for reproducibility
# -trimpath: remove all file system paths from the binary
# -a: force rebuilding of packages for reproducibility
RUN --mount=type=cache,target=/go/pkg/mod \
  --mount=type=cache,target=/root/.cache/go-build \
  CGO_ENABLED=0 \
  go build \
  -a \
  -trimpath \
  -ldflags="-w -s -X main.Version=${VERSION} -X main.BuildDate=${BUILD_DATE} -X main.Commit=${VCS_REF}" \
  -o /build/app ./cmd/draftsman

# ==============================
# --- Development Stage ---
# ==============================
FROM golang:${GO_VERSION}-alpine${ALPINE_VERSION} AS development

ARG GID=1000
ARG UID=1000

WORKDIR /app

RUN --mount=type=cache,target=/go/pkg/mod \
  --mount=type=cache,target=/root/.cache/go-build \
  go install github.com/go-delve/delve/cmd/dlv@latest

RUN addgroup -g ${GID} -S appgroup && \
  adduser -u ${UID} -S appuser -G appgroup

COPY . .

USER appuser

CMD ["go", "run", "./cmd/draftsman"]

# ==============================
# --- Production Stage ---
# ==============================
FROM alpine:${ALPINE_VERSION} AS production

ARG BUILD_DATE
ARG VCS_REF
ARG VERSION=latest

ARG GID=1000
ARG UID=1000

RUN addgroup -g ${GID} -S appgroup && \
  adduser -u ${UID} -S appuser -G appgroup

RUN apk --no-cache add ca-certificates curl

COPY --from=builder /build/app /usr/local/bin/draftsman

RUN chown appuser:appgroup /usr/local/bin/draftsman && \
  chmod +x /usr/local/bin/draftsman

USER appuser

LABEL org.opencontainers.image.title="draftsman" \
  org.opencontainers.image.version="${VERSION}" \
  org.opencontainers.image.created="${BUILD_DATE}" \
  org.opencontainers.image.revision="${VCS_REF}" \
  org.opencontainers.image.description="CLI tool that generates release notes from Conventional Commits, maintaining a continuously-updated draft release across GitHub, Gitea, and Forgejo" \
  org.opencontainers.image.source="https://github.com/brpaz/draftsman"

ENTRYPOINT ["/usr/local/bin/draftsman"]
