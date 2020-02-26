SHELL    = /bin/bash
AUTHOR   = kolonialno
PACKAGE  = pr-deployment-controller
REGISTRY = gcr.io/kolonial-no-test-environment

DATE    ?= $(shell date +%FT%T%z)
VERSION ?= $(shell git describe --tags --always --dirty --match=v* 2> /dev/null || \
			cat $(CURDIR)/.version 2> /dev/null || echo v0)
BIN      = $(GOPATH)/bin
IMPORT   = github.com/$(AUTHOR)/$(PACKAGE)
BASE     = $(GOPATH)/src/$(IMPORT)
PKGS     = $(or $(PKG),$(shell cd $(BASE) && env GOPATH=$(GOPATH) $(GO) list ./... | grep -Ev "vendor"))
TESTPKGS = $(shell env GOPATH=$(GOPATH) $(GO) list -f '{{ if or .TestGoFiles .XTestGoFiles }}{{ .ImportPath }}{{ end }}' $(PKGS))
NODE_MODULES = $(BASE)/node_modules

GO      = go
GODOC   = godoc
GOFMT   = gofmt
TIMEOUT = 15
NPM = npm
V = 0
Q = $(if $(filter 1,$V),,@)
M = $(shell printf "\033[34;1m▶\033[0m")

.PHONY: all
all: test build | $(BASE) ;
	$Q

$(BASE): ; $(info $(M) checking GOPATH…)
	@echo

# Tools
.PHONY: gopath
gopath:
	@echo $(GOPATH)

DEP = $(BIN)/dep
$(BIN)/dep: | $(BASE) ; $(info $(M) building dep…)
	$Q go get github.com/golang/dep/cmd/dep

GOLANGCI-LINT = $(BIN)/golangci-lint
$(BIN)/golangci-lint: | $(BASE) ; $(info $(M) building golangci-lint…)
	$Q go get github.com/golangci/golangci-lint/cmd/golangci-lint

GOCOVMERGE = $(BIN)/gocovmerge
$(BIN)/gocovmerge: | $(BASE) ; $(info $(M) building gocovmerge…)
	$Q go get github.com/wadey/gocovmerge

GOCOV = $(BIN)/gocov
$(BIN)/gocov: | $(BASE) ; $(info $(M) building gocov…)
	$Q go get github.com/axw/gocov/...

GOCOVXML = $(BIN)/gocov-xml
$(BIN)/gocov-xml: | $(BASE) ; $(info $(M) building gocov-xml…)
	$Q go get github.com/AlekSi/gocov-xml

GO2XUNIT = $(BIN)/go2xunit
$(BIN)/go2xunit: | $(BASE) ; $(info $(M) building go2xunit…)
	$Q go get github.com/tebeka/go2xunit

STRINGER = $(BIN)/stringer
$(BIN)/stringer: | $(BASE) ; $(info $(M) building stringer…)
	$Q go get golang.org/x/tools/cmd/stringer

TEST-RESULTS = $(BASE)/test-results
$(BASE)/test-results: | $(BASE) ; $(info $(M) creating test-results…)
	$Q mkdir $(BASE)/test-results

ARTIFACTS = $(BASE)/artifacts
$(BASE)/artifacts: | $(BASE) ; $(info $(M) creating artifacts…)
	$Q mkdir $(BASE)/artifacts

# Tests

TEST_TARGETS := test-default test-bench test-short test-verbose test-race
.PHONY: $(TEST_TARGETS) test-xml test
test-bench:   ARGS=-run=__absolutelynothing__ -bench=. ## Run benchmarks
test-short:   ARGS=-short        ## Run only short tests
test-verbose: ARGS=-v            ## Run tests in verbose mode with coverage reporting
test-race:    ARGS=-race         ## Run tests with race detector
$(TEST_TARGETS): NAME=$(MAKECMDGOALS:test-%=%)
$(TEST_TARGETS): test
test: generate lint vendor | $(BASE) ; $(info $(M) running $(NAME:%=% )tests…) @ ## Run tests
	$Q cd $(BASE) && $(GO) test -timeout $(TIMEOUT)s $(ARGS) $(TESTPKGS)

test-xml: generate lint vendor | $(BASE) $(GO2XUNIT) $(TEST-RESULTS) ; $(info $(M) running $(NAME:%=% )tests…) @ ## Run tests with xUnit output
	$Q cd $(BASE) && 2>&1 $(GO) test -timeout 20s -v $(TESTPKGS) | tee $(TEST-RESULTS)/tests.output
	$(GO2XUNIT) -fail -input $(TEST-RESULTS)/tests.output -output $(TEST-RESULTS)/tests.xml

