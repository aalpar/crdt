GO        ?= go
GOFLAGS   ?=
PACKAGES   = ./...
PROF_PKG  ?= ./dotcontext/
PROF_DIR  ?= profiles
VERSION   := $(shell cat VERSION 2>/dev/null || echo v0.0.0)

.PHONY: all
all: build test vet

.PHONY: build
build:
	$(GO) build $(GOFLAGS) $(PACKAGES)

.PHONY: test
test:
	$(GO) test $(GOFLAGS) $(PACKAGES)

.PHONY: test-v
test-v:
	$(GO) test $(GOFLAGS) -v $(PACKAGES)

.PHONY: test-race
test-race:
	$(GO) test $(GOFLAGS) -race $(PACKAGES)

.PHONY: benchmark
benchmark:
	$(GO) test $(GOFLAGS) -bench=. -benchmem $(PACKAGES)

.PHONY: profile
profile:
	@mkdir -p $(PROF_DIR)
	$(GO) test $(GOFLAGS) -bench=. -benchmem \
		-cpuprofile=$(PROF_DIR)/cpu.prof \
		-memprofile=$(PROF_DIR)/mem.prof \
		$(PROF_PKG)
	@echo ""
	@echo "Profiles written to $(PROF_DIR)/"
	@echo "  go tool pprof -http=:8080 $(PROF_DIR)/cpu.prof"
	@echo "  go tool pprof -http=:8080 $(PROF_DIR)/mem.prof"

.PHONY: fuzz
fuzz:
	$(GO) test $(GOFLAGS) -fuzz=. -fuzztime=30s $(PACKAGES)

.PHONY: lint
lint:
	$(GO) vet $(GOFLAGS) $(PACKAGES)
	@command -v staticcheck >/dev/null 2>&1 && staticcheck $(PACKAGES) || echo "staticcheck not installed"

.PHONY: vet
vet:
	$(GO) vet $(GOFLAGS) $(PACKAGES)

.PHONY: fmt
fmt:
	gofmt -s -w .

.PHONY: fmt-check
fmt-check:
	@test -z "$$(gofmt -s -l .)" || (echo "unformatted files:" && gofmt -s -l . && exit 1)

.PHONY: clean
clean:
	$(GO) clean $(PACKAGES)
	$(GO) clean -cache -testcache -modcache -fuzzcache

.PHONY: tag
tag:
	git tag -a $(VERSION) -m "Release $(VERSION)"
	@echo "Tagged $(VERSION). Push with: git push origin $(VERSION)"

.PHONY: bump-major
bump-major:
	@echo $(VERSION) | sed -E 's/v([0-9]+)\..*/v'$$(echo $(VERSION) | sed -E 's/v([0-9]+)\..*/\1/' | awk '{print $$1+1}')'.0.0/' > VERSION
	@echo "$(VERSION) → $$(cat VERSION)"

.PHONY: bump-minor
bump-minor:
	@echo $(VERSION) | sed -E 's/v([0-9]+)\.([0-9]+)\..*/v\1.'$$(echo $(VERSION) | sed -E 's/v[0-9]+\.([0-9]+)\..*/\1/' | awk '{print $$1+1}')'.0/' > VERSION
	@echo "$(VERSION) → $$(cat VERSION)"

.PHONY: bump-patch
bump-patch:
	@echo $(VERSION) | sed -E 's/v([0-9]+)\.([0-9]+)\.([0-9]+).*/v\1.\2.'$$(echo $(VERSION) | sed -E 's/v[0-9]+\.[0-9]+\.([0-9]+).*/\1/' | awk '{print $$1+1}')'/' > VERSION
	@echo "$(VERSION) → $$(cat VERSION)"
