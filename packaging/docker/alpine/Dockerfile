# syntax=docker/dockerfile:1.7

ARG BUILDPLATFORM=linux/amd64

# Build on the native BuildKit host architecture and cross-compile the pure-Go
# picoflux binary for the requested target. No target binary is executed here,
# so multi-arch builds do not need QEMU.
FROM --platform=$BUILDPLATFORM docker.io/library/golang:alpine3.23 AS build

ARG TARGETOS
ARG TARGETARCH
ARG TARGETVARIANT
ARG VERSION=dev

RUN apk add --no-cache git make

ADD . /go/src/app
WORKDIR /go/src/app

RUN set -eux; \
    case "${TARGETARCH}/${TARGETVARIANT}" in \
        arm/v6) export GOARM=6 ;; \
        arm/v7) export GOARM=7 ;; \
        *) unset GOARM ;; \
    esac; \
    CGO_ENABLED=0 GOOS="${TARGETOS}" GOARCH="${TARGETARCH}" \
        go build \
        -ldflags="-s -w -X miniflux.app/v2/internal/version.Version=${VERSION}" \
        -o /out/picoflux .

# ca-certificates and tzdata contain architecture-independent data. Prepare
# them once on BUILDPLATFORM, then copy them into the target stage without
# running apk under emulation.
FROM --platform=$BUILDPLATFORM docker.io/library/alpine:3.23 AS runtime-data
RUN apk add --no-cache ca-certificates tzdata

FROM docker.io/library/alpine:3.23

LABEL org.opencontainers.image.title=picoflux
LABEL org.opencontainers.image.description="picoflux is a SQLite-backed fork of Miniflux, a minimalist feed reader"
LABEL org.opencontainers.image.vendor="Frédéric Guillot"
LABEL org.opencontainers.image.licenses=Apache-2.0
LABEL org.opencontainers.image.url=https://github.com/rcarmo/picoflux
LABEL org.opencontainers.image.source=https://github.com/rcarmo/picoflux
LABEL org.opencontainers.image.documentation=https://miniflux.app/docs/

EXPOSE 8080
ENV LISTEN_ADDR=0.0.0.0:8080

COPY --from=runtime-data /etc/ssl/certs/ /etc/ssl/certs/
COPY --from=runtime-data /usr/share/zoneinfo/ /usr/share/zoneinfo/
COPY --from=build /out/picoflux /usr/bin/picoflux

USER 65534
CMD ["/usr/bin/picoflux"]
