# Build client side assets (JS bundling + minification, CSS concat).
# Phase 4a: replaced gulp + webpack + babel with a single esbuild driver
# (build.mjs). 745 transitive npm deps (47 vulns) collapsed to 4 (0 vulns).
FROM node:22-alpine AS build-js

WORKDIR /build
COPY package.json package-lock.json build.mjs ./
RUN npm ci
COPY static/ ./static/
RUN npm run build


# Build Golang binary
FROM golang:1.25-bookworm AS build-golang

# Pure-Go build (sqlite via modernc.org/sqlite, no libsqlite3 native dep).
# This makes the resulting binary statically linked and removes the gcc
# requirement from this stage.
ENV CGO_ENABLED=0

WORKDIR /go/src/github.com/rdumanski/gophish
COPY . .
RUN go build -v ./...
RUN go build -v -o gophish .


# Runtime container
FROM debian:stable-slim

RUN useradd -m -d /opt/gophish -s /bin/bash app

RUN apt-get update && \
	apt-get install --no-install-recommends -y jq libcap2-bin ca-certificates && \
	apt-get clean && \
	rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/*

WORKDIR /opt/gophish
COPY --from=build-golang /go/src/github.com/rdumanski/gophish/ ./
COPY --from=build-js /build/static/js/dist/ ./static/js/dist/
COPY --from=build-js /build/static/css/dist/ ./static/css/dist/
COPY --from=build-golang /go/src/github.com/rdumanski/gophish/config.json ./
RUN chown app. config.json

RUN setcap 'cap_net_bind_service=+ep' /opt/gophish/gophish

USER app
RUN sed -i 's/127.0.0.1/0.0.0.0/g' config.json
RUN touch config.json.tmp

EXPOSE 3333 8080 8443 80

CMD ["./docker/run.sh"]
