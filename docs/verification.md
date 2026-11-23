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

## Persistent Discovery Slice

Date: 2026-11-21. Linux development container, branch `feat/read-only-discovery`
based on `4f8215b`; results refer to the uncommitted implementation alongside
the branch's decision and schema commits. The slice implements a single saved
root, requested direct-child scans, persistent listings and permission-only
activation. It uses compose-go v2.17.0 without inheriting the manager process
environment. It does not connect to Docker or run Compose actions.

- `go test ./internal/discovery ./internal/web ./internal/auth ./cmd/server -count=1`:
  passed. Cases cover schema v1/v2 migration with auth/session preservation,
  root and project persistence, stale scan/activation rejection, discovery of
  valid and invalid projects, missing/restored entries, local interpolation,
  skipped symlinks, failed scans, authenticated requests and activation without
  implicitly discovering other projects.
- `PLAYWRIGHT_PORT=18148 make check`: passed after activation revalidation;
  includes Go race tests, vet, build, 14 Node tests,
  7 HTTP tests, 9 browser tests with 3 intentional skips, assets and contract
  structure checks. The browser flow exercised discovery/activation in desktop
  Chromium and the resulting dashboard on mobile Chromium.
- `make check-security`: passed npm audit and govulncheck, no vulnerabilities.
- `make check-container`: Linux amd64 image built and scanned with zero findings.
- Contract-inclusive Hugo v0.169.0 build via `go run` passed.

The scanned project filesystem is external to manager-owned state: it can change
after validation, and activation is not a guarantee of safe targeting by future
actions. Path restoration is not identity proof. Availability is last-scan state,
not live status. `COMPOSE_FILE` selection is explicitly unsupported during this
slice, and `.env` defaults referring to missing variables can be conservatively
flagged. The bounded `.env` reader checks references in file order and limits
file and statement sizes. Compose input references may read outside the project folder if
accessible to the process; a read-only mount prevents writes but is not a read
sandbox. No native-host, Docker operations, full Compose compatibility or
comprehensive contract compliance is claimed.

## UI Foundation Slice

Environment: Linux development container. The dashboard was restructured using
the owner's supplied design reference; the reference's simulated runtime data
and actions were not implemented. The theme follows the system or an explicit
browser-local selection. Discovery and permission-only activation remain the
only project workflows in this slice.

- `PLAYWRIGHT_PORT=18176 make check`: passed Go vet, race tests and build,
  14 Node tests, 7 HTTP tests, 17 browser tests with 3 intentional skips,
  embedded-asset and contract-structure checks.
- Chromium exercised first login, password setup, discovery errors, keyboard
  navigation, scanning, activation, theme persistence, no-JavaScript and
  touch-enabled mobile scanning. Login and theme behavior were also exercised
  in Firefox, WebKit and mobile Chromium. Desktop checks measure the header,
  rail, sidebar and telemetry dimensions and test horizontal overflow.
- Desktop dark and mobile light screenshots were inspected locally. This is
  not a pixel-comparison test against the reference, nor native-device evidence.
- `make docs` could not run because the Hugo binary is not installed;
  `go run github.com/gohugoio/hugo@v0.169.0 --source docs --minify --panicOnWarning`
  passed instead.

Runtime telemetry, Docker integration and management actions remain unimplemented.
The supplied reference was inspected as text, not executed or included in the app.

## Reusable UI Components

Environment: Linux development container. Scope: server-rendered metric, chart
and selectable list panels, compact filter styling, and a test-only component
fixture. The production dashboard continues to show unavailable runtime data;
the fixture is not an application route and its data is synthetic.

- `PLAYWRIGHT_PORT=18187 make check`: passed Go vet, race tests and build,
  14 Node tests, 7 HTTP tests, 37 browser tests with 3 intentional skips,
  embedded-asset and contract-structure checks.
- Browser coverage exercises chart ranges, missing samples, accessible data,
  empty and invalid states, independent multi-selection, disabled entries,
  keyboard operation, htmx reinitialization, no-JavaScript behavior, themes
  and responsive overflow in Chromium, Firefox, WebKit and mobile Chromium.
  Desktop and mobile dark/light fixture screenshots were inspected locally.
- `PLAYWRIGHT_PORT=18188 npx playwright test tests/e2e/components.spec.js
  --project=chromium --project=mobile-chromium`: passed 10 tests after adding
  touch-selection and chart-data removal checks.
- `make check-security`: npm audit and govulncheck reported no vulnerabilities.
- Hugo 0.169.0 documentation build through its Go runner passed. A standalone
  Hugo executable is not installed.

These are component-level results, not evidence of live Docker metrics,
filtering, bulk operations, real-device testing or production accessibility
certification.

PR review follow-up: `PLAYWRIGHT_PORT=18191 make check` and the Hugo 0.169.0
Go-runner build passed after preserving missing chart observations in the
accessible table, namespacing list events, and correcting text contrast.
Browser tests assert at least 4.5:1 contrast for the reviewed theme colors.

## Discovery Scan Diagnostics

Environment: Linux development container. Based on merged UI revision
`ccf1865`, this bounded follow-up adds categorized scan failure logs without
logging paths or file contents. It does not change scanning, activation, or
failed-scan retention policy under PROJ-006, PROJ-010 and PROJ-011.

- `go test ./internal/web ./cmd/server -count=1`: passed, including a failed
  authenticated scan that preserves the previous project state and logs a
  `not_found` category without the root path.
- `PLAYWRIGHT_PORT=18192 make check`: passed Go vet/race/build, 14 Node tests,
  7 HTTP tests, 37 browser tests with 3 intentional skips, asset and contract
  structure checks.
