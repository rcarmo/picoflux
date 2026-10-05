# Disposable output only; retained .profiles evidence is never cleaned here.
# Resolve once with the original TMPDIR still available. Explicit unsafe
# overrides fail instead of falling back. The resolver is vendored for portability.
ifneq ($(origin PROJECT_TMP_ROOT),undefined)
TMP_OVERRIDE := PROJECT_TMP_ROOT='$(PROJECT_TMP_ROOT)'
endif
RESOLVED_TMP_ROOT := $(shell $(TMP_OVERRIDE) PROJECT=picoflux bash scripts/project-tmp.sh init | sed -n 's/^PROJECT_TMP_ROOT=//p')
ifeq ($(strip $(RESOLVED_TMP_ROOT)),)
$(error Unable to resolve a safe picoflux temporary root)
endif
override PROJECT_TMP_ROOT := $(RESOLVED_TMP_ROOT)
export PROJECT_TMP_ROOT
export TMPDIR := $(PROJECT_TMP_ROOT)/runs/tools
export TMP := $(TMPDIR)
export TEMP := $(TMPDIR)
export GOCACHE := $(PROJECT_TMP_ROOT)/cache/go-build
export GOMODCACHE := $(PROJECT_TMP_ROOT)/cache/go-mod
export GOBIN := $(PROJECT_TMP_ROOT)/build/bin
export BUN_INSTALL_CACHE_DIR := $(PROJECT_TMP_ROOT)/cache/bun
export npm_config_cache := $(PROJECT_TMP_ROOT)/cache/npm
export PYTHONPYCACHEPREFIX := $(PROJECT_TMP_ROOT)/cache/python
export UV_CACHE_DIR := $(PROJECT_TMP_ROOT)/cache/uv
export PLAYWRIGHT_BROWSERS_PATH := $(PROJECT_TMP_ROOT)/cache/playwright
$(shell mkdir -p $(TMPDIR) $(GOCACHE) $(GOMODCACHE) $(GOBIN))
INTEGRATION_RUN := $(PROJECT_TMP_ROOT)/runs/integration
$(shell mkdir -p $(INTEGRATION_RUN))

APP             := picoflux
DOCKER_IMAGE    := ghcr.io/rcarmo/picoflux
VERSION         := $(shell git describe --tags --exact-match 2>/dev/null)
LD_FLAGS        := "-s -w -X 'miniflux.app/v2/internal/version.Version=$(VERSION)'"
PKG_LIST        := $(shell go list ./... | grep -v /vendor/)
DB_URL          := $(INTEGRATION_RUN)/picoflux_test.db
DOCKER_PLATFORM := amd64


.PHONY: \
	picoflux \
	picoflux-no-pie \
	linux-amd64 \
	linux-arm64 \
	linux-armv7 \
	linux-armv6 \
	linux-armv5 \
	linux-riscv64 \
	darwin-amd64 \
	darwin-arm64 \
	freebsd-amd64 \
	openbsd-amd64 \
	build \
	run \
	clean \
	add-string \
	test \
	lint \
	integration-test \
	clean-integration-test \
	docker-image \
	docker-image-distroless \
	docker-images \
	rpm \
	debian \
	debian-packages

picoflux:
	@ go build -buildmode=pie -ldflags=$(LD_FLAGS) -o $(APP)

picoflux-no-pie:
	@ go build -ldflags=$(LD_FLAGS) -o $(APP)

linux-amd64:
	@ CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags=$(LD_FLAGS) -o $(APP)-$@
	@ sha256sum $(APP)-$@ > $(APP)-$@.sha256

linux-arm64:
	@ CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags=$(LD_FLAGS) -o $(APP)-$@
	@ sha256sum $(APP)-$@ > $(APP)-$@.sha256

linux-armv7:
	@ CGO_ENABLED=0 GOOS=linux GOARCH=arm GOARM=7 go build -ldflags=$(LD_FLAGS) -o $(APP)-$@
	@ sha256sum $(APP)-$@ > $(APP)-$@.sha256

linux-armv6:
	@ CGO_ENABLED=0 GOOS=linux GOARCH=arm GOARM=6 go build -ldflags=$(LD_FLAGS) -o $(APP)-$@
	@ sha256sum $(APP)-$@ > $(APP)-$@.sha256

linux-armv5:
	@ CGO_ENABLED=0 GOOS=linux GOARCH=arm GOARM=5 go build -ldflags=$(LD_FLAGS) -o $(APP)-$@
	@ sha256sum $(APP)-$@ > $(APP)-$@.sha256

linux-riscv64:
	@ CGO_ENABLED=0 GOOS=linux GOARCH=riscv64 go build -ldflags=$(LD_FLAGS) -o $(APP)-$@
	@ sha256sum $(APP)-$@ > $(APP)-$@.sha256

darwin-amd64:
	@ GOOS=darwin GOARCH=amd64 go build -ldflags=$(LD_FLAGS) -o $(APP)-$@
	@ sha256sum $(APP)-$@ > $(APP)-$@.sha256

