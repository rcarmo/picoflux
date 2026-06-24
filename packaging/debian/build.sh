#!/bin/sh

PKG_ARCH=$(dpkg --print-architecture)
PKG_DATE=$(date -R)
PKG_VERSION=$(cd /src && git describe --tags --abbrev=0 | sed 's/^v//')

echo "PKG_VERSION=$PKG_VERSION"
echo "PKG_ARCH=$PKG_ARCH"
echo "PKG_DATE=$PKG_DATE"

cd /src

if [ "$PKG_ARCH" = "armhf" ] || [ "$PKG_ARCH" = "riscv64" ]; then
    make picoflux-no-pie
else
    CGO_ENABLED=0 make picoflux
fi

mkdir -p /build/debian && \
cd /build && \
cp /src/picoflux /build/ && \
cp /src/picoflux.1 /build/ && \
cp /src/LICENSE /build/ && \
cp /src/packaging/picoflux.conf /build/ && \
cp /src/packaging/systemd/picoflux.service /build/debian/ && \
cp /src/packaging/debian/compat /build/debian/compat && \
cp /src/packaging/debian/copyright /build/debian/copyright && \
cp /src/packaging/debian/picoflux.manpages /build/debian/picoflux.manpages && \
cp /src/packaging/debian/picoflux.postinst /build/debian/picoflux.postinst && \
cp /src/packaging/debian/rules /build/debian/rules && \
cp /src/packaging/debian/picoflux.dirs /build/debian/picoflux.dirs && \
echo "picoflux ($PKG_VERSION) experimental; urgency=low" > /build/debian/changelog && \
echo "  * Miniflux version $PKG_VERSION" >> /build/debian/changelog && \
echo " -- Frédéric Guillot <f@miniflux.net>  $PKG_DATE" >> /build/debian/changelog && \
sed "s/__PKG_ARCH__/${PKG_ARCH}/g" /src/packaging/debian/control > /build/debian/control && \
dpkg-buildpackage -us -uc -b && \
lintian --check --color always ../*.deb && \
cp ../*.deb /pkg/
