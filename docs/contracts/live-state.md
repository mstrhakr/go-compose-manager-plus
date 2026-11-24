+++
title = 'Dashboard And Live State'
weight = 8
+++
# LIVE: Dashboard And Live State

**Status:** Draft / prototype scope and SSE preference recorded; detailed live
state and recovery policies proposed; opt-in metrics direction recorded

**Baseline:** 2026-11-19, product `68ab9d2`, documentation-only planning. Plugin
runtime-plan passages and metrics-publisher boundaries were inspected, not
adopted wholesale or reproduced on an Unraid host.

## Purpose And Scope

Provide a dashboard home with live project/container/operation status and resource
visibility within an agreed measurement budget. UniFi is the owner's design and
interaction inspiration, not a request to copy assets or architecture. The owner
has selected per-container metrics history as the next product direction; its
detailed retention and resource policies still require decisions.

## Ownership

LIVE owns accepted presentation/read-model freshness and browser delivery, not
identity, action permission, execution results or Docker truth. Keep runtime
observations, operation progress/outcomes and metric samples distinct. A metrics
tick is not operation completion. These are proposed portable principles from the
plugin runtime plan, not already-verified standalone mechanisms.

## Requirements

### LIVE-001: Live Status Dashboard For The Local Prototype

The prototype has an operational dashboard home and timely live updates across
status and operation views. SSE is the owner's preferred browser transport;
heartbeat, reconnect, fallback-polling and visibility policies remain to refine,
not fixed by that preference.

**Approval:** Owner-requested prototype scope and transport preference,
2026-11-19. No specific interval, metrics set, protocol or freshness SLA approved.

**Source provenance:** Source UI-RUNTIME/UI-METRICS candidates and
`docs/ui-runtime-plan.md` distinguish runtime reconciliation, metrics transport
and operation receipts. The plugin uses Nchan WebSocket transport; SSE is a
standalone preference, not unchanged source behavior.

**Enforcement:** Not implemented. No collector, event subscriber or SSE endpoint
exists in the scaffold.

### LIVE-002: On-Demand Read-Only Runtime Snapshot

On explicit request, show the current Docker container states for each discovered
project whose Compose inputs can be resolved. Associate containers by the
resolved native Compose project name and the `com.docker.compose.project` label.
This association also applies when multiple project folders resolve to the same
name: it is a view of that Docker project, not proof that a folder created or
owns its resources. Discovery activation is not needed to inspect status.

Show an empty result as no matching containers, not as proof that a project was
never deployed. A disconnected daemon, failed request or unresolved project
identity is unavailable or unknown, not an empty or stopped result. The
snapshot changes no containers or project files and does not authorize actions.
Continuous updates, metrics, identity adoption and lifecycle targeting remain
separate decisions.

Opening the distinct containers page requests a read-only snapshot of all
containers returned by the daemon, including stopped containers and those
without a Compose project label. Opening the distinct projects page requests
the same kind of snapshot and shows discovered folders with expandable
project details. Both show snapshot freshness and distinguish a failed or
unconfigured daemon from an empty result. The overview retains its explicit
refresh behavior; these pages introduce no background collection or operations.
The inventory sidebar offers search, name ordering and facets based on the
displayed discovery or snapshot attributes. Applying or clearing filters
requests a fresh snapshot; facet counts are calculated before filtering.
The owner approved an initial navigation shell with separate Overview, Projects
and Containers entries and disabled future Resources, Activity and Settings
entries. Contextual subview navigation is present even with one available view;
future subviews are disabled and do not authorize their capabilities.

**Approval:** Owner approved the bounded read-only status slice and clarified
on 2026-11-22 that same-name copies should use the Compose runtime name without
special adoption handling for this view. Activation remains a separate
management permission, not evidence of filesystem ownership.
The owner subsequently requested distinct all-container and expandable project
views as a reusable UI foundation; this does not approve lifecycle actions.

