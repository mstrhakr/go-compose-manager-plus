# Product Staging Verification

Date: 2026-11-19. Environment: Linux development container. Scope: initial
product scaffold and planning, imported from published go-template main
`f281859b8c912ab7e251c7b2a6ddcaf6e663393b`. This record applies to the bootstrap
commit containing it; it does not inherit the template's release evidence.

## Executed

- Initial imported scaffold: `make check-go` passed vet, Go race tests, formatting
  check and build.
- After module rename: first `make check-go` caught a test import still using
  go-template; corrected it, then the same command passed all three Go packages.
- `npm ci --ignore-scripts`: passed; install audit reported zero vulnerabilities.
- `make test-unit check-contracts`: passed 14 Node tests and structure/stable-ID
  checks for two inherited contracts. This does not approve their behavior.
- `git check-ignore scratch/manager.sh`: confirmed uploaded script files ignored.
- Planning/instruction relative-link check and `git diff --check`: passed.

- Final `make check`: passed Go vet/race tests/format/build, 14 Node unit tests,
  6 API tests, 24 desktop/mobile browser tests, embedded-asset verification and
  both inherited contract structure/stable-ID checks.
- `make check-workflows`: passed workflow policy checks and Actionlint.
- `make check-security`: npm audit and govulncheck reported no vulnerabilities.
- `make docs`: passed the default Hugo build with warnings fatal.
- Contract-inclusive Hugo build with `hugo.toml,hugo.contracts.toml`: passed
  with warnings fatal.
- GitHub readback: public visibility; active main/version-tag rulesets;
  secret scanning, push protection and Dependabot security updates enabled;
  SHA-pinned selected Actions with read-only default tokens; squash-only merges;
  release approval configured. Release, container, docs and contract publishing
  variables are explicitly false.

## Not Verified By These Checks

No Compose SDK installed or built; no Docker management behavior exists.
No source-plugin test suites were run. No representative Ubuntu VM measurement,
resource budget, auth, persistence, lifecycle/restart recovery or network/volume
mutation guarantee is verified. Script inspection remains pending.

Native hosted Windows/macOS runs, container runtime, image scans, published
downloads/images/docs, signing, release approvals and consumer attestation
verification are not implied by template imports or local Linux tests.
The owner approved public visibility on 2026-11-19. The public-only GitHub setup
helper completed successfully, with readback confirming active protected-main
and immutable-version-tag rulesets, selected SHA-pinned Actions, disabled
release/container publishing and a release environment requiring owner approval.
This settings readback is not evidence that hosted workflows have run.

## Local Authentication Slice

Date: 2026-11-20. Branch: `feat/local-auth`, based on product main `f3f3820`.
Scope: AUTH-001 through AUTH-006 implementation slice for one-time local admin
bootstrap, local login, forced initial password change, SQLite-backed sessions,
protected dashboard shell and container/Compose configuration. This is partial
contract enforcement, not a production Compose manager.

- `go test ./cmd/server ./internal/auth ./internal/config ./internal/web -count=1`:
  passed.
- `PLAYWRIGHT_PORT=18127 npm run test:js`: passed, 7 tests.
- `PLAYWRIGHT_PORT=18128 npm run test:e2e`: passed, 9 tests; 3 expected skips
  because the one-time provisioning case runs only in desktop Chromium.
- `docker compose config --quiet`: passed.
- `make check`: passed Go race tests, vet, formatting and build; 14 Node unit
  tests, 7 HTTP tests, 9 browser tests with 3 intentional skips; asset and
  contract structure checks.
- `make check-security`: npm audit found no vulnerabilities; govulncheck found
  no reachable vulnerabilities, while reporting one advisory in an imported
  package whose vulnerable function is not called.
- `make docs`: passed with Hugo 0.169.0.
- `docker buildx build --no-cache --load -t go-compose-manager-plus:dev .`:
  passed. A disposable non-root, read-only container and dedicated data volume
  passed login, initial password change and restart persistence checks. A second
  container verified login over the Docker bridge. The image was not published
  or scanned; access through the remote Docker host's published port was not
  verified from this development container.

## 2026-11-21 Follow-Up

Environment: Linux development container. Scope: finish and revalidate the
pending `feat/local-auth` work before delivery.

- `make check`: passed Go race tests, vet, formatting and build; 14 Node unit
  tests, 7 HTTP tests, 9 browser tests with 3 intentional skips; asset and
  contract structure checks.
- `make check-security`: npm audit reported zero vulnerabilities;
  govulncheck reported no reachable vulnerabilities.
- `docker compose config --quiet`: passed.
- `docker buildx build --no-cache --load -t go-compose-manager-plus:dev .`:
  passed.
- `make docs`: not run successfully because Hugo is not installed in the
  current environment.