- `make check-security`: npm audit and govulncheck reported no vulnerabilities.
- Hugo 0.169.0 documentation build through its Go runner passed.

The reported user's container and host filesystem were not accessed; the
previously shared Compose configuration had no projects bind mount. The exact
cause of that deployment's failure remains unverified until tested there.

## Compact Development Checks

Environment: Linux development container. `PLAYWRIGHT_PORT=18193 make check
QUIET=1` passed with per-phase progress; `PLAYWRIGHT_PORT=18194 make check`
passed with default output. Node tests verify the quiet runner prints complete
failure output and retains its exit status, and that GitHub watch targets
forward selectors and report failures. `make -s pr-checks PR=9` successfully
read completed PR checks. `make check-workflows`, `make check-security`, and
the Hugo 0.169.0 Go-runner docs build passed. An actual failing GitHub run and
native non-Linux Make environments were not exercised.

## On-Demand Docker Snapshot

2026-11-22, Linux development container, branch `feat/read-only-runtime-status`
based on `cafa352`: the app reads Docker Engine API v1.40 only on explicit
dashboard refresh. It groups container summaries by Compose project name; this
is not filesystem ownership evidence or authorization for actions.

- `CMP_LIVE_DOCKER=1 CMP_LIVE_ACK=/var/run/docker.sock make test-live-docker
  QUIET=1` passed against Docker Engine 29.5.3. It created a uniquely named
  Compose fixture container, verified its `created` state and service label
  through the snapshot, removed its verified container ID and checked removal.
  No image pull, build, published ports, host bind mounts or global cleanup was
  used. The fixture did not exercise a running or exited container.
- `PLAYWRIGHT_PORT=18198 make check QUIET=1` passed Go vet, race tests and
  build, Node, HTTP and browser tests, asset verification and contract checks.
- A temporary, isolated app instance using the current binary, separate
  ephemeral state, a read-only playground mount and the host socket was
  exercised in Chromium. It scanned 27 playground projects, displayed a
  successful Docker snapshot and resolved names for all 27. Desktop and mobile
  viewport checks reported no horizontal overflow. It was stopped and removed.
- `make check-security` passed npm audit and govulncheck. `make docs` could not
  run because Hugo was not installed; the pinned `go run
  github.com/gohugoio/hugo@v0.169.0 --source docs --minify --panicOnWarning`
  build passed.
- `make check-container` built the Linux amd64 image and scanned it with Trivy;
  no vulnerabilities were reported. The local build image and scanner image
  remain available in Docker's image store.

The shared daemon is not isolated by a read-only socket bind mount. No project
lifecycle action, running-state fixture, automatic refresh, events, metrics,
remote-administration certification or full Docker compatibility is claimed.

PR review follow-up: the live Docker test was rerun after fixing cleanup to
remove and verify the fixture's anonymous volume along with its container.
The authenticated handler regression also checks that a project restored since
the last discovery scan can display current runtime status without changing
saved discovery availability or activation.

## Project And Container Inventory Foundation

Linux development container, branch `feat/project-container-views` based on
`b97ca66`: distinct authenticated inventory pages use the existing read-only
Docker container-list endpoint. Visiting either page requests a snapshot; the
overview still requests one only on refresh. Go integration tests use a mock
Unix-socket daemon to cover unlabelled and stopped containers, escaped names,
same-name folders, unresolved inputs, empty and failed snapshots, authentication
gates and the one-request-per-view boundary. Discovery form redirects are
restricted to local inventory pages or the overview.

- `go test ./internal/runtime ./internal/web` passed focused tests.
- `PLAYWRIGHT_PORT=18208 make check QUIET=1` passed Go vet and race tests,
  build, Node, HTTP, browser, asset and contract checks. The authenticated
  inventory interaction test runs in Chromium, including keyboard, a simulated
  mobile touch viewport, long paths, no JavaScript and overflow checks; the
  other browser projects skip that bootstrap-dependent interaction.
- `make docs` passed with Hugo 0.169.0.

This slice was not tested against a live Docker daemon. Its tests do not establish
native mobile behavior, continuous freshness, lifecycle control or resource
ownership, and no such behavior is claimed.

Inventory redesign follow-up on the same branch: server-rendered search, sorting
and facet combinations have unit and authenticated mock-daemon HTTP coverage,
including stale selected facets and no-match results. Chromium exercised filter
controls without JavaScript and in a simulated mobile touch viewport. Mock
screenshots were captured across multiple iterations, including a filtered view,
light theme and mobile layouts; these are design checks, not live-Docker or native
mobile evidence. `PLAYWRIGHT_PORT=18211 make check QUIET=1` passed after fixing
the shared component fixture.

The inventory filter sidebar and overview host panel use native disclosure
controls. The main navigation can expand to labels and remembers its setting
in browser storage; storage errors are reported. Desktop keyboard, simulated
mobile touch and JavaScript-disabled disclosure paths are covered by the browser
test. The expanded navigation is a browser enhancement and remains icon-only
without JavaScript.

The owner approved separate Overview, Projects and Containers primary entries,
with disabled placeholders for Resources, Activity and Settings. The shared
view-navigation component renders a current page link and disabled future
subviews in the contextual sidebar; these placeholders do not implement or
approve the corresponding functionality. Project status labels are shared by
the dashboard, inventory rows and inventory filters.
`PLAYWRIGHT_PORT=18219 make check QUIET=1` and `make docs` passed after these
changes. Browser checks cover current and disabled navigation entries as well
as the desktop keyboard, simulated touch and JavaScript-disabled paths above.

## Docker Metrics Storage Proof Of Concept

