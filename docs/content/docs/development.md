+++
title = 'Development'
url = '/development/'
weight = 30
+++

## Daily commands

<p>Install Go 1.27.4, Node 24, GNU Make, and a C compiler (Go race tests use CGO).
On Windows, use an appropriate Make/GCC toolchain or WSL for the local full suite;
native Windows behavior is separately checked by CI. Docs builds need extended
Hugo 0.169.0; Hugo resolves the pinned theme as a Go module. Docker is optional
until working on container surfaces.</p>
<pre><code>make setup       # once: locked npm tools and Playwright browsers
make test        # all Go, JS/unit/API, and desktop/mobile browser tests
make check       # above plus vet, formatting, build, assets, contracts
make fmt         # format Go code
make test-go      # focused race-enabled Go checks
make test-js      # Node unit tests and browserless HTTP tests
make test-e2e     # Chromium, Firefox, WebKit, mobile Chromium
make check-workflows # workflow lint and token/pin policy
make check-security  # npm audit and Go vulnerability analysis
make check-container # Docker build and vulnerability scan
make docs         # default docs build
make docs-contracts # opt-in preview including contracts
make docs-development # opt-in preview including contributor docs
make check-docs   # test public and opt-in site variants</code></pre>
<p>Linux browser system dependencies can be installed with
<code>npm exec -- playwright install --with-deps chromium firefox webkit</code>.
Run privileged OS installation yourself when your environment requires it.
VS Code's default Test task runs <code>make test</code>. No separate test.sh is
needed: a wrapper would only delegate to Make, never maintain a second test list.</p>
<p>Playwright starts/stops a dedicated Go server at 127.0.0.1:18080 and refuses
to reuse an unrelated existing listener. Two workers bound memory use; tests
do not retry failures. Reports and failure traces are local ignored artifacts.
Node unit tests use the built-in runner; HTTP and browser tests use Playwright.</p>
<p>For populated dashboard screenshots without a Docker connection, run
<code>npx playwright test tests/e2e/dashboard-preview.spec.js --project=chromium</code>.
This test renders the real dashboard templates and aggregate-chart builders
using clearly labeled synthetic container observations for the current last
hour. The Overview selector switches a single visible chart between CPU,
memory and combined network receive/transmit. It generates new timestamps on
every run, includes sample gaps and a
load spike, and writes light/dark 2560×1440 screenshots under
<code>test-results/dashboard-synthetic-*.png</code>. Preview data uses temporary
test state only; it does not seed the running application's database or enable
production collection.</p>
<p>Go tests use the standard testing and httptest packages. Add table-driven
configuration tests and handler tests near the implementation. For an individual
package, run <code>go test -race ./internal/web</code>.</p>
<p>For shorter interactive output, use <code>make check QUIET=1</code>.
Each check announces its start and completion; output is captured and printed
in full if that check fails. The default remains unchanged. This reduces
visible logs, not test runtime. For GitHub progress, <code>make pr-checks</code>
watches the current branch's PR, or use <code>make pr-checks PR=9</code>.
<code>make run-watch RUN=123</code> follows a specific Actions run with
<code>gh run watch --compact --exit-status</code>. These targets need an
authenticated GitHub CLI; PR watch output is the standard <code>gh</code> output
and may repeat status lines.</p>
<p>For an opt-in live Docker snapshot check, run
<code>CMP_LIVE_DOCKER=1 CMP_LIVE_ACK=/var/run/docker.sock make test-live-docker</code>,
replacing the acknowledgement with the exact socket path. It creates a
uniquely named disposable Compose container with no host mounts, ports, networks
or named volumes, then removes only a verified fixture container and its
anonymous volumes. It requires
the development image to be present locally; it does not pull, build or prune.
It is not part of <code>make check</code> and a crash can leave a fixture behind.
Use a disposable daemon when stronger isolation is needed.</p>
<p>The test-only metrics spike can stream read-only stats for one explicitly
selected running container for ten seconds, writing only to a temporary
tstorage directory. It does not create, start or stop containers:</p>
<pre><code>CMP_METRICS_SOCKET=/var/run/docker.sock CMP_METRICS_CONTAINER=CONTAINER_NAME \
  go test -race -count=1 -run '^TestLiveStatsStorage$' -v ./internal/metricspoc</code></pre>
<p>Select a daemon and container only with their owner's permission. The socket
grants host-administrative access even though this test uses a read-only API
request. This short single-container test is not a representative resource
benchmark, retention implementation or continuously running collector.</p>
<p>To measure one hour of synthetic one-second samples for 1, 5, 25 and 50
containers without contacting Docker, run:</p>
<pre><code>CMP_METRICS_CAPACITY_TEST=1 go test -race -count=1 \
  -run '^TestSyntheticHourCapacity$' -v ./internal/metricspoc</code></pre>