COVERAGE_MODE = atomic
COVERAGE_PROFILE = $(COVERAGE_DIR)/profile.out
COVERAGE_XML = $(COVERAGE_DIR)/coverage.xml
COVERAGE_HTML = $(COVERAGE_DIR)/index.html
.PHONY: test-coverage test-coverage-tools
test-coverage-tools: | $(GOCOVMERGE) $(GOCOV) $(GOCOVXML) $(ARTIFACTS)
test-coverage: COVERAGE_DIR := $(ARTIFACTS)/coverage
test-coverage: generate lint vendor test-coverage-tools | $(BASE) ; $(info $(M) running coverage tests…) @ ## Run coverage tests
	$Q mkdir -p $(COVERAGE_DIR)/coverage
	$Q cd $(BASE) && for pkg in $(TESTPKGS); do \
		$(GO) test \
			-coverpkg=$$($(GO) list -f '{{ join .Deps "\n" }}' $$pkg | \
					grep '^$(PACKAGE)/' | grep -Ev 'vendor/' | \
					tr '\n' ',')$$pkg \
			-covermode=$(COVERAGE_MODE) \
			-coverprofile="$(COVERAGE_DIR)/coverage/`echo $$pkg | tr "/" "-"`.cover" $$pkg ;\
	 done
	$Q $(GOCOVMERGE) $(COVERAGE_DIR)/coverage/*.cover > $(COVERAGE_PROFILE)
	$Q $(GO) tool cover -html=$(COVERAGE_PROFILE) -o $(COVERAGE_HTML)
	$Q $(GOCOV) convert $(COVERAGE_PROFILE) | $(GOCOVXML) > $(COVERAGE_XML)

.PHONY: lint
lint: generate vendor | $(BASE) $(GOLANGCI-LINT) ; $(info $(M) running golangci-lint…) @ ## Run golangci-lint
	$Q cd $(BASE) && $(GOLANGCI-LINT) run ./...

.PHONY: fmt
fmt: ; $(info $(M) running gofmt…) @ ## Run gofmt on all source files
	@ret=0 && for d in $$($(GO) list -f '{{.Dir}}' ./...); do \
		$(GOFMT) -s -l -w $$d/*.go || ret=$$? ; \
	 done ; exit $$ret

# Dependency management

vendor: Gopkg.lock | $(BASE) $(DEP) ; $(info $(M) retrieving dependencies…) @ ## Download dependencies
	$Q cd $(BASE) && $(DEP) ensure
	@touch $@

$(NODE_MODULES): | package-lock.json  ; $(info $(M) retrieving frontend dependencies…) @ ## Download dependencies
	$Q cd $(BASE) && $(NPM) ci

# Build

generate: vendor manifests | $(BASE) $(STRINGER) ; $(info $(M) generating code…) @ ## Generate code
	$Q cd $(BASE) && $(GO) generate ./...

manifests: vendor | $(BASE) $(STRINGER) ; $(info $(M) generating manifests…) @ ## Generate CRD manifests
	go run vendor/sigs.k8s.io/controller-tools/cmd/controller-gen/main.go all

build: vendor generate | $(BASE) ; $(info $(M) building executable…) @ ## Build program binary
	$Q cd $(BASE) && $(GO) build \
		-tags release \
		-ldflags '-X $(IMPORT)/pkg.Version=$(VERSION) -X $(IMPORT)/pkg.BuildDate=$(DATE)' \
		-o bin/$(PACKAGE) main.go

frontend: | $(NODE_MODULES) src/ ; $(info $(M) building frontend…) @ ## Build frontend
	$Q cd $(BASE) && $(NPM) run build

# Docker

PREFIX=$(REGISTRY)/$(AUTHOR)/$(PACKAGE)

.PHONY: container
container: generate vendor frontend | $(BASE) ; $(info $(M) building container…) @ ## Build container
	$Q cd $(BASE) && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GO) build \
		-tags release \
		-ldflags '-X $(IMPORT)/pkg.Version=$(VERSION) -X $(IMPORT)/pkg.BuildDate=$(DATE)' \
		-o bin/$(PACKAGE) main.go
	$Q docker build --pull -t $(PREFIX):$(VERSION) . --no-cache
	$Q docker tag $(PREFIX):$(VERSION) $(PREFIX):latest

.PHONY: push
push: container ## Push container to remote docker registry
	$Q docker push $(PREFIX):$(VERSION)
	$Q docker push $(PREFIX):latest

# Misc

.PHONY: clean
clean: ; $(info $(M) cleaning…)	@ ## Cleanup everything
	@rm -rf bin vendor public node_modules
	@rm -rf test/tests.* test/coverage.*

.PHONY: help
help:
	@grep -E '^[ a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-15s\033[0m %s\n", $$1, $$2}'

.PHONY: version
version: ## Show application version
	@echo $(VERSION)