2026-11-22, Linux x86_64 development environment, branch
`spike/docker-metrics-storage`, based on `c723364`. The owner approved a bounded
proof of concept and a read-only test against the running Compose Manager
container on the shared Docker daemon. No container was created or modified.
The test uses the Docker Engine v1.40 per-container stats stream and a temporary
tstorage v0.3.6 database, not the application SQLite database. Only a
test-only package imports tstorage; no collector is started by the app.

- `go test -race -count=1 ./internal/metricspoc` passed deterministic tests
  for sample mapping, missing observations and clean storage reopen.
- `CMP_METRICS_SOCKET=/var/run/docker.sock CMP_METRICS_CONTAINER=compose-manager-plus go test -race -count=1 -run '^TestLiveStatsStorage$' -v ./internal/metricspoc`
  passed. The ten-second stream produced ten responses and 59 metric points
  (nine CPU, ten each of raw memory, network receive/transmit and block
  read/write counters); all points were readable after close and reopen.
  The resulting temporary directory was 1,678 bytes after close; Go reported
  144,944 bytes allocated during the measured test loop.
- `make check-security` reported no npm or Go vulnerabilities.

The sample uses raw memory usage and cumulative network/block byte counters,
not CLI-adjusted memory or derived rates. CPU requires a prior valid counter
sample. The first stream response lacked usable previous CPU counters, so it
was not stored as zero. The test removes its temporary database.

This short single-container run does not measure Docker daemon overhead,
production memory usage, multi-container scale, retention or downsampling,
hard disk budgets, crash recovery, long-running collection, native Windows/macOS
behavior or performance on a representative low-power machine. The directory
size and Go allocations are observations for this run, not preset budgets.
tstorage's retention is partition-based, and its own shutdown requires callers
to stop writers before close. Compatibility, recovery and retention need
additional investigation before the dependency is used by the running app.

Subsequent static inspection of tstorage v0.3.6 found that its public storage
interface exposes insert, select and close, but no oldest-first eviction or
storage-budget API. Its retention check runs hourly and applies to disk
partitions based on partition creation time; the two writable memory
partitions and WAL also need accounting. Its asynchronous partition flush
errors are logged rather than returned by the insert that triggered the flush.
These observations do not establish that the approved early-eviction, one-hour
history, warning or recovery behaviors can be enforced. A safe design and
targeted tests are needed before enabling collection.

### Synthetic One-Hour Storage Measurement

2026-11-22, Linux x86_64 development container, branch
`spike/docker-metrics-storage`, based on `a3b010d`. An opt-in synthetic test
inserted six generated metrics per container at one-second timestamps for one
hour, using five-minute tstorage partitions, an unbuffered WAL and 64-character
container labels. No Docker daemon was accessed.
`CMP_METRICS_CAPACITY_TEST=1 go test -race -count=1 -run '^TestSyntheticHourCapacity$' -v ./internal/metricspoc`
passed and verified the count of every series after closing and reopening:

| Containers | Scalar points | Bytes at end of ingestion | Bytes after close |
| ---: | ---: | ---: | ---: |
| 1 | 21,600 | 588,824 | 208,971 |
| 5 | 108,000 | 2,938,242 | 1,037,813 |
| 25 | 540,000 | 14,686,381 | 5,183,340 |
| 50 | 1,080,000 | 29,371,640 | 10,365,393 |

These are individual file lengths observed in one accelerated run, not peak
disk use or allocated filesystem blocks. The data values are synthetic and
compressibility, WAL and partition behavior depend on workload; this does not
measure Docker overhead, process memory, realtime collection, deletion,
filesystem exhaustion, crash recovery or the production storage design.
The results do not set preset budgets or validate oldest-first eviction.

### Docker Stats Conversion Foundation

2026-11-22, Linux x86_64 development container, branch
`spike/docker-metrics-storage`, based on `a2d9ff6` with these changes
uncommitted during the checks. The runtime package can request one bounded
read-only Docker stats response and convert consecutive observations from the
same uninterrupted container into CPU utilization and network/block byte
rates. Raw memory usage and limit preserve missing values. Counters are
compared per interface or block device so resets and membership changes do not
become false rates. The caller must discard its previous sample across a
disconnect or container identity change. No background collection is wired.

- `go test -race -count=1 ./internal/runtime ./internal/metricspoc`: passed,
  using fixture and mock HTTP responses; the opt-in live test was skipped.
- `PLAYWRIGHT_PORT=18334 make check QUIET=1`: passed Go race tests, vet, build,
  Node, HTTP and browser tests, asset verification and contract structure.
- `make docs`, `make check-docs` and `git diff --check`: passed.

The new on-demand endpoint and calculations have not been tested against a live
daemon, all platform/cgroup variants, a scheduler, storage or the UI. The
five-second snapshot timeout is not a streaming implementation or proof of the
one-second target. Retention enforcement, safe eviction and recovery remain
unimplemented.

### tstorage Partition Write Failure Probe

Linux development container, branch `spike/docker-metrics-storage`, baseline
`4b3862d`. A temporary, subsequently removed race-enabled Go test inserted one
point into a temporary tstorage v0.3.6 database with an unbuffered WAL. It
simulated an exhausted data file by linking the test partition's `data` file to
`/dev/full`, without filling the filesystem. `Close` logged a no-space error
but returned nil. After removing the simulated failed partition's data link and
metadata, reopening did not recover the point from the WAL. The test passed in
the sense that it reproduced this failure, not that storage met the contract.
No Docker daemon was accessed.

This confirms a specific failure mode, not the behavior of every full-disk or
crash scenario. In this case the library's partition flush logs an encoding
failure instead of returning it, and its normal close path can remove the WAL.
Capturing the log alone therefore does not prove that acknowledged history can
be recovered. The owner subsequently selected SQLite for the initial store;
the approved pause, warning and retry behavior still needs verification.

