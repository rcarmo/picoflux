FROM docker.io/library/golang:alpine3.23 AS build
RUN apk add --no-cache build-base git make
ADD . /go/src/app
WORKDIR /go/src/app
RUN make picoflux

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
RUN apk --no-cache add ca-certificates tzdata
COPY --from=build /go/src/app/picoflux /usr/bin/picoflux
USER 65534
CMD ["/usr/bin/picoflux"]