<p>This writes temporary databases, verifies every series after reopening, and
reports directory sizes at the end of ingestion and after closing. Its generated
values and accelerated writes do not model production workloads or peak disk
usage and must not be used as storage guarantees.</p>
<p>The runtime package also exposes a bounded, on-demand Docker stats snapshot
and conversion of two consecutive observations to CPU and I/O rates. It does
not start a collector. The caller must pass the previous observation only for
the same uninterrupted container instance and discard it after a disconnect.
Memory usage is raw Docker usage, not the CLI's cache-adjusted value. Missing
counters, resets and changing interface/device sets produce unavailable rates
instead of zero; platform support remains unverified.</p>
<p>A read-only Docker container inspection supplies a discrete running/health
state and start time. The sampler brackets each running stats request with
inspections, drops its baseline on errors or changes, and emits no stopped-state
zeroes. The opt-in application collector resolves the configured scope, updates
it on each collection cycle, and limits concurrent observations to eight.
Rates require an uninterrupted event stream and a stable container start time;
disconnects and errors discard rate continuity. Sampling intervals are targets,
not schedule guarantees.</p>
<p>The SQLite metrics persistence foundation stores observations in a
separate <code>metrics.db</code>, preserving unavailable fields as SQL NULL.
Memory values use decimal text to retain the full unsigned Docker counter range.
It persists observed event and state records but does not infer state between
snapshots. The collector reconnects the read-only Docker event stream and
deduplicates exact event replay keys (container, timestamp, kind and health).
Replay identities expire with their one-hour event records, and deduplication
cannot restore events missed during a disconnect. The approved inference of
zero values from specific known states, such as stopped, remains
unimplemented; an absent observation alone is not evidence of zero. A caller
can atomically delete
sample, event and state records before a cutoff, inspect SQLite file lengths,
inspect logical database page counts and reusable pages, attempt a WAL
checkpoint, and remove batches of oldest samples. Reusable pages are still in
the database file; reusing them can require additional WAL space. Each nonempty
sample eviction atomically updates a persistent cumulative count and timestamp
bounds; the bounds do not imply a continuous gap, and evictions made before
schema version 4 cannot be reconstructed. The persisted summary does not
describe event or state loss; the collector surfaces it as a warning when
collection is active.</p>
<p><code>ManagedStore</code> serializes reads, writes and maintenance under an
exclusive SQLite lock. Its caller supplies a byte limit. The collector runs
maintenance immediately and retries every five seconds, including while idle.
Maintenance expires records
older than one hour and attempts bounded oldest-sample eviction at a 90% active
logical-usage target. Its durable pressure gate prevents repeated eviction
when no measurable progress is made, including after reopening. It reports
paused writes and a cumulative eviction summary. A write can succeed before subsequent maintenance
fails; callers must distinguish <code>ErrWriteCommitted</code> and not retry
that committed batch. A failed commit returns
<code>ErrWriteOutcomeUnknown</code>; reconcile the batch before retrying it.
The managed store records a durable pending-write marker before each batch and
marks it committed in the same transaction as the data. A surviving marker
pauses maintenance and writes across reopening. The collector inspects the
atomic committed bit and clears the marker without replaying the interrupted
batch. The marker records the batch kind and start time, not the payload or a
replay identity. Event replay deduplication is separate; neither mechanism
reconstructs gaps. The implementation checkpoints frequently and its cost at
target sampling rates is unmeasured.</p>
<p>The authenticated Settings page can save Low, Medium, High or Custom metrics
preferences and an explicit byte target, and collection is default-off until
explicitly enabled. Enabling requires a configured Docker socket. Status shows
the latest cycle, event-stream state and failures. Once the collector has read
eviction history, Settings warns when storage pressure has removed samples,
including the cumulative count and timestamp bounds (not a continuous-gap
claim). To preserve default-off behavior, a disabled collector does not reopen
the metrics database on startup, so it cannot rediscover this warning until
collection is enabled. Preset size recommendations and the setup wizard remain
unimplemented.</p>
<p>This is not a hard disk quota: SQLite deletion does not itself shrink the
database, file lengths are not allocated blocks or peak usage, WAL and
transactions can exceed the target, and physical filesystem-full recovery is
unverified. If file length itself reaches the limit, maintenance pauses rather
than assuming row deletion will reclaim filesystem space. An eviction that
does not reduce active usage also pauses until accounting conditions change or
the configured limit increases. The collector retries maintenance every five
seconds; failed writes pause metrics collection but leave the rest of the app
usable.</p>
<p>Metrics settings are persisted separately in application state. An
unconfigured installation is disabled, and the storage limit must be explicitly
provided; no preset storage defaults are assumed. The authenticated Settings
page exposes enable/disable, the low, medium and high profile scopes and
intervals, Custom controls, and collection status. Settings use revision
checks to reject stale updates. Saving settings while disabled does not open
the metrics database or access Docker. The collector uses the persisted
opt-in after restart and closes on disable or application shutdown. The setup
wizard and historical logs remain unimplemented. The Containers observability
view shows per-container charts for the last hour, a shared time slider, observed
lifecycle markers and a newest-first event list capped at 200 entries. Chart,
slider, marker and list selection move the same selected-time cursor; the view
does not claim Docker event history is complete. It uses stored events without
requesting a live Docker snapshot. The overview dashboard reads the opted-in
SQLite history and displays per-container CPU, Docker raw memory usage and
network receive/transmit rates for the last hour. It selects the latest stored
sample per 30-second display bucket without combining containers or changing
retained samples; missing buckets stay gaps. Disabled and unavailable states
are explicit.</p>
<p>To measure one synthetic hour of SQLite samples for 1, 5, 25 and 50
containers without Docker, run:</p>
<pre><code>CMP_METRICS_SQLITE_CAPACITY_TEST=1 go test -count=1 \
  -run '^TestSyntheticHourSQLiteCapacity$' -v ./internal/metrics</code></pre>
