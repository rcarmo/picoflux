# syntax=docker/dockerfile:1.7

ARG BUILDPLATFORM=linux/amd64

# Compile on BUILDPLATFORM and target GOOS/GOARCH directly. picoflux uses the
# pure-Go modernc SQLite driver, so CGO and target emulation are unnecessary.
FROM --platform=$BUILDPLATFORM docker.io/library/golang:trixie AS build

ARG TARGETOS
ARG TARGETARCH
ARG TARGETVARIANT
ARG VERSION=dev

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

FROM gcr.io/distroless/base-debian13:nonroot

LABEL org.opencontainers.image.title=picoflux
LABEL org.opencontainers.image.description="picoflux is a SQLite-backed fork of Miniflux, a minimalist feed reader"
LABEL org.opencontainers.image.vendor="Frédéric Guillot"
LABEL org.opencontainers.image.licenses=Apache-2.0
LABEL org.opencontainers.image.url=https://github.com/rcarmo/picoflux
LABEL org.opencontainers.image.source=https://github.com/rcarmo/picoflux
LABEL org.opencontainers.image.documentation=https://miniflux.app/docs/

EXPOSE 8080
ENV LISTEN_ADDR=0.0.0.0:8080

COPY --from=build /out/picoflux /usr/bin/picoflux
CMD ["/usr/bin/picoflux"]