### SQLite Metrics Persistence Foundation

Linux development container, branch `spike/docker-metrics-storage`, based on
`ca69169` with the changes uncommitted during these checks. A separate SQLite
metrics database can atomically insert and query numeric container observations,
retain missing fields, reopen, and logically delete observations before a
specified cutoff. It is not connected to the application or Docker. The owner
selected SQLite for the initial one-hour implementation; TimescaleDB and a
migration path are future possibilities, not implemented.

- `go test -race -count=1 ./internal/metrics ./internal/runtime`: passed.
  Tests include reopening, missing versus zero, time boundaries, duplicate
  transaction rollback, concurrent calls, unsupported schemas, cancellation,
  and retry after a simulated SQLite page-limit failure.
- `PLAYWRIGHT_PORT=18334 make check QUIET=1`: passed Go race tests, vet and
  build, Node, HTTP and browser tests, asset verification and contracts.
- `make docs`, `make check-docs` and `git diff --check`: passed.

The page-limit test is not a real filesystem-full, WAL-write or crash-recovery
test. Logical deletion does not guarantee filesystem space is returned. No
one-hour expiry scheduler, storage-budget accounting, early eviction, warning,
write-pause/retry mechanism, event/state storage, collection, migration,
performance measurement or live Docker test is verified by this slice.

Review of `3304849` found that a Docker memory value above SQLite's signed
integer range rejected the entire batch. The follow-up stores memory values as
decimal text and migrates version-one rows transactionally. Focused race tests
cover the full unsigned range, reopening and migration. This does not resolve
the storage-budget or collector gaps listed above.

The follow-up also adds unwired persistence for discrete container state and
observed lifecycle/health events, and a bounded read-only Docker event stream.
`go test -race -count=1 ./internal/runtime ./internal/metrics` passed with
mock HTTP responses, idle-stream cancellation, persistence, invalid records,
ordering and reopen tests. The event stream does not reconcile snapshots,
reconnect, replay, deduplicate or enforce scope, and has not been tested against
a real daemon. State snapshots do not establish continuous state between
observations. Event/state retention and budget accounting are not implemented.
An initial full check encountered two Firefox fixture setup timeouts; the
focused Firefox component suite subsequently passed, followed by
`PLAYWRIGHT_PORT=18334 make check QUIET=1`, `make docs`, and `make check-docs`.

### Synthetic SQLite One-Hour Measurement

Linux development container, branch `spike/docker-metrics-storage`, based on
`aed51d0`, with the measurement test uncommitted. The command
`CMP_METRICS_SQLITE_CAPACITY_TEST=1 go test -count=1 -run '^TestSyntheticHourSQLiteCapacity$' -v ./internal/metrics`
passed. It writes one-hour synthetic observations containing seven numeric
values per container, batching each simulated minute into one SQLite
transaction, and checks every observation after close and reopen. No Docker
daemon was accessed.

| Containers | Observations | File lengths at end of ingestion | Database file length after close |
| ---: | ---: | ---: | ---: |
| 1 | 3,600 | 3,382,336 bytes | 954,368 bytes |
| 5 | 18,000 | 7,565,560 bytes | 4,542,464 bytes |
| 25 | 90,000 | 26,628,256 bytes | 22,814,720 bytes |
| 50 | 180,000 | 49,698,184 bytes | 45,305,856 bytes |

The ingestion column sums database, WAL and shared-memory file lengths; neither
it nor the after-close size represents peak or allocated disk usage. Generated
values and minute-sized transactions do not model real workloads, one-second
commits or a collector's CPU, memory, daemon or filesystem overhead. This
measurement does not test eviction, physical disk exhaustion, crash recovery or
set preset budgets. Two earlier attempts were interrupted because they were
too slow; their temporary test data was inspected and removed.

### SQLite Migration And Retention Primitives

Linux development container, branch `spike/docker-metrics-storage`, based on
`1ce627c`, with changes uncommitted during these checks. Review found that the
version-one and version-two schema migrations were nested under new-database
creation: existing databases retained their older schema. The earlier
version-one test checked only that an old sample could be read, not that the
schema actually advanced. The corrected migration and tests exercise both
upgrade paths and the resulting event, state and full-width memory writes.

The unwired store now supports a transactional cutoff across sample, event and
state records, logical SQLite file-length accounting and an explicit WAL
checkpoint that reports blocking readers. `go test -race -count=1
./internal/metrics ./internal/runtime` passed, including a forced transaction
failure with rollback, exact-cutoff preservation, reopen, file accounting and
blocked-checkpoint retry.
`PLAYWRIGHT_PORT=18334 make check QUIET=1`, `make docs`, `make check-docs` and
`git diff --check` also passed.

An additional unwired oldest-first sample eviction operation returns its
removed time range and keeps event history intact. `go test -race -count=1
./internal/metrics ./internal/runtime` passed ordering, tied timestamps,
reopen, empty store and injected-transaction-failure checks. This is logical
deletion only: it does not by itself recover disk space or warn users.

These are not a scheduled one-hour retention implementation or a budget
controller. Logical deletion can leave allocated database pages behind; file
lengths are not atomic snapshots or filesystem block counts. Filesystem-full
recovery, earliest-first pressure eviction, warning and write pause/retry,
collector startup and shutdown, and Docker integration remain unverified.

### Durable Sample-Eviction Summary

Linux development container, branch `spike/docker-metrics-storage`, based on
`c209dcf` with changes uncommitted during these checks. Schema version 4 records
a cumulative sample-eviction count and the oldest and newest removed sample
timestamps in the same transaction as each nonempty eviction. The bounds do not
describe a continuous gap or identify affected containers. The summary persists
across normal expiry and reopen; it cannot reconstruct prior evictions in
version 3 or earlier. No acknowledgement or automatic clearing policy is
implemented.

