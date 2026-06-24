FROM golang:1 AS build
ENV CGO_ENABLED=0
ADD . /go/src/app
WORKDIR /go/src/app
RUN make picoflux

FROM rockylinux:9
RUN dnf install --setopt=install_weak_deps=False -y rpm-build systemd-rpm-macros
RUN mkdir -p /root/rpmbuild/{BUILD,RPMS,SOURCES,SPECS,SRPMS}
RUN echo "%_topdir /root/rpmbuild" >> .rpmmacros
COPY --from=build /go/src/app/picoflux /root/rpmbuild/SOURCES/picoflux
COPY --from=build /go/src/app/LICENSE /root/rpmbuild/SOURCES/
COPY --from=build /go/src/app/picoflux.1 /root/rpmbuild/SOURCES/
COPY --from=build /go/src/app/packaging/systemd/picoflux.service /root/rpmbuild/SOURCES/
COPY --from=build /go/src/app/packaging/picoflux.conf /root/rpmbuild/SOURCES/
COPY --from=build /go/src/app/packaging/rpm/picoflux.spec /root/rpmbuild/SPECS/picoflux.spec
