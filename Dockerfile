# syntax=docker/dockerfile:1.7
#
# Targets:
#   standalone (default) — iag-crm repo root on Railway
#   monorepo             — IAG_multi_backend root context (deploy/docker-compose)
#
# Monorepo:   docker build -f services/commercial/crm/Dockerfile --target monorepo .
# Standalone: docker build --target standalone .

FROM golang:1.25-alpine AS base
RUN apk add --no-cache git ca-certificates
ENV PLATFORM_GO_DEP=/deps/platform-go

FROM base AS platform-go-clone
ARG IAG_META_REF=main
ARG IAG_META_REPO=https://github.com/AlexanderKiyingi/IAG_multi_backend.git
RUN git clone --depth 1 --branch "${IAG_META_REF}" "${IAG_META_REPO}" /tmp/iag \
    && mv /tmp/iag/shared/platform-go "${PLATFORM_GO_DEP}" \
    && rm -rf /tmp/iag

FROM base AS platform-go-copy
COPY shared/platform-go ${PLATFORM_GO_DEP}

FROM base AS build-standalone
COPY --from=platform-go-clone ${PLATFORM_GO_DEP} ${PLATFORM_GO_DEP}
WORKDIR /src
COPY go.mod go.sum ./
COPY pkg/authclient/go.mod ./pkg/authclient/
RUN go mod edit -replace=github.com/alvor-technologies/iag-platform-go=${PLATFORM_GO_DEP} \
    && go mod download
COPY . .
ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-s -w -X main.version=${VERSION}" \
    -o /crm .

FROM base AS build-monorepo
COPY --from=platform-go-copy ${PLATFORM_GO_DEP} ${PLATFORM_GO_DEP}
WORKDIR /src/services/commercial/crm
COPY services/commercial/crm/go.mod services/commercial/crm/go.sum ./
COPY services/commercial/crm/pkg/authclient/go.mod ./pkg/authclient/
RUN go mod edit -replace=github.com/alvor-technologies/iag-platform-go=${PLATFORM_GO_DEP} \
    && go mod download
COPY services/commercial/crm/ .
ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-s -w -X main.version=${VERSION}" \
    -o /crm .

FROM alpine:3.21 AS monorepo
RUN apk add --no-cache ca-certificates tzdata wget
WORKDIR /app
COPY --from=build-monorepo /crm /app/crm
ENV PORT=4101 \
    GIN_MODE=release \
    LOG_FORMAT=json \
    AUTO_MIGRATE=true \
    SEED_ON_EMPTY=false
EXPOSE 4101
HEALTHCHECK --interval=15s --timeout=5s --start-period=25s --retries=5 \
  CMD wget -q -O /dev/null http://127.0.0.1:4101/v1/health/ready || exit 1
USER nobody
ENTRYPOINT ["/app/crm"]

FROM alpine:3.21 AS standalone
RUN apk add --no-cache ca-certificates tzdata wget
WORKDIR /app
COPY --from=build-standalone /crm /app/crm
ENV PORT=4101 \
    GIN_MODE=release \
    LOG_FORMAT=json \
    AUTO_MIGRATE=true \
    SEED_ON_EMPTY=false
EXPOSE 4101
HEALTHCHECK --interval=15s --timeout=5s --start-period=25s --retries=5 \
  CMD wget -q -O /dev/null http://127.0.0.1:4101/v1/health/ready || exit 1
USER nobody
ENTRYPOINT ["/app/crm"]