<p>The test batches a minute of generated observations per transaction, checks
every observation after reopening, and reports file lengths at the end of
ingestion and after closing. These are not peak sizes or a realtime benchmark;
they do not establish storage limits or disk-full recovery.</p>
<p>To measure checkpointed sample-eviction WAL sizes in a synthetic local store,
run:</p>
<pre><code>CMP_METRICS_SQLITE_EVICTION_PROBE=1 go test -race -count=1 \
  -run '^TestBoundedEvictionCheckpointProbe$' -v ./internal/metrics</code></pre>
<p>The observed sizes are not worst-case bounds or safe budget defaults.</p>

## UI components

<p>Reusable server-rendered metric, chart and list panels are defined in
<code>internal/web/templates/partials/components.html</code>, with their Go
view types in <code>internal/web/components.go</code> and shared styles in
<code>internal/web/static/foundation.css</code>. Use unique, stable heading IDs
for each panel. Prefer the existing panels over copying page-specific markup.
The dashboard uses unavailable states until real data is available; sample data
exists only in <code>tests/ui-fixture/</code>, not an application route.</p>
<p>The chart's optional <code>Series</code> is a JSON string supplied by the
server, automatically escaped by the template. Serialize it with
<code>encoding/json</code> rather than concatenating input. Its object has finite numeric
<code>min</code> and <code>max</code> values, optional text <code>xLabel</code>
and <code>yLabel</code>, and a nonempty <code>series</code> array. Each series
has a label, a color from <code>blue</code>, <code>amber</code>,
<code>purple</code>, <code>green</code>, or <code>red</code>, and points with
strictly increasing numeric <code>x</code> and numeric or null <code>y</code>.
Values must lie within the shared range; null separates lines. Callers must
choose consistent units, meaningful labels, and an appropriate range. The
component does not fetch or aggregate data; chart freshness and authority remain
the caller's responsibility. Optional <code>xMin</code>, <code>xMax</code> and
<code>selectedTime</code> are numeric milliseconds when using timestamps. Charts
listen for <code>observability:timechange</code> events with
<code>detail.time</code> as a Unix-millisecond timestamp; chart clicks and
keyboard selection dispatch that event, allowing a timeline to synchronize
without coupling the component to one. Hover shows nearby series values, and
arrow keys move between recorded timestamps. The chart includes a data table,
an empty state and an explicit invalid-data error. With JavaScript disabled it
reports that chart data cannot be displayed.</p>
<p>List panels render semantic tables and optional native checkboxes; enable
<code>Selectable</code> only when a consumer needs selection. Local enhancement
provides a scoped select-all checkbox, count and a bubbling
<code>list:selectionchange</code> event whose <code>detail.selected</code> contains
the selected values. Disabled rows are excluded. Selection is ephemeral and
does not authorize or perform any operation. After htmx swaps, controls are
reinitialized from the new DOM; consumers own persistence, data refresh and
action policy. Inventory filters and ordering are server-rendered GET forms;
applying them requests a fresh snapshot and uses counts before filtering.
View navigation uses ordinary links and disabled buttons rather than an ARIA
tab widget; add real destinations only when their behavior is implemented.
With no JavaScript the individual native checkboxes remain usable.</p>
<p>Inventory column preferences use the reusable column customizer and are
stored independently per view in the browser. The first identifying column
stays visible. If browser storage is unavailable, the current page can still
change columns and reports that preferences could not be saved. Without
JavaScript, all columns remain visible and the customizer is hidden.</p>
<p>Run <code>npx playwright test tests/e2e/components.spec.js --project=chromium</code>
to render the actual templates in a test-only page, with populated, empty,
invalid and selectable examples. Browser screenshots are local test artifacts;
the fixture is not served by the application.</p>