The focused metrics and runtime race tests cover reopen, concurrent evictions,
schema-version-3 migration, injected delete and summary-write failures, and
count overflow: `go test -race -count=1 ./internal/metrics ./internal/runtime`
passed. `PLAYWRIGHT_PORT=18334 make check QUIET=1`, `make docs` and
`make check-docs` passed. These checks do not establish crash or physical-disk-full
recovery. The store remains unwired: it has no user-facing warning, budget
controller, write pause/retry, scheduled retention or collector.

### SQLite Page-Reuse Probe

Linux development container, branch `spike/docker-metrics-storage`, based on
`b6fe3c6` with changes uncommitted during these checks. The focused real-SQLite
test `go test -race -count=1 -run
'TestDeletedPagesCanBeReusedWithoutShrinkingDatabase|TestPageUsageReportsCancellationAndClose'
-v ./internal/metrics` passed. With 5,000 synthetic samples and a simulated
SQLite page-count limit, oldest-first deletion of 4,000 retained the event and
the remaining samples. After a successful WAL checkpoint, the database still
measured 983,040 logical bytes, but 184 of its 240 4,096-byte pages were on
SQLite's freelist. Inserting 2,000 new samples reused pages without increasing
the database page count; the freelist fell to 90 pages and the WAL grew to
428,512 logical bytes before its checkpoint. The eviction itself had produced
a 795,192-byte WAL before its checkpoint. The resulting samples and
eviction summary survived reopening.
`go test -race -count=1 ./internal/metrics ./internal/runtime`,
`PLAYWRIGHT_PORT=18334 make check QUIET=1`, `make docs` and `make check-docs`
passed for the page-accounting change.

This is one synthetic workload, not a budget recommendation. SQLite freelist
pages are reusable within the database, not reclaimed filesystem space; WAL
growth can exceed the proposed 10% headroom even when reusable pages exist.
These measurements do not establish peak, allocated-block usage, physical
disk-full recovery, an eviction batch size or a safe resume rule. The new
page-accounting method is an unwired observation, not budget enforcement.

In a separate synthetic store with the same sample shape and 40 oldest-first
batches of 100, checkpointing between batches, the largest WAL observed after
a batch was 70,072 bytes; the database remained 983,040 bytes. The event and
page cap in the first probe were not included in this second store. This is
not a worst-case bound: real rows, indexes, readers, other writers and storage
errors can change the result, and checkpointing each batch has a cost. Run the
opt-in probe with `CMP_METRICS_SQLITE_EVICTION_PROBE=1 go test -race -count=1
-run '^TestBoundedEvictionCheckpointProbe$' -v ./internal/metrics`.

### Managed Metrics Store Foundation

Linux development container, branch `spike/docker-metrics-storage`, based on
`0d26687` with changes uncommitted during these checks. A separate managed
store now owns a SQLite connection with exclusive locking, serializes its
read/write/maintenance calls and closes after in-flight work. The caller must
schedule maintenance, including when idle, and supply the storage limit.
Maintenance expires samples, events and states strictly older than one hour,
checkpoints the WAL, and calculates active logical usage by subtracting
reusable database pages from the lengths of the database and sidecars. It
attempts at most one 128-sample oldest-first pressure eviction per call. The
eviction summary and a pressure gate commit atomically with deletion. A
subsequent attempt is blocked if active usage has not decreased; the gate
survives reopening. If total file length reaches the limit, it pauses rather
than treating reusable pages as returned filesystem space. Status reports the
last pause and cumulative eviction record; writes that commit before subsequent
maintenance fails return a distinct committed-write error. Commit failures
report an unknown write outcome rather than promising a rollback; callers must
reconcile before retrying.

Focused Go race tests exercise retention boundaries for all three record types,
ordering and recovery under synthetic pressure, no-progress behavior across
reopen, transactional gate and deletion failures, cancellation, concurrent
writes, shutdown, version-four migration, and a competing writer in a separate
process. SQLite `max_page_count` simulates a full database and subsequent
retry; it is not a physical filesystem-full test.
Deferred-constraint failures test the unknown-outcome error and subsequent
maintenance for sample, event and state commits. An in-process latch now keeps
maintenance and subsequent writes paused until the caller acknowledges that
the uncertain batch was reconciled. At that revision the latch was not persisted
across restart.
`go test -race -count=1 ./internal/metrics ./internal/runtime`,
`PLAYWRIGHT_PORT=18334 make check QUIET=1`, `make docs` and `make check-docs`
passed.

This path is not yet connected to app startup, settings, a scheduler, collector
or user-facing status. It is not a hard quota, does not verify physical disk
availability, and its WAL peaks, checkpoint overhead, crash behavior and
high-frequency multi-container throughput are not established. The cumulative
eviction summary is not a precise per-container gap or a user warning. Normal
expiry and transactions can temporarily grow the WAL beyond the limit. The
underlying raw store remains an unwired primitive, not a bypass for a future
collector. LIVE-003 remains partially implemented and unverified.

### Metrics Settings And Maintenance Scheduling

Linux development container, branch `spike/docker-metrics-storage`, based on
`64580e4` with changes uncommitted during these checks. Application schema
version 4 adds an initially empty metrics-settings table; its absence of a row
means disabled and unconfigured. Settings enforce the approved preset scopes
and intervals, require an explicit storage budget, and reject stale updates.
Saving settings does not start collection or alter metrics history. Tests cover
version-3 migration, preservation of discovery data, default-off, reopen,
concurrent update rejection and validation.