**Source provenance:** Compose Specification `02-model.md` at `3a4a699`
defines project-name resource grouping and its label. Source STK-ID distinguishes
runtime names and evidence, but its saved-pin and migration policies are not
adopted. See [RES-001](project-resolution.md#res-001-accepted-runtime-identity-is-not-a-display-name).

**Enforcement:** The opt-in snapshot is implemented with explicit refresh;
verification and environment limits are recorded separately.

### LIVE-003: Opt-In Container Metrics And Historical Views

The owner wants per-container Docker statistics and historical context in the
Containers observability view and real metric data in the overview dashboard.
The dashboard should summarize rather than repeat the complete detailed project
list, which belongs on the Projects page. Metrics collection is opt-in: a Docker
socket configuration alone must not enable background metrics collection. The
owner wants a Settings page that lets the user choose whether to collect for
all running containers or only containers associated with activated stacks.
Collection scope is separate from view filters and has disk-use consequences.
When a container leaves collection scope because of deactivation or a settings
change, stop collecting new samples for it but retain existing samples until
normal expiry or storage pressure removes them. For activated-stack collection, associate
containers by resolved Compose project name and Docker project label, as in
LIVE-002. If multiple discovered folders resolve to the same project name,
collect that Docker project's containers when at least one matching folder is
activated. Make the shared identity clear; this association is not evidence that
an activated folder owns the containers.

The owner wants an opt-in setup wizard with Low, Medium, High and Custom
profiles, with the underlying collection and retention options available in
Settings. Each preset configures the whole metrics feature, including its
storage limit; the setup wizard shows an adjustable limit, with suggested
defaults informed by measurement. Individual changes select Custom. Preset
values and sizing measurements are not yet established. These are
performance-oriented modes, not a claim that every mode is lightweight.
Low and Medium select containers associated with activated stacks;
High selects all running containers. For the initial single-resolution history,
Low targets one sample every 15 seconds, Medium every 5 seconds and High every
1 second, with a one-hour history window for each. These are targets, not a
guarantee that Docker supplies samples on schedule. Actual resource costs are
not yet measured. In this initial
single-resolution version, samples older than one hour may expire without being
downsampled; the UI must clearly describe the one-hour history limit. This
exception does not approve discarding raw data before scheduled downsampling
once longer-term tiers are implemented. Automatic tuning, preset defaults,
confirmation and data-retention effects when changing settings remain open.

For the first collection slice, the owner selected per-container CPU utilization,
memory usage and limit, network receive/transmit byte rates, and block read/write
byte rates. Show running and health-check state, including no health check and
unknown states. Capture observed start, stop, exit, restart, OOM and health
changes as timeline events, and report collector freshness, missing samples,
errors and shortened history. Label the memory usage definition; derive rates
from valid counter deltas. Missing values, counter resets and disconnected
periods are gaps, not zeroes. Docker event replay is limited, so event history
cannot be assumed complete after a disconnect. Metric availability, details of
recovery and per-platform support require verification.

The owner subsequently clarified that a known container state, such as stopped,
can justify zero rather than a missing value. Which metrics qualify and how
observation and disconnection boundaries constrain that inference remain to be
specified; a missing observation alone is not proof of a stopped state.

The initial observability view defaults to the last hour, with a timeline whose
selected time is reflected across the charts, inspired by the owner's reference
view. The owner wants event markers on the timeline and an event list for
context. The owner also wants historical logs alongside relevant usage and other
context at the selected time, so an incident can be investigated in one view.
On 2026-11-23 the owner requested implementing container-log collection using
the same local observability SQLite database with one-hour retention; optional
TimescaleDB support is deferred for longer log and metric history. The owner
also requested an aggregated dashboard log view over that history. This is
approval of the bounded feature/backend/retention direction, not evidence that
logs are already collected. Log-source compatibility, collection consent,
access, synchronization, shared-budget pressure behavior and unavailable
states remain to be settled and verified.

The owner then approved read-only Docker container stdout/stderr as the initial
source and authorized beginning implementation. Preserve the original message
alongside Docker timestamp, receipt timestamp, immutable container ID and stream;
decode transport framing rather than storing raw Docker wire frames. Structured
application fields may later be parsed without replacing the original message;
non-JSON output and parse failures must not be discarded, and stderr alone must
not be interpreted as error severity. Oversized records need explicit limits
and truncation reporting. Do not read arbitrary host/container log files or
change logging drivers. Driver/cache availability, rotation and disconnects
prevent a guarantee of complete history.

Application output can contain sensitive data. The earlier recommendation for
an additional default-off "Collect container logs" checkbox was not approved.
On 2026-11-23 the owner instead requested log settings inside the Metrics
category, with separate Docker stats and Container logs controls using five
discrete positions: Off, Low, Medium, High and Custom. Off represents disabled
collection for that source, not merely hiding its view. Each source's selector
and expanded settings must remain contained together in its own card/panel;
exact card styling, dimensions and disclosure behavior are not yet selected.
This approves the category, per-source profile-control direction and containment,
not log preset values, default/migration behavior or production capture.
The existing Docker stats sampling targets remain unchanged by this presentation
decision.

On 2026-11-23 the owner selected option A for the overarching
Off/Low/Medium/High/Custom Metrics selector: it is a category-wide preset,
not an independent effective-profile gate. Selecting Low, Medium or High sets
both Docker stats and Container logs to that preset, including a previously
Off source. In the reviewed scenario, Docker stats High plus Container logs Off
becomes Low for both when overall Low is saved, enabling log capture. Overall
Off sets both sources Off. Individual source changes afterward make the overall
selection Custom. Preset effects, particularly enabling logs, must be clearly
visible before Save; selecting a control does not itself apply collection changes.
This approval replaces the tentative overall-selector proposal and permits
explicit category-wide log enablement on Save, not automatic enablement through
migration or merely because existing numeric metrics were enabled.
New log defaults and the effects of Off on retained history and other observed
state/event collection remain separate lifecycle decisions.

The existing storage-pressure owner evicts
metric samples only, so production capture requires an explicit shared-budget policy
rather than pretending that sample eviction already bounds log storage.

For later integration, the proposed shared-pressure policy is bounded eviction
of the oldest sample/log records by observation time, preserving current
event/state retention behavior and reporting shortened history separately for
logs and samples. This is not approved yet; dedicated log budgets or prioritizing
log removal are alternatives requiring an owner decision. No early log eviction
or changed sample-eviction ordering is implemented by the foundation.

A possible dedicated logs navigation view with stack, container and network
filters is a follow-up idea, not an approved complete feature set. Network
filtering needs evidence and time-bound association rules; current inventory
does not establish historical network membership.
Chart rendering may reduce displayed points independently of retained raw
resolution. Longer ranges and custom timeframes are future advanced options;
their retention, zoom resolution and display behavior remain to be designed.
Use a reusable preset-selection and customization UI structure suitable for
other feature settings, rather than duplicating an observability-specific
pattern. Future features' actual options and behavior require their own decisions.

The owner selected SQLite for the initial one-hour metrics implementation,
separate from application state, after the tstorage failure probe. A future
optional TimescaleDB backend is a possibility, not an implemented feature or
an approval of its retention limits. Longer SQLite limits and differences
between backends require evidence and a later decision. Netdata-style historical retention and a
UniFi-inspired display are design goals, not approval of a longer-term
retention duration, aggregation, disk limit or visual implementation. The owner wants
the storage budget managed toward approximately 90% usage, reserving roughly
10% as working headroom. For later tiered retention, downsampling should happen
before eligible raw data is removed; configurations known to prevent that
should be rejected. This is a
target rather than a guarantee against storage exhaustion. For the initial
single-resolution version, if the metrics store reaches its limit before normal
expiry, remove the oldest metrics early and clearly warn that history was
shortened. If space cannot be recovered, pause metric writes, clearly report the
problem and retry when space is available; the rest of the app remains usable.
Exact accounting and compatibility validation remain open.
A reusable column
customizer is also requested for inventory views. These directions do not
authorize Compose mutations or imply that unavailable metrics are zero.

**Approval:** Owner selected the opt-in collection boundary, initial preset
sampling targets, first collection scope and the above product and storage
directions on 2026-11-22. Other metrics, collection edge cases, remaining preset behavior,
longer-term retention/downsampling, resource budgets,
recovery and other presentation details remain unresolved outside the bounded
Settings/chart refinement below. The storage dependency's fitness and
operational characteristics still need evaluation.

On 2026-11-23 the owner approved a bounded Settings and dashboard presentation
refinement: settings-category navigation is distinct from the main application
navigation; Low, Medium and High hide Custom-only scope and interval controls,
while Custom reveals them; and the storage target remains available with B, KB,
MB and GB input units. These units use decimal multiples (1 KB = 1,000 bytes)
and persist as an exact whole-byte limit. The unit selector is embedded at the
right end of the storage input and defaults to MB for initial setup. For charts
with many series, retain all series and keep the chart layout bounded, with
individually switchable series. These presentation and input-format decisions
do not change collection, retention or storage-pressure behavior.

**Enforcement:** The reusable column customizer is implemented for the Projects
and Containers inventory views; preferences are browser-local and the
identifying column remains visible. An authenticated Settings page persists
Low, Medium, High and Custom preferences, an explicit storage target and a
default-off collection opt-in. Settings presents category navigation separately
from the content panel; scope and interval controls are visible only for Custom,
while the editable storage target accepts exact decimal B/KB/MB/GB values.
The dashboard's series selector is collapsed and bounded, preserves access to
all lines, and allows individual lines to be hidden without changing chart
geometry. The application collector opens the separate
SQLite store and contacts Docker only while opted in. It samples selected
containers read-only, persists discrete state and metric records, and captures
observed lifecycle/health events. Activated scope uses resolved Compose project
names and Docker project labels; leaving scope drops baselines and keeps history.
The single-process event stream reconnects after disconnects, event fingerprints
make identical replays idempotent and expire with their one-hour event records,
and rate fields remain unavailable unless event-stream continuity and container
identity are stable. Missing observations are not inferred as zero; the approved
inference from a specific known state (such as stopped) remains unimplemented
pending its metric and time-boundary rules.

The collector is connected to app startup/shutdown and reports its enabled,
cycle, event-stream and failure status on the authenticated Settings page.
Maintenance expires the one-hour window and retries storage recovery while the
rest of the app remains available. Cumulative early sample evictions are shown
as a shortened-history warning with timestamp bounds that are explicitly not a
continuous-gap claim. Durable pending-write markers are reconciled from their
atomic committed bit without replaying the interrupted batch. While collection
is disabled, the default-off collector does not reopen the metrics database;
after a process restart, stored eviction history is therefore not rediscovered
until collection is enabled again.
Collection targets are not schedule guarantees. The initial overview dashboard
read the opted-in SQLite history and plotted per-container CPU, raw memory usage,
and network receive/transmit rates, selecting the latest recorded sample in each
30-second bucket. On 2026-11-23 the owner requested an overview focused on
aggregate and system metrics rather than per-container lines; detailed
per-container charts remain in Observability. This changes presentation only:
collection, stored samples and the one-hour range remain unchanged.

For this bounded UI slice, aggregate charts sum values from the collected
container series for each display bucket, and show a gap if any returned
container series lacks a value for that metric in the bucket. They do not infer
zeroes or alter retained samples. CPU totals may exceed 100%; raw memory sums
describe Docker container usage, not host RAM. The chart selects the latest
recorded sample per container in each 30-second bucket, so a displayed total is
not a simultaneous host measurement. True host CPU, memory, storage and network
measurements require a separate host data source and remain unimplemented; do
not present container aggregates as host metrics or assume additional mounts or
privileges. These charts cover the last hour and report when collection is
disabled, starting, paused or has no observations.
The authenticated Containers observability view presents one selected container's
one-hour metric charts with a vertical timeline slider, observed lifecycle
markers and a newest-first list bounded to 200 events for that container.
The filter sidebar remains separate from a slimmer container-name sidebar.
The name list combines an on-demand read-only Docker snapshot with retained
event/state/sample identities and defaults to its first filtered entry in name
order. Historical-only identities use IDs and unknown association/state rather
than inventing current metadata. An explicit unknown or filtered-out identity
is unavailable, never a fallback to all containers.
Slider, marker, list, chart-click and
chart-keyboard selection share the `observability:timechange` event and keep
the timeline and chart cursors synchronized. Opening, filtering or selecting
in this view requests one read-only snapshot for names and filter values; chart
and event data still come from stored history, without enabling collection.
Event history may be incomplete; the view labels
observed events rather than asserting a complete lifecycle record. A bounded
event list is explicitly marked when older events are omitted. No persisted
per-container gap ledger, hard filesystem quota or measured preset
sizing/throughput is implemented. Historical logs and TimescaleDB remain
unimplemented in the collector and web views. A bounded Docker API reader and
SQLite log-persistence foundation are implemented separately: original message
bytes and timestamp/stream/truncation metadata survive reopening; selected and
aggregate queries are newest-first, limited and explicitly signal omitted rows;
log-only identities join the existing history inventory. Shared maintenance
expires logs atomically with samples/events/states using the same cutoff.
The foundation does not enable log capture, add a background collector, choose
replay deduplication, extend managed pressure eviction or provide log tabs.
Source decoding preserves individual non-TTY Docker frame records, including
multiline and non-newline-terminated output. TTY output is a merged, line-oriented
stream; individual source-record boundaries and stdout/stderr separation cannot
be reconstructed reliably for non-newline-terminated TTY output.
The implementation bounds stored messages to 64 KiB, Docker frames to 1 MiB,
insert batches to 128 records and query results to 1,000 records plus a 1 MiB
message-byte budget over at most an hour. These are implementation resource
guards, not measured owner-approved throughput targets. Truncation is metadata,
oversized/malformed frames are
explicit source errors, and following EOF reports lost continuity. Plain store
writes require managed-store integration before production collection, including
durable pending-write handling and the still-unresolved shared pressure policy.
Platform behavior,
live-Docker recovery and physical disk-full behavior remain unverified. A
test-only Docker/tstorage feasibility spike is recorded in
[verification](../verification.md#docker-metrics-storage-proof-of-concept); it
does not implement these behaviors.

## Dependencies

- Depends on [AUTH-001 through AUTH-006](authentication.md) for user-bound local-session
  authentication through the shared app access boundary.
  SSE/session expiry and denied access remain unresolved, not separate auth
  policy owned by a metrics or transport handler.
  AUTH-005's required password-change gate also applies to protected streams.
- Depends on [RES](project-resolution.md) for accepted identity/inputs and
  [PROJ](project-membership.md) for membership/active state.
- Depends on [OPS-001](operation-coordination.md) for queued/executing
  coordination. Broader outcome/unknown semantics still need adoption.
- Consumed by the [prototype plan](../planning/README.md#local-prototype-contract-triage).
  Desktop/mobile presentation does not grant authority to execute actions.

## Acceptance Cases

### Approved Presentation Cases

- On 2026-11-23 the owner requested fixed navigation and side panels with
  independent content scrolling and internal sidebar scrolling. This alignment
  uses a viewport-height desktop shell above 900 CSS pixels; compact layouts
  retain natural page scrolling and existing disclosure controls. The breakpoint
  is an implementation choice, not an owner-approved exact widget dimension.
- Settings navigation is separate from the selected category's content and
  remains usable on desktop, narrow screens, keyboard and touch.
- Preset profiles hide Custom-only scope and interval controls; selecting
  Custom reveals them. Editing an individual setting selects Custom.
- On 2026-11-23 the owner requested Docker stats and Container logs profile
  controls together in Metrics, each with Off/Low/Medium/High/Custom positions.
  Expanded source settings stay contained with their own selector in a
  card/panel rather than spilling into another source's controls. The overall
  selector applies the owner-approved category-wide preset described above;
  no concrete log preset value, default,
  migration, shared-budget or collection implementation is approved merely by
  approving this presentation.
- Storage targets accept B, KB, MB and GB, convert using decimal units to exact
  whole bytes, default to MB for initial setup, and reject invalid or
  overflowing amounts without saving. The unit selector appears within the
  right end of the input.
- Large chart legends stay bounded; every series remains discoverable and can
  be toggled individually without changing the plot's layout.
- The overview shows aggregate collected-container metrics rather than
  per-container metric lines; per-container charts remain in Observability.
  On 2026-11-23 the owner requested one main Overview chart with switchable
  CPU, memory and network views, following the supplied dashboard recording.
  Network keeps receive and transmit together. Switching changes presentation
  only, not collection, samples, selected-time synchronization or the hour range.
  Missing observations remain gaps, and container totals are not presented as
  host-level measurements. The wide dashboard is designed for the owner's
  1440p reference screenshots and remains usable without overflow at smaller
  desktop and mobile widths.
- The initial observability view shows the last hour; chart, slider, marker and
  event-list selections stay synchronized. Only recorded events appear, and the
  view does not imply Docker event history is complete.
- On 2026-11-23 the owner requested a UniFi-inspired single-container
  Observability workspace: selecting a container in the secondary sidebar opens
  only its detail/history view, the first available container is selected by
  default, the timeline is vertical, metric charts are stacked, and records sit
  underneath. There is no multi-container overlay or aggregate option on this
  page. This supersedes the earlier all-container Observability presentation,
  not aggregate Overview.   The owner clarified that filters belong in the main sidebar and container
  names in a separate slimmer sidebar, and supplied an additional unscaled
  1080p desktop reference with Chrome occupying part of the screen. The
  implementation uses an on-demand read-only snapshot for names and filters,
  with retained-ID fallback for history absent from the snapshot. CPU, memory
  and network charts are stacked; RX/TX are two metrics of the same container,
  not an overlapping-container or aggregate view.
  The bottom section exposes observed lifecycle events and an explicitly
  unavailable container-log view. That presentation request did not authorize
  a new log collector; the subsequent one-hour SQLite collection direction is
  recorded above, with consent and remaining policies still unresolved.
- The owner subsequently confirmed desktop-first delivery and deferred mobile
  alignment. Full desktop/mobile parity and 100% mobile feature coverage are
  not required. Basic controls and basic statistics should remain usable when
  implemented; advanced views may later be simplified, truncated or disabled.
  Exact mobile feature availability remains a later decision. This direction
  does not authorize implementing Up/Down effects or disabling existing views.

### Proposed Cases Requiring Review And Eventual Execution-Based Checks

- External lifecycle changes reconcile dashboard status independently of metrics.
- Missing metrics do not mean stopped containers or completed operations.
- A quiet healthy connection is not failed merely because no state event occurred.
- Bootstrap/reconnect obtain an authoritative snapshot rather than assuming no
  events were missed. Reject stale/out-of-order updates across views.
- Multiple tabs do not cause duplicate complete collector sets; connection
  recovery, hidden views and subscription cleanup respect measured resource bounds.
- Desktop/mobile and keyboard/touch retain readable status, usable actions and
  recoverable connection feedback. No exact widget/layout is approved for the
  remaining live-state actions yet.
- The exact reduced mobile feature set remains undecided under the owner's
  desktop-first direction. Current bounded reflow preserves access until mobile
  alignment is authorized; no particular advanced view has been selected for
  removal.

## Decisions

- Owner prefers SSE and raised polling fallback, Docker stats/status streams and
  later UniFi-like historical tracing. Detailed implementation was explicitly
  uncertain; the alternatives below remain proposed.
- Proposed: shared Docker events for debounced invalidation plus authoritative
  list/inspect reconciliation, distinct from metrics sampling. Engine stats API
  streams are per container, not one all-container stats endpoint. A browser SSE
  feed can multiplex data without duplicating Docker work per viewer; collection
  limits and cadence still need measurement/approval.
- Proposed: fallback or low-rate reconciliation based on connection health and
  freshness, not absence of lifecycle events alone. Runtime status refresh is
  not project discovery; PROJ-006's on-demand discovery remains unchanged.
- History: distinguish operation records, runtime transitions and metric
  samples, with identities/timestamps/outcomes and redaction boundaries. Retention,
  downsampling, incident thresholds and idle collection remain unapproved. No
  operation-history backend or event-sourced core is mandated merely to preserve
  future options. LIVE-003 selects an opt-in metrics storage direction, not
  automatic collection for every configured Docker socket.

## Verification

The approved Settings/chart presentation refinement and the bounded
authenticated timeline implementation were exercised on Linux on 2026-11-23.
`make check QUIET=1`, `make docs && make check-docs`, and `make check-security`
passed. Browser checks cover chart selection, profile transitions, unit
conversion, keyboard/touch use, the no-JavaScript form fallback and
synchronization of the last-hour timeline, event markers/list and charts.
See the [verification record](../verification.md#metrics-settings-and-large-chart-usability)
and [timeline verification](../verification.md#authenticated-one-hour-observability-timeline)
for exact scope and remaining limits.

These checks do not establish the whole LIVE-003 contract. Live Docker
collection, physical disk-full recovery, platform-specific behavior, measured
resource costs, and historical logs remain unverified or unimplemented. See
the [source register](../planning/sources.md).

The aggregate Overview presentation slice was exercised on Linux on
2026-11-23. `make check QUIET=1` and `make docs && make check-docs` passed.
Regressions verify complete-bucket sums, gaps for incomplete aggregate buckets,
and per-container chart retention in Observability. Browser checks verify
aggregate-only Overview labels, explicit unavailability of host-level resource
metrics, and no horizontal overflow at 2560×1440 and desktop widths from 1440
through 620 pixels; the full browser suite includes mobile Chromium. These
checks do not add or verify a host telemetry source, live Docker collection,
physical disk-full recovery, platform-specific behavior or measured resource
costs.

The single-container Observability refinement is recorded separately in
[workspace verification](../verification.md#single-container-observability-workspace).
It replaces the all-container presentation with selected-container charts and
events, plus on-demand snapshot names and filtering. Historical logs and final
mobile feature scope remain unresolved; no whole-contract fulfillment is claimed.

The subsequent [Docker log API and SQLite foundation](../verification.md#docker-log-api-and-sqlite-foundation)
was exercised separately with mock HTTP/Unix-socket data, reopen/migration,
bounded-query and exact maintenance-cutoff checks. Application, documentation
and dependency security checks passed; final finite/follow input refinements
were rechecked with focused runtime/integration race tests. This does not
verify production log collection, consent, pressure/replay policy, log UI,
live Docker driver behavior or secure erasure of expired data.

The desktop shell's independent main/sidebar scrolling was exercised on Linux
on 2026-11-23. `make check QUIET=1`, `make docs && make check-docs`, and
`make check-security` passed with a persistent `/config` toolchain supplied
through `PATH`. The authenticated Chromium flow checks main and sidebar
scrolling, keyboard and wheel interaction, Settings navigation, and
collapse/reopen controls; compact-layout touch behavior is covered in that
flow. This does not verify native mobile browsers or approve future actions.
