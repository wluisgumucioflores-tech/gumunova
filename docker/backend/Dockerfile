# syntax=docker/dockerfile:1
# Build context: the repository root (see docker-compose.yml).
# One image with both binaries: the API and the migration runner, started in that order.

FROM golang:1.26.4-alpine AS build
WORKDIR /src
COPY backend/go.mod backend/go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY backend/ .
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api \
    && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/migrate ./cmd/migrate

FROM alpine:3.22
# ca-certificates: HTTPS to Cloudflare R2. tzdata: business time zone.
RUN apk add --no-cache ca-certificates tzdata \
    && adduser -D -u 10001 app
COPY --from=build /out/ /app/
COPY docker/backend/entrypoint.sh /app/entrypoint.sh
USER app
EXPOSE 8080
ENTRYPOINT ["/app/entrypoint.sh"]
