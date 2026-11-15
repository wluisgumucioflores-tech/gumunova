# syntax=docker/dockerfile:1
# Builds the web app and serves it with nginx, which also proxies /api to the Go API.
# Build context: the repository root (see docker-compose.yml).

FROM node:24-slim AS build
RUN corepack enable && corepack prepare pnpm@10.33.3 --activate
WORKDIR /web
COPY web/package.json web/pnpm-lock.yaml ./
RUN --mount=type=cache,target=/root/.local/share/pnpm/store pnpm install --frozen-lockfile
COPY web/ ./
# Public address printed in the QR labels. Empty means "the address the app is opened from".
# Everything that reaches the bundle is public: never pass secrets here.
ARG VITE_PUBLIC_BASE_URL=
ENV VITE_PUBLIC_BASE_URL=$VITE_PUBLIC_BASE_URL
RUN pnpm build

FROM nginxinc/nginx-unprivileged:1.28-alpine
COPY docker/nginx/default.conf /etc/nginx/conf.d/default.conf
COPY --from=build /web/dist /usr/share/nginx/html
EXPOSE 8080