darwin-arm64:
	@ GOOS=darwin GOARCH=arm64 go build -ldflags=$(LD_FLAGS) -o $(APP)-$@
	@ sha256sum $(APP)-$@ > $(APP)-$@.sha256

freebsd-amd64:
	@ CGO_ENABLED=0 GOOS=freebsd GOARCH=amd64 go build -ldflags=$(LD_FLAGS) -o $(APP)-$@
	@ sha256sum $(APP)-$@ > $(APP)-$@.sha256

openbsd-amd64:
	@ GOOS=openbsd GOARCH=amd64 go build -ldflags=$(LD_FLAGS) -o $(APP)-$@
	@ sha256sum $(APP)-$@ > $(APP)-$@.sha256

build: linux-amd64 linux-arm64 linux-armv7 linux-armv6 linux-armv5 linux-riscv64 darwin-amd64 darwin-arm64 freebsd-amd64 openbsd-amd64

run:
	@ LOG_DATE_TIME=1 LOG_LEVEL=debug RUN_MIGRATIONS=1 CREATE_ADMIN=1 ADMIN_USERNAME=admin ADMIN_PASSWORD=test123 go run main.go

clean:
	@ rm -f $(APP)-* $(APP) $(APP)*.rpm $(APP)*.deb $(APP)*.exe $(APP)*.sha256

add-string:
	cd internal/locale/translations && \
	for file in *.json; do \
		jq --indent 4 --arg key "$(KEY)" --arg val "$(VAL)" \
		   '. + {($$key): $$val} | to_entries | sort_by(.key) | from_entries' "$$file" > tmp && \
		mv tmp "$$file"; \
	done

test:
	scripts/test-profile.sh -cover -race

lint:
	go vet ./...
	test -z "$$(gofmt -l .)"
	golangci-lint run

integration-test:
	rm -f $(INTEGRATION_RUN)/picoflux_test.db $(INTEGRATION_RUN)/picoflux_test.db-wal $(INTEGRATION_RUN)/picoflux_test.db-shm

	DATABASE_URL=$(DB_URL) \
	ADMIN_USERNAME=admin \
	ADMIN_PASSWORD=test123 \
	CREATE_ADMIN=1 \
	RUN_MIGRATIONS=1 \
	LOG_LEVEL=debug \
	FETCHER_ALLOW_PRIVATE_NETWORKS=1 \
	INTEGRATION_ALLOW_PRIVATE_NETWORKS=1 \
	go run main.go >$(INTEGRATION_RUN)/picoflux.log 2>&1 & echo "$$!" > "$(INTEGRATION_RUN)/picoflux.pid"

	while ! nc -z localhost 8080; do sleep 1; done

	TEST_MINIFLUX_BASE_URL=http://127.0.0.1:8080 \
	TEST_MINIFLUX_ADMIN_USERNAME=admin \
	TEST_MINIFLUX_ADMIN_PASSWORD=test123 \
	TEST_PACKAGES=./internal/api scripts/test-profile.sh -v

clean-integration-test:
	@ kill -9 `cat $(INTEGRATION_RUN)/picoflux.pid`
	@ rm -f $(INTEGRATION_RUN)/picoflux.pid $(INTEGRATION_RUN)/picoflux.log
	@ rm -f $(INTEGRATION_RUN)/picoflux_test.db $(INTEGRATION_RUN)/picoflux_test.db-wal $(INTEGRATION_RUN)/picoflux_test.db-shm

docker-image:
	docker build --pull -t $(DOCKER_IMAGE):$(VERSION) -f packaging/docker/alpine/Dockerfile .

docker-image-distroless:
	docker build -t $(DOCKER_IMAGE):$(VERSION) -f packaging/docker/distroless/Dockerfile .

docker-images:
	docker buildx build \
		--platform linux/amd64,linux/arm64,linux/arm/v7,linux/arm/v6,linux/riscv64 \
		--file packaging/docker/alpine/Dockerfile \
		--tag $(DOCKER_IMAGE):$(VERSION) \
		--push .

rpm: clean
	@ docker build \
		-t picoflux-rpm-builder \
		-f packaging/rpm/Dockerfile \
		.
	@ docker run --rm \
		-v ${PWD}:/root/rpmbuild/RPMS/x86_64 picoflux-rpm-builder \
		rpmbuild -bb --define "_miniflux_version $(VERSION)" /root/rpmbuild/SPECS/picoflux.spec

debian:
	@ docker buildx build --load \
		--platform linux/$(DOCKER_PLATFORM) \
		-t picoflux-deb-builder \
		-f packaging/debian/Dockerfile \
		.
	@ docker run --rm --platform linux/$(DOCKER_PLATFORM) \
		-v ${PWD}:/pkg picoflux-deb-builder

debian-packages: clean
	$(MAKE) debian DOCKER_PLATFORM=amd64
	$(MAKE) debian DOCKER_PLATFORM=arm64
	$(MAKE) debian DOCKER_PLATFORM=arm/v7
	$(MAKE) debian DOCKER_PLATFORM=riscv64
