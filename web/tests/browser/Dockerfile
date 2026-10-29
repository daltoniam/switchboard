# syntax=docker/dockerfile:1
ARG NODE_VERSION
FROM golang:1.27.2-bookworm@sha256:5cf287a799e6b94384bad13d16b14904c531f51ba65792237e122ce42b392f61 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY . .
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 go build -buildvcs=false -o /switchboard ./cmd/server

FROM node:${NODE_VERSION}-bookworm-slim
ARG NPM_VERSION
ARG PLAYWRIGHT_VERSION
WORKDIR /tests
RUN npm install --global --ignore-scripts npm@${NPM_VERSION} \
    && npm install --no-save --package-lock=false --ignore-scripts @playwright/test@${PLAYWRIGHT_VERSION} htmx.org@2.0.11 @tarekraafat/autocomplete.js@10.2.10 \
    && npx --no-install playwright install --with-deps chromium
COPY --from=build /switchboard /usr/local/bin/switchboard
COPY web/tests/browser/ui.test.ts ./ui.test.ts
CMD ["node", "--test", "--test-timeout=60000", "ui.test.ts"]