An unwired maintenance loop immediately runs the managed store's maintenance,
reports every attempt, and retries on a caller-supplied interval until
cancellation. A SQLite integration test verifies idle one-hour expiry; a
deterministic test exercises a failed attempt, recovery and shutdown. It does
not start automatically and is not the Docker collector. The settings UI,
app lifecycle integration, freshness and gap reporting remain unimplemented.
`go test -race -count=1 -run
'TestMaintenance|TestScheduledMaintenance|TestSettings|TestConcurrentSettings|TestOpenDatabaseMigrates'
./internal/metrics ./internal/auth`, `PLAYWRIGHT_PORT=18334 make check QUIET=1`,
`make docs` and `make check-docs` passed.

### Read-Only Container Sampling

Linux development container, branch `spike/docker-metrics-storage`, based on
`c4bfd86` with changes uncommitted during these checks. A read-only Docker
inspection request validates full container identity and returns discrete
state, health and start time. A missing Docker config object yields unknown
health rather than an assumed absent health check. A single-owner sampler inspects before and after each
running stats request and discards its baseline on a failed or changed
observation. It does not infer zero for stopped containers. Rates require an
explicit continuity assertion from its caller as well as a matching nonzero
start time; matching start times alone cannot prove no intervening pause or
event-stream gap.

The focused race tests exercise healthy/unknown/no-health-check states,
malformed responses and IDs, stopped containers, failed requests, reset and
instance change. An integration test uses a mock read-only Unix Docker socket,
the real runtime client and the managed SQLite store to persist and retrieve
state and samples. It does not implement scope selection, event-stream
reconciliation, app startup, settings UI, collector freshness or user warnings.
No live Docker test or throughput measurement was performed.
`go test -race -count=1 ./internal/runtime ./internal/metrics`,
`PLAYWRIGHT_PORT=18334 make check QUIET=1`, `make docs` and `make check-docs`
passed.

### Durable Managed Write Guard

Linux development container, branch `spike/docker-metrics-storage`, based on
`c4d92e4` with changes uncommitted during these checks. Schema version 6 adds
a pending-write marker. The managed store records the batch kind and start
time before each write, and marks the pending write committed atomically with
its samples, events or states. A surviving marker pauses maintenance and writes
after reopening until explicitly acknowledged. Tests cover a prepared marker,
a committed marker, real deferred-constraint commit failures for all three
batch kinds, a committed event followed by failed marker cleanup, and the
version-five migration. These use close and reopen and injected SQLite failures,
not power-loss testing or a physical full filesystem.

The marker is not a batch payload or replay identity. It cannot alone recover
events, deduplicate a replay, or identify an interrupted batch after the marker
has already been cleared. The collector, event continuity and recovery policy
remain unimplemented. This adds extra SQLite transactions; throughput at the
target sampling rates has not been measured. Collection remains off.

`go test -race -count=1 ./internal/metrics ./internal/runtime`,
`PLAYWRIGHT_PORT=18334 make check QUIET=1`, `make docs` and `make check-docs`
passed.

### Metrics Preference Settings Page

Linux development container, branch `spike/docker-metrics-storage`, based on
`34f3dd8` with changes uncommitted during verification. The authenticated
Settings page saves preset or custom metrics preferences and an explicit byte
target, using revision checks to reject stale saves. It cannot turn collection
on and states that collection is unavailable. The page does not create
`metrics.db` or start Docker collection.

HTTP tests cover anonymous and forced-password-change gates, CSRF, forged enable
fields, stale saves, validation, preset and custom values, and database reopen.
Desktop and mobile browser coverage exercises the form without JavaScript,
keyboard submission, touch focus, persisted values and horizontal overflow.
The wizard, enable/disable control, running collector, pressure warning and
history remain unimplemented.

`go test -race -count=1 ./internal/web ./cmd/server`,
`PLAYWRIGHT_PORT=18334 make check QUIET=1`, `make docs` and `make check-docs`
passed. The first targeted browser attempt timed out while the web-server Go
build was still starting; subsequent targeted and full browser runs passed.

## Inventory Column Customizer

2026-11-22, Linux development container, branch `spike/docker-metrics-storage`,
based on `2480184` with the column customizer changes uncommitted during these
checks. The Projects view and the shared Containers list panel expose the same
browser-local column controls. The identifying column stays visible; without
JavaScript, all columns remain visible. This does not implement metrics or logs.

- `go test -race ./internal/web ./tests/ui-fixture`: passed.
- `PLAYWRIGHT_PORT=18334 npx playwright test tests/e2e/components.spec.js tests/e2e/home.spec.js --project=chromium --project=mobile-chromium`:
  passed 23 tests with one intentional bootstrap skip before the final test
  additions.
- `PLAYWRIGHT_PORT=18334 make check QUIET=1`: passed Go race tests, vet and
  build, Node, HTTP and desktop/mobile browser tests, asset verification and
  contract structure checks. Browser tests exercise the populated shared list
  fixture, real project rows, keyboard, simulated touch, reload, restore,
  blocked browser storage, htmx reinitialization and no-JavaScript behavior.
- `make docs`, `make check-docs` and `git diff --check`: passed.

Real container rows were not exercised against a Docker daemon for this UI
change; the populated shared list fixture covers the same table component.
At `193500f`, a review found that replacing the customizer's options inside
an htmx swap left event handlers referring to the removed controls. The fix
uses the current controls and preserves the in-page selection when browser
storage is unavailable. Focused Chromium and mobile Chromium tests cover the
partial swap; `PLAYWRIGHT_PORT=18334 make check QUIET=1` passed with the fix.

## Opt-In Metrics Collector

