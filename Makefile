.SHELLFLAGS := -eu -o pipefail -c
SHELL := bash
.DEFAULT_GOAL := help
.PHONY: help run build setup test test-go test-unit test-js test-e2e test-live-docker test-smoke vet fmt check check-go check-assets check-contracts docs docs-serve docs-contracts check-workflows check-security check-container pr-checks run-watch

QUIET ?= 0
PR ?=
RUN ?=
export PR RUN

ifeq ($(QUIET),1)
.SILENT:
RUN_CHECK = bash scripts/quiet-run.sh
else
RUN_CHECK = bash scripts/run-check.sh
endif

help:
	@echo "make setup       Install locked JS tools and Playwright browsers"
	@echo "make test        Run all Go, JS/unit, API, and browser tests"
	@echo "make check       Full tests, static checks, contracts, asset verification, build"
	@echo "make test-go     Race-enabled Go tests (requires a C compiler)"
	@echo "make test-js     JS/unit and browserless API tests"
	@echo "make test-e2e    Desktop browsers and mobile Chromium"
	@echo "make test-live-docker  Opt-in disposable Docker snapshot test"
	@echo "make run         Start the application"
	@echo "make docs        Build documentation (requires Hugo)"
	@echo "make check QUIET=1  Show phase progress and full output only on failure"
	@echo "make pr-checks [PR=number]  Watch PR checks (default: current branch)"
	@echo "make run-watch RUN=id       Watch a specific GitHub Actions run"

setup:
	npm ci --ignore-scripts
	npm run test:setup

run:
	go run ./cmd/server

build:
	go build -trimpath -o bin/server ./cmd/server

test: test-go test-js test-e2e

test-go:
	$(RUN_CHECK) "Go race tests" env CGO_ENABLED=1 go test -race -count=1 ./...

test-unit:
	$(RUN_CHECK) "Node unit tests" npm run test:unit

test-js: test-unit
	$(RUN_CHECK) "HTTP tests" npm run test:js

test-e2e:
	$(RUN_CHECK) "Browser tests" npm run test:e2e

test-live-docker:
	@test "$${CMP_LIVE_DOCKER:-}" = 1 && test -n "$${CMP_LIVE_ACK:-}" || (echo "Set CMP_LIVE_DOCKER=1 and CMP_LIVE_ACK to the exact Docker socket path" >&2; exit 1)
	$(RUN_CHECK) "Live Docker snapshot test" go test ./internal/runtime -run '^TestLiveDockerSnapshot$$' -count=1 -v

test-smoke:
	$(RUN_CHECK) "Browser smoke tests" npm run test:smoke

vet:
	$(RUN_CHECK) "Go vet" go vet ./...

fmt:
	gofmt -w cmd internal

check-go: vet test-go
	@test -z "$$(gofmt -l cmd internal)" || (echo "Run make fmt"; exit 1)
	$(RUN_CHECK) "Go build" go build ./...

check-assets:
	$(RUN_CHECK) "Asset verification" npm run assets:check

check-contracts:
	$(RUN_CHECK) "Contract structure" npm run contracts:check

check: check-go test-js test-e2e check-assets check-contracts

pr-checks:
	@if [ -n "$$PR" ]; then gh pr checks "$$PR" --watch --interval 20; else gh pr checks --watch --interval 20; fi

run-watch:
	@if [ -z "$$RUN" ]; then echo "Specify a workflow run ID: make run-watch RUN=123" >&2; exit 2; fi
	@gh run watch "$$RUN" --compact --exit-status --interval 20

docs:
	hugo --source docs --minify --panicOnWarning

docs-serve:
	hugo server --source docs --bind 127.0.0.1

docs-contracts:
	hugo server --source docs --config hugo.toml,hugo.contracts.toml --bind 127.0.0.1

check-workflows:
	node scripts/workflows.mjs
	go run github.com/rhysd/actionlint/cmd/actionlint@v1.7.12 -shellcheck=

check-security:
	npm audit --audit-level=low
	go run golang.org/x/vuln/cmd/govulncheck@v1.9.0 ./...

check-container:
	docker build -t go-compose-manager-plus:ci .
	node scripts/scan-image.mjs go-compose-manager-plus:ci
