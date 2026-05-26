# syntax=docker/dockerfile:1.7
# Build from monorepo root:
#   docker build -f services/commercial/crm/Dockerfile .
FROM golang:1.25-alpine AS build

WORKDIR /src
RUN apk add --no-cache git ca-certificates

COPY shared/platform-go /src/shared/platform-go

WORKDIR /src/services/commercial/crm
COPY services/commercial/crm/go.mod services/commercial/crm/go.sum ./
COPY services/commercial/crm/pkg/authclient/go.mod ./pkg/authclient/
RUN go mod download

COPY services/commercial/crm/ .
ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-s -w -X main.version=${VERSION}" \
    -o /crm .

FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata wget
WORKDIR /app
COPY --from=build /crm /app/crm

ENV PORT=4101 \
    GIN_MODE=release \
    LOG_FORMAT=json \
    AUTH_MODE=jwt \
    AUTO_MIGRATE=true \
    SEED_ON_EMPTY=false

EXPOSE 4101
HEALTHCHECK --interval=15s --timeout=5s --start-period=25s --retries=5 \
  CMD wget -q -O /dev/null http://127.0.0.1:4101/v1/health/ready || exit 1
USER nobody
ENTRYPOINT ["/app/crm"]
