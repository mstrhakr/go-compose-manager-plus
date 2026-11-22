# Compose Manager Plus

A standalone, Compose-native manager for resource-constrained
Ubuntu Docker hosts, including network and volume management. This is the
end-product repository, not a separate planning repository.

The functional baseline is the Unraid Compose Manager Plus plugin, reusing its
useful workflows and portable business rules with a modern standalone interface.
UniFi is the design inspiration; Unraid-specific UI and legacy compatibility
machinery are not the target. Features will arrive in bounded, tested slices.

**Current stage: local authentication and read-only project discovery.** Save
one application-visible root and request a scan to persist discovered project
entries. Discovery and permission-only activation do not connect to Docker or
run Compose actions. Container and resource values are placeholders, not host
status. A Linux amd64 development image is published at
`ghcr.io/mstrhakr/go-compose-manager-plus:dev`; it reflects the latest published
main, not unmerged branch work.

Start with the [product plan](docs/planning/README.md),
[contract reuse map](docs/planning/contract-reuse.md), and
[source provenance](docs/planning/sources.md). These documents distinguish
approved direction from proposals; the initial authentication/dashboard slice
was explicitly authorized.
Upload the redacted existing VM manager script to [scratch/](scratch/README.md);
its other contents are ignored by Git and excluded from the Docker build.

The scaffold uses one Go executable, escaped HTML templates, local htmx and small
browser JS modules. Node is development/test tooling only. The MIT license and
vendored htmx notice are retained. GitHub repository visibility is public;
development-image publishing is enabled, while release and documentation
publishing remain disabled. Package access is controlled separately.

## Start here

Install Go **1.27.4** (see `go.mod`). A fresh native run needs a bootstrap admin;
keep the temporary password out of shell history and logs:

```sh
read -rp 'Admin username: ' APP_BOOTSTRAP_USERNAME
read -rsp 'Temporary admin password: ' APP_BOOTSTRAP_PASSWORD
printf '\n'
export APP_BOOTSTRAP_USERNAME APP_BOOTSTRAP_PASSWORD
go run ./cmd/server
```

Open <http://127.0.0.1:8080/> and complete the forced first-login password change.
The saved account/database is in `APP_DATA_DIR` (default `./data`); later restarts
ignore changed bootstrap env values. The native listener defaults to loopback.
`APP_ADDR`, `APP_PUBLIC_URL`, `APP_TRUSTED_PROXIES` and `APP_LOG_LEVEL` control
listen/origin/proxy/log configuration. Do not expose admin routes without TLS,
proxy trust, request protection and app authentication configured.

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
cmd/server/       process wiring, auth-state lifecycle, health probe
internal/config/  validated environment configuration
internal/auth/    SQLite account/bootstrap state, Argon2id, SCS sessions
internal/web/     auth/access handlers, dashboard shell, local assets
tests/            JS/unit, API, desktop/mobile browser regressions
scripts/          test, security, CI/release and repository setup helpers
docs/contracts/   canonical behavior definitions (not published by default)
docs/             self-contained Hugo documentation site
.github/          CI, dependency updates, opt-in publishing
```

Keep application logic in focused `internal` packages as requirements emerge.
Go owns business rules and escaped HTML; htmx owns server-driven updates; small
JS modules own browser-local behavior. Authentication is a first foundation;
Docker operations, live state and schedules remain separate slices. See
the [contract guide](docs/contracts/README.md).
This foundation is imported from go-template; no Compose Manager or Docker
Networks runtime code has been copied. Development/deployment docs below describe
the current auth foundation and repository tooling, not implemented
Compose-management workflows.

## Deploy locally

```sh
cp .env.example .env
# Set APP_BOOTSTRAP_USERNAME and a temporary APP_BOOTSTRAP_PASSWORD in .env
docker compose up --build -d
docker compose logs -f
```

Open <http://127.0.0.1:8080/> and complete first-run setup. Compose binds loopback
and persists the database in its `app-data` volume at `/data`. After setup, remove
the bootstrap username/password from `.env` and restart; saved credentials remain
authoritative and are not reset from environment values. Protect `.env` and Docker
administration access: bootstrap variables may be visible to Docker administrators.
For a reverse proxy set `APP_PUBLIC_URL` to the external HTTPS origin and
`APP_TRUSTED_PROXIES` to only the immediate proxy CIDRs; the proxy must prevent
untrusted direct-backend access. The image remains non-root, read-only except for
the data volume, and drops Linux capabilities.

To test the published Linux amd64 development image instead of building locally,
create a separate `compose.dev.yaml` alongside `compose.yaml`:

```yaml
services:
  app:
    image: ghcr.io/mstrhakr/go-compose-manager-plus:dev
```

Set the temporary bootstrap values in the ignored `.env` as above, restrict access
to it (for example `chmod 600 .env` on Linux), then run:

```sh
docker compose -f compose.yaml -f compose.dev.yaml pull
docker compose -f compose.yaml -f compose.dev.yaml up -d --no-build --pull never
```

The base Compose file normally builds locally; the override and explicit pull
select the registry image. After setup, remove the bootstrap values and recreate
the service with the same `up` command. The dev tag can move; use a SHA tag or
digest when a fixed image is needed. Registry package visibility is separate
from repository visibility and may require authentication.

### Discover existing projects

Mount the directory containing project folders read-only, for example by adding
this to the `app` service in a local Compose override (replace the host path):

```yaml
services:
  app:
    volumes:
      - /absolute/host/projects:/projects:ro
```

Recreate the service, sign in, save `/projects` as the discovery root, and
select **Scan for projects**. The app user must have read/traverse permission on
the host directory. Paths shown in the UI are paths inside the container, not
necessarily Docker-host paths. Discovery is manual, reads direct-child folders
only, and does not require the Docker socket.

Newly discovered projects start inactive; activation changes manager permission
only and does not start, stop or alter them. Missing entries remain saved, and a
project found again at the same path retains its saved activation state; a path
match is not proof of identity. Input resolution uses only the project's local
`.env`, not the manager's environment, and may differ from the environment used
to launch an existing stack. `.env`-only `COMPOSE_FILE` discovery is deferred;
projects whose local `.env` sets `COMPOSE_FILE` are listed with an unsupported
input error rather than silently resolved with other files. Symlinked child
entries are skipped and reported. Some `.env` expressions referencing unset
variables, including defaults, may be conservatively flagged. Compose inputs can
reference paths outside the project folder that the app can read; a read-only
mount prevents writes but is not a read-access sandbox. Other environment,
path and activation edge cases remain under review. The initial `.env` reader
limits files to 1 MiB and 4,096 lines, with individual statements limited to
128 KiB and 256 lines.

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
version tags, scoped Actions, security features and release approval; development
image publishing was enabled later. The inherited
release provenance schema URI and generic CI image labels may still refer to
go-template; they are template tooling identifiers, not upstream release claims.

The initial authentication/dashboard shell is not yet a production Compose
manager. Docker socket access remains absent. Authentication does not itself
authorize future per-project operations; request/proxy/session and host/container
path policies still require verification before remote administration.