## Dependencies and updates

<p>The auth slice uses third-party Go modules for SQLite, password hashing,
server-side sessions and CSRF protection. Use <code>go get module@version</code>
and <code>go mod tidy</code> when adding dependencies, and commit both module
files. Docker copies both automatically.</p>
<p>Dependabot checks Go modules/toolchain, npm, GitHub Actions references in
workflows, and Dockerfile base images. Weekly version updates group minor and
patch updates by ecosystem; majors get individual PRs. Security updates use
separate minor/patch groups and individual majors; their timing is independent
of the weekly version-update schedule. Groups do not cross ecosystems. Review
updates rather than automatically merging them. Go starts at 1.27.4 and
Hugo at 0.169.0, the latest stable releases checked for this first pass.</p>
<p>When updating Go, align <code>go.mod</code> and the Dockerfile tag/digest.
When updating Hugo, update the version and official release SHA-256 in
<code>.github/actions/setup-hugo/action.yml</code>; Hugo's binary is not managed
by Dependabot. Install that same Hugo version locally from the official release,
checking its checksum before running it.</p>
<p>htmx is pinned in package-lock.json and committed under internal/web/static
with its license and SHA-256 metadata. After an intentional package update, run
<code>npm run assets</code>, review the generated changes, then run
<code>make check-assets</code>. A Dependabot htmx PR updates npm metadata, not
these committed assets: CI will fail until a maintainer updates the assets in
the PR. Never execute PR code in a privileged workflow to do that automatically.
Builds do not download browser code from a CDN.
Node is not required to run an already-built executable or container.</p>
<p>Dependabot does not monitor every version embedded in arbitrary shell or JS:
GoReleaser and Hugo release URLs/checksums, govulncheck and actionlint invocations,
Buildx versions, and the Trivy image tag/digest in the scanner script need manual
version checks and reviewed updates. Node 24, Go versions in prose and other
duplicate pins also need coordination; a Docker or go.mod update does not rewrite
them all. Check upstream releases and independently verify downloaded checksums.
Direct Action pins are not a guarantee about transitive Actions or downloads.</p>

## Documentation

<p>Content lives in <code>docs/content/</code>. Architecture, development and
deployment pages are excluded from the default public site and enabled by
<code>hugo.development.toml</code>; contracts are independently opt-in.
Add any new contributor-only page to the exclusions in <code>hugo.toml</code>
before adding it to the content directory.
<code>make check-docs</code> builds and checks all four combinations in temporary
directories. Existing pages use HTML with TOML
front matter; Hugo also supports Markdown pages. Add a title and weight to include
a page in navigation. Layouts and CSS are local, avoiding theme/submodule drift.
CI builds the site with a project subpath to check GitHub Pages compatibility.</p>

## Continuous integration

<p>Protect main; use short-lived branches and squash PRs with Conventional Commit
titles. No permanent dev branch, mandatory second reviewer, or local commit hook.
The optional VS Code extension assists authoring; CI checks the PR title and the
squash defaults use it. Maintainers must preserve that checked message on merge.</p>
<p>CI detects changed surfaces and runs only relevant read-only jobs. Unknown
paths/shared tooling run broadly; failed detection or cancelled/failed checks
fail the final CI gate. Contract-only edits skip app jobs unless mixed with code.
When contract publication is enabled, those edits also validate the docs build.</p>
<p>Native Go CI covers Linux x64, Windows x64, macOS x64 and macOS ARM64.
Cross-compiling releases is not native execution evidence. Browser tests on
Linux exercise Chromium, Firefox and WebKit; WebKit is not a native Safari run,
and mobile emulation is not a physical-device test.</p>
<p>PR jobs never receive publishing secrets or write scopes. Privileged events
do not execute PR source; no public-PR self-hosted runners or cross-trust caches.
Dependency scans run on relevant changes and a schedule. Protected-main CodeQL
has security-report write scope, not PR access or package publication rights.</p>
<p>See the contract guide before behavioral changes. Structure checks do not
approve a requirement; report actual executed checks and remaining gaps.</p>
