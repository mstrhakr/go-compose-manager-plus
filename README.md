# Compose Manager Plus

A standalone, Compose-native manager being planned for resource-constrained
Ubuntu Docker hosts, including network and volume management. This is the
end-product repository, not a separate planning repository.

**Current stage: scaffold and planning only.** The app currently runs the
go-template greeting/health demonstration. It cannot manage Docker yet. No Docker
socket, SDK dependency, credential store or management routes have been added.

Start with the [product plan](docs/planning/README.md),
[contract reuse map](docs/planning/contract-reuse.md), and
[source provenance](docs/planning/sources.md). Proposed boundaries are not
approved product contracts. Implementation needs agreement on a bounded slice.
Upload the redacted existing VM manager script to [scratch/](scratch/README.md);
its other contents are ignored by Git and excluded from the Docker build.

The scaffold uses one Go executable, escaped HTML templates, local htmx and small
browser JS modules. Node is development/test tooling only. The MIT license and
vendored htmx notice are retained. GitHub repository visibility is public;
release, image and documentation publishing remain disabled.

## Start here

Install Go **1.27.4** (see `go.mod`), then:

```sh
go run ./cmd/server
curl http://localhost:8080/healthz
```

Open <http://localhost:8080/>. Set `APP_ADDR=127.0.0.1:9000` to change the
listener and `APP_LOG_LEVEL=debug` to adjust JSON logging. The default listener
binds all interfaces; use loopback for local-only development.
The service validates configuration and shuts down gracefully on SIGINT/SIGTERM.

## Develop

Install Node **24**, GNU Make, and a C compiler for Go's race detector. Then:

```sh
make setup      # locked npm install, browser binaries (once)
make test       # ALL Go, JS/unit, API, and browser tests
make check      # tests + vet/format/build + contracts + embedded assets
make build      # bin/server
```

On Linux, Playwright may also need system libraries:

```sh
npm exec -- playwright install --with-deps chromium firefox webkit
```

Use `make` for a command list; VS Code's default Test task runs `make test`.
Focused targets: `make test-go`, `make test-js`, `make test-e2e`.
A separate test.sh is unnecessary: Make owns the orchestration. `make check`
does not silently include Docker, Hugo, or vulnerability scans; run their explicit
checks when those surfaces change, as described in the development guide.

Make is optional; see the Makefile for equivalent Go commands.

```text
cmd/server/       process wiring, lifecycle, health probe
internal/config/  validated environment configuration
internal/web/     HTTP handlers, HTML layouts/partials, embedded assets
tests/            JS/unit, API, desktop/mobile browser regressions
scripts/          test, security, CI/release and repository setup helpers
docs/contracts/   canonical behavior definitions (not published by default)
docs/             self-contained Hugo documentation site
.github/          CI, dependency updates, opt-in publishing
```

Keep application logic in focused `internal` packages as requirements emerge.
No database, authentication, scheduler, or client-rendering framework is imposed.
Go owns business rules and escaped HTML; htmx owns server-driven updates; small
JS modules own browser-local behavior. The greeting form demonstrates this and
still works with JS disabled. See the [contract guide](docs/contracts/README.md).
This foundation is imported from go-template; no Compose Manager or Docker
Networks runtime code has been copied. Template development/deployment docs below
describe the scaffold, not implemented Compose-management workflows.

## Deploy locally

```sh
cp .env.example .env
docker compose up --build -d
docker compose logs -f
docker compose down
```

Compose binds `127.0.0.1:8080` by default. Change `APP_PORT` in `.env` for its
host port; native Go execution reads exported variables, not `.env`.
The image is multistage, non-root, and includes a built-in health probe. Compose
uses a read-only filesystem and drops Linux capabilities. Put a TLS reverse
proxy in front of any remotely accessible deployment.

## Documentation

Install Hugo **0.169.0** from its [official releases](https://github.com/gohugoio/hugo/releases),
verifying the release checksum, then:

```sh
make docs-serve  # http://localhost:1313/
make docs       # docs/public/
```

Start with [getting started](docs/content/getting-started.html),
[architecture](docs/content/architecture.html),
[development](docs/content/development.html), and
[deployment](docs/content/deployment.html). Content, layouts, and styles live
together under `docs/`, with no third-party theme.

## Automation and privacy

- CI runs change-scoped checks with a stable required `CI gate`; PR validation
  is read-only and secret-free. Contract-only changes skip app jobs.
- Dependabot groups minor/patch updates per Go, npm, Docker, or Actions ecosystem;
  majors get individual PRs. Version checks are weekly; security updates are
  separate. Some tool pins and vendored htmx assets need manual maintenance.
- Actions/base images/tools are pinned; npm installs skip lifecycle scripts.
- **Publishing is opt-in.** After setup, `PUBLISH_CONTAINER=true` enables dev
  images from relevant passing main CI. Beta/RC are explicit actions; normal
  releases use Release Please PRs, full validation and environment approval.
- Initial downloads: Linux/Windows x64, macOS x64/ARM64. Images: Linux x64.
- Downloads include checksums/provenance; image tags separate dev/beta/RC/normal.
- Pages requires `PUBLISH_DOCS=true`, Pages configured for GitHub Actions, and
  a manual default-branch run with explicit publication confirmation.
- **A private repository can still have a public Pages site.** Review content
  and access controls before enabling publishing. Repository/package visibility
  is never changed by these workflows.

See [delivery and setup](docs/content/deployment.html) and
[security policy](SECURITY.md). GitHub settings do not automatically transfer
to repositories created from a template.

## Foundation And Safety

Imported from published go-template main at
`f281859b8c912ab7e251c7b2a6ddcaf6e663393b`, without upstream Git history or
release tags. Product release numbering is independent; no product release exists.
See [executed verification](docs/verification.md) for what was actually checked.

The owner approved public visibility to enable the full GitHub protection
baseline. The inherited setup helper configured protected main, immutable
version tags, scoped Actions, security features and release approval without
enabling publishing. The inherited
release provenance schema URI and generic CI image labels may still refer to
go-template; they are template tooling identifiers, not upstream release claims.

The native starter is unauthenticated and binds all interfaces by default. Use
loopback for staging. Docker socket access is effectively host-administrative;
authentication, authorization, host/container paths and recovery must be decided
before adding reachable administrative routes. The starter deployment is not
yet a production Compose-manager deployment.