- A follow-up disposable container smoke could not reach its published or Docker
  bridge port from this development environment, so no additional HTTP
  authentication result is claimed beyond the container verification recorded
  above.
- Initial PR CI found that the Windows test incorrectly expected POSIX mode
  bits, and the container smoke job omitted the required bootstrap credentials.
  The test now checks POSIX mode only on applicable systems; both validation
  and release-image smoke jobs provide disposable test credentials. Locally,
  `go test ./internal/auth -count=1`, `PLAYWRIGHT_PORT=18148 make check`,
  `make check-workflows`, and the Docker liveness smoke passed. Native Windows
  and hosted workflow results for the fixes require CI verification; Windows
  data-directory ACL protection is not tested by the POSIX mode check.
- `node scripts/scan-image.mjs go-compose-manager-plus:dev`: failed on
  CVE-2025-47909 in `github.com/gorilla/csrf`. The Go reachability scan reports
  the affected code is not called, and this application does not configure
  `TrustedOrigins`; nevertheless the image scan blocked the dependency. The owner
  approved retaining token and browser-origin protections while replacing the
  library. The replacement uses `nosurf` masked double-submit tokens with a
  host-only cookie (and `__Host-` prefix on HTTPS), strict same-origin checks,
  and Go's cross-origin middleware; it is not the previous signed cookie.
- After the review fixes, `PLAYWRIGHT_PORT=18148 make check` and
  `make check-security` passed; the latter reported no vulnerabilities.
  `docker buildx build --load -t go-compose-manager-plus:auth-review .`
  and `node scripts/scan-image.mjs go-compose-manager-plus:auth-review`
  passed, with zero image findings. Focused tests cover schema-v1 migration,
  stale session commits after logout, expiry cleanup, oversized auth forms and
  per-client login-key limits. Session cleanup runs on subsequent session
  commits: expired data is cleared and revoked token markers remain until the
  absolute session deadline, preventing stale requests from restoring them.
  No hosted run of these changes is claimed yet.

The dashboard is a visual/authenticated shell with explicit placeholders. Docker
Engine/Compose discovery, real status, network/volume management and lifecycle
actions are not implemented. Remote administration is not certified; configure
TLS/proxy trust carefully and do not expose Docker socket access.

## Fixture-Only Compose Resolution Proof of Concept

Date: 2026-11-21. Linux development container, branch
`feat/compose-resolution-fixtures`, based on `4db7c0d` with the fixture changes
uncommitted during these checks. Compose specification checkout:
`3a4a6995e4a971f6173dff9130cb3e38e6738651`; model loader:
`github.com/compose-spec/compose-go/v2` v2.17.0; Go 1.27.4. The loader is not
the Compose orchestration SDK.

- `COMPOSE_FIXTURE_COMPARE_CLI=1 go test ./internal/composeresolution -count=1 -v`:
  passed. The installed Docker Compose CLI v2.24.1 `config --format json` agreed
  on name and service image for five valid fixtures: top-level name and `.env`
  interpolation, directory-derived name and override, default filename
  precedence, required interpolation from controlled environment, and absolute
  `COMPOSE_FILE` inputs. These comparisons do not invoke the daemon.
- Loader-only assertions check selected ordered filenames, an explicit env file
  with native discovery, explicit Compose files taking precedence over
  `COMPOSE_FILE`, errors for missing explicit Compose/env files, malformed YAML
  and missing required interpolation, and unchanged fixture contents and
  directory entries.
- `PLAYWRIGHT_PORT=18148 make check`: passed Go race tests, vet, formatting and
  build, 14 Node tests, 7 HTTP tests, 9 browser tests with 3 intentional skips,
  and asset and contract structure checks.
- `make check-security`: passed npm audit and govulncheck with no vulnerabilities.
- `make check-container`: built and scanned the Linux amd64 image, with zero
  reported findings.
- `make docs`: could not run because Hugo was not installed.
- `go run github.com/gohugoio/hugo@v0.169.0 --source docs --minify --panicOnWarning`:
  passed using Hugo v0.169.0 as a temporary Go-run tool.
- The same Hugo invocation with `--config hugo.toml,hugo.contracts.toml`
  passed the contract-inclusive documentation build.

These are disposable fixtures with a controlled environment, not integration
with real projects or a claim of full RES compliance. Relative `COMPOSE_FILE`
paths, includes, profiles, parent-directory discovery, broader CLI versions and
the product's process-environment policy remain unverified. In this loader
version, relative `COMPOSE_FILE` paths are resolved against the process working
directory, not the supplied fixture working directory; no product policy is
inferred from that behavior. File checks cover the fixture directory contents,
not metadata or arbitrary external paths. No host scanning, project persistence,
Docker socket access, UI integration or container operation was exercised.