2026-11-23, Linux development container, branch `spike/docker-metrics-storage`,
based on `802b209` with implementation changes uncommitted during verification.
The authenticated Settings page now controls a persisted opt-in collector.
Disabled mode does not open `metrics.db` or access Docker. Enabled collection
uses the one-hour managed SQLite store, targets the selected interval, caps
concurrent per-container observations at eight, saves discrete states/samples,
and observes lifecycle/health events from a reconnecting Docker event stream.
Activated scope resolves Compose project names and matches Docker project
labels, including shared names; leaving scope retains history but drops rate
baselines. Rates remain unavailable unless event continuity and container
identity are stable. Identical event replay keys are deduplicated and their
identities expire atomically with the one-hour event records.

Pending write markers carry a committed bit atomically with batch records. On
reopen, the collector reconciles and clears a marker without replaying the
batch. The Settings page reports collector health and cumulative sample
evictions with timestamp bounds that are not claimed to represent a continuous
gap. Because default-off mode does not reopen the metrics store, a prior
eviction warning is not rediscovered after a restart until collection is
enabled.

Temporary SQLite and mock Unix Docker sockets cover opt-out/no database
creation, collector start/stop on settings changes, shared-name scope,
sample/state/event persistence, duplicate event replay, rate continuity,
eviction status and transactional recovery. Authenticated HTTP tests cover
session/CSRF gates, validation, stale settings and refusing opt-in without a
configured Docker socket. Browser tests cover profile-to-Custom transitions,
preset resets, storage-limit edits, keyboard/touch use and mobile overflow.

- `go test -race -count=1 ./internal/metrics ./internal/runtime ./internal/web ./cmd/server`:
  passed.
- `PLAYWRIGHT_PORT=18334 npx playwright test tests/e2e/home.spec.js --project=chromium --project=mobile-chromium`:
  passed 9 tests with one intentional mobile bootstrap skip.
- `PLAYWRIGHT_PORT=18334 make check QUIET=1`: passed Go race tests, vet, build,
  Node, HTTP and desktop/mobile browser tests, asset verification and contract
  structure checks.
- `make docs`, `make check-docs` and `git diff --check`: passed.

No live Docker daemon, throughput, physical disk-full, abrupt power-loss,
Windows/macOS or hosted CI verification was performed. The approved zero
inference from a known stopped state remains unimplemented pending metric and
time-boundary rules. The overview chart slice below does not add a per-container
gap ledger, dedicated Containers observability view, timeline event markers/list,
historical logs, preset sizing recommendations or TimescaleDB. SQLite deletion
does not impose a hard physical disk quota.

## Dashboard Charts And Shared Time Selection

2026-11-23, Linux development container. The authenticated overview displays
per-container CPU percentage, Docker raw memory usage, and network receive and
transmit rates from the active managed SQLite store. It selects the latest
recorded observation in each 30-second display bucket (at most 120 points per
container across the last hour), preserves missing buckets as gaps, and does not
combine values across containers or change retained samples. Dashboard project
content is a summary linking to the detailed Projects view.

The reusable SVG chart shows hover values, a keyboard-operable data table and
shared selected-time cursors. Clicking or using the arrow keys emits
`observability:timechange` with `detail.time` as Unix milliseconds; the overview
charts listen to the same event. This supports a future timeline consumer, but
does not implement a timeline or claim that clicking adjusts one that is not
present.

- Focused storage, collector, dashboard serialization, discovery and runtime
  handler tests passed.
- `PLAYWRIGHT_PORT=18334 make check QUIET=1`: passed Go race tests, vet, build,
  Node, HTTP, desktop/Firefox/WebKit/mobile browser tests, asset verification and
  contract structure checks.
- `make docs`, `make check-docs` and `git diff --check`: passed.

No live Docker-backed dashboard collection or platform-specific verification
was performed. Collection remains opt-in; disabled and unavailable states were
covered without opening the metrics database from dashboard reads.

## Aggregate Overview Dashboard Presentation

2026-11-23, Linux development container, branch
`feat/dashboard-aggregate-overview`. Overview now presents aggregate CPU,
Docker raw memory and combined receive/transmit throughput from existing opt-in
container samples. Per-container chart lines remain on Observability. An
aggregate bucket is shown only when every returned container series has a value
for that metric; otherwise it remains a gap. The UI labels these as
collected-container totals and explicitly says host-level resources are not
collected. Collection, retained samples and the one-hour range are unchanged.

- `go test ./internal/web`: passed aggregate sum/gap, degraded-history and
  per-container Observability regressions.
- `npm run test:e2e -- --grep='first login forces password setup before protected dashboard and works on mobile'`:
  passed in Chromium; Firefox, WebKit and mobile Chromium skip this
  bootstrap-only flow by design.
- `make check QUIET=1`: passed Go race tests, vet, build, Node and HTTP tests,
  the complete browser suite, asset verification and contract structure checks.
- `make docs && make check-docs`: passed public/opt-in Hugo variants and local
  link validation.
- Browser layout assertions cover 2560×1440 and narrower desktop widths from
  1440 through 620 pixels, checking for horizontal overflow and visible charts.

No host-level data source or live Docker collection was added or verified.
Aggregate totals are derived from independently recorded container samples and
are not simultaneous host measurements.

## Fixed Desktop Shell And Independent Scrolling

2026-11-23, Linux development container, branch
`feat/dashboard-aggregate-overview`, based on `3362254`. The desktop shell keeps
navigation and side panels in place while the main pane scrolls. Scrollable
sidebar contents and Settings navigation retain access at short viewport
heights; panel collapse and reopen controls remain available. At 900 CSS pixels
and below, the existing compact layout retains document scrolling.

