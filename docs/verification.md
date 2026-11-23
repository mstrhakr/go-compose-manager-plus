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

These are not a scheduled one-hour retention implementation or a budget
controller. Logical deletion can leave allocated database pages behind; file
lengths are not atomic snapshots or filesystem block counts. Filesystem-full
recovery, earliest-first pressure eviction, warning and write pause/retry,
collector startup and shutdown, and Docker integration remain unverified.

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