- `PATH=/config/.local/bin:$PATH make check QUIET=1`: passed Go race tests,
  vet, build, Node, HTTP, browser, asset and contract checks.
- `PATH=/config/.local/bin:$PATH make docs`, `make check-docs`, and
  `make check-security`: passed documentation builds/link checks and npm/Go
  vulnerability scans.
- The authenticated Chromium flow tests independent pane scrolling, keyboard
  and wheel input, Settings navigation and collapse/reopen. The full browser
  suite includes compact-layout touch interactions; its duplicate authenticated
  bootstrap flow is intentionally skipped in other browser projects.

The requested right-side project/container details panel is a future direction;
it and Compose actions were not implemented or verified in this slice. Native
mobile browsers and the development-image rebuild were not tested.

## Metrics Settings And Large Chart Usability

2026-11-23, Linux development container, branch `feat/settings-metrics-ui`,
based on `6f6f49e` with changes uncommitted during verification. The Settings
page now separates category navigation from the selected content, presents
Low/Medium/High/Custom as accessible profile choices, and only exposes custom
scope and interval controls for Custom. Storage targets accept B, KB, MB and
GB, convert decimal units exactly to whole bytes, and retain legacy byte-only
form submissions. Initial setup defaults to MB, with the unit selector
embedded at the trailing edge of the numeric input. If JavaScript is
unavailable, the server still changes the profile to Custom when the saved
storage target changes; CSS reveals Custom controls when Custom is selected.

Dashboard chart series controls are collapsed by default and bounded when
opened. Every line remains available and can be individually hidden; chart
geometry remains fixed, hover values include only visible lines, and touch
selection can show a tooltip that is dismissed by tapping elsewhere.

- `go test ./internal/web -run 'TestMetricsSettingsAuthenticatedConfiguration|TestMetricsStorageLimitUnits|TestFormatStorageLimitRoundTrips' -count=1`:
  passed.
- `npx playwright test --project=chromium --project=mobile-chromium tests/e2e/components.spec.js tests/e2e/home.spec.js --grep "renders chart series|keeps large chart legends|first login forces password setup"`:
  passed desktop and mobile chart cases and the desktop settings flow; the
  bootstrap test intentionally skips its duplicate mobile run.
- `make check QUIET=1`: passed Go race tests, vet, build, Node, HTTP, complete
  desktop/Firefox/WebKit/mobile browser tests, asset verification and contract
  checks.
- `make docs && make check-docs`: passed.
- `make check-security`: passed npm audit and govulncheck with no findings.

The 40-series case uses the component fixture; it is not evidence from a live
Docker workload. The storage-write issue reported earlier had resolved before
this change and was not reproduced, so no root cause is claimed. Live Docker,
physical disk-full behavior and Windows/macOS remain unverified.

## Authenticated One-Hour Observability Timeline

2026-11-23, Linux development container, branch `feat/observability-timeline`,
based on `6ee8015` with implementation changes uncommitted during verification.
The authenticated Containers observability view uses the same last-hour range
as the charts, presents observed lifecycle events and synchronizes the slider,
markers, event list and chart cursors. The event query is bounded; the UI
displays the newest 200 events and identifies when older events were omitted.
The route reads stored history without requesting a live Docker snapshot, and
the UI does not claim that Docker event history is complete.

- `go test ./internal/metrics ./internal/web`: passed.
- `npm run test:e2e -- --grep='timeline markers and keyboard selection|authenticated observability route'`:
  passed the timeline interaction in Chromium, Firefox, WebKit and mobile
  Chromium.
- `npm run test:e2e -- --grep='first login forces password setup before protected dashboard and works on mobile'`:
  passed the authenticated route flow in Chromium; the other three projects
  were intentionally skipped by the test's single-bootstrap-account guard.
- `make check QUIET=1`: passed Go race tests, vet, build, Node, HTTP, complete
  desktop/Firefox/WebKit/mobile browser tests, asset verification and contract
  structure checks.
- `make docs && make check-docs` and `make check-security`: passed, including
  npm audit and govulncheck with no findings.

No live Docker daemon, event-replay completeness, physical disk-full,
platform-specific or historical-log behavior was verified. The event display
limit does not establish a retention policy; the broader LIVE-003 contract
remains partially implemented and unverified.

PR review follow-up keeps event-row selection inside the shared timeline
boundary and preserves exact millisecond timestamps from chart/event selections
while the range slider continues using one-second keyboard steps. The selected
event row and marker expose their current state, and fractional milliseconds
remain visible in the selected-time label. Browser regressions exercise the
rendered authenticated page layout and non-second-aligned chart/event times.
The focused timeline tests passed in Chromium, Firefox, WebKit and mobile
Chromium; the authenticated rendered-page interaction passed in Chromium, with
the other projects intentionally skipped by the single-bootstrap-account guard.

## Automatic Documentation Publishing

2026-11-23, Linux development container, branch `feat/settings-metrics-ui`,
based on `5810f98` with changes uncommitted during verification. The Pages
workflow now listens for completed CI runs and proceeds automatically only
when the run succeeded for a same-repository push to the default branch. The
source job checks that its SHA is still the branch head and that the commit
changes Hugo documentation (or contracts when contract publication is enabled)
before the build/deploy jobs run. `PUBLISH_DOCS=true` remains required; manual
publication remains available with its explicit confirmation.

- `npm run test:unit`: passed, including a workflow-policy regression.
- `make check-workflows`: passed YAML/actionlint and workflow policy checks.
- `gh variable list`: confirmed `PUBLISH_DOCS=true`.
- `make docs && make check-docs`: passed.

The GitHub Actions workflow itself has not yet run on the updated default
branch. A successful CI-triggered Pages deployment therefore remains to be
confirmed after merge.
