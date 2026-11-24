# Planning Sources

**Inspected:** 2026-11-19. This is a bounded local staging/research record, not
an exhaustive inventory, fresh forum audit or source implementation review.

## Fixed Baselines

| Source | Revision / scope | Qualifications |
| --- | --- | --- |
| Product scaffold | `mstrhakr/go-template`, published `main`, `f281859b8c912ab7e251c7b2a6ddcaf6e663393b` | Fetched main; exported tracked files without Git history or ignored outputs. Kept MIT and htmx notices. Current local template branch was not used. |
| Compose Manager | Refreshed local HEAD `10e9179cbb0331fb956a0b60ba671e477a74638e`; original staging baseline `5ba0c341c7e985004c8ef3802f80793ecbfae509` | Refreshed 2026-11-19: worktree has unrelated planning/source-register/development/e2e changes. Read affected unchanged source contracts, runtime-plan passages and metrics-publisher boundaries; no sibling edits/tests or assumption of remote publication. Original clean staging evidence retains its original scope. |
| Docker Networks | Local `/code/docker.networks`, `main`, `d8d392f758250bdbfb181a196b283b14429f8a69` | Worktree clean; local main reported one commit behind origin/main. Used local checkout as supplied, not a claim to latest remote behavior. |
| Compose Specification | Local `/code/compose-spec`, `3a4a6995e4a971f6173dff9130cb3e38e6738651` | Clean worktree at the bounded 2026-11-19 reading of Compose files, include, build Git contexts and deploy purpose. Not a complete spec audit; network/volume chapters remain to inspect for those workstreams. No sibling files changed. |
| VM script | Pending upload to `scratch/` | Not supplied or inspected. No inferred storage format, credential handling, operations or deployment model. |

Template source at a published fixed revision:
<https://github.com/mstrhakr/go-template/tree/f281859b8c912ab7e251c7b2a6ddcaf6e663393b>.
The source repositories retain their own ownership; nothing in this product
bootstrap publishes their branches or modifies their worktrees.

## Standalone Owner Refinement

2026-11-22: the owner selected opt-in per-container metrics collection and
history, tstorage for metrics with SQLite retained for other application state,
and a low-power-friendly settings approach to be refined. Docker socket
configuration alone does not opt in. The owner also requested real overview
charts, a summary-focused dashboard, and a reusable column customizer. See
[LIVE-003](../contracts/live-state.md#live-003-opt-in-container-metrics-and-historical-views);
sampling, retention tiers, limits, and tstorage fitness remain unverified.
The owner subsequently requested a Settings page with collection scope selectable
between all running containers and containers associated with activated stacks,
because collection affects disk use; the default and edge cases remain open.
The owner then selected a setup wizard with Low, Medium, High and Custom profiles,
selecting activated stacks for Low and Medium and all running containers for
High; changing an individual setting switches the profile to Custom. Details and
actual resource costs remain open.
The owner selected a roughly 90% metrics-storage usage target with 10%
headroom, intending downsampling to precede removal and incompatible
configurations to be prevented. Exceptional exhaustion behavior remains open;
the target is not a guaranteed disk-space limit.
The owner selected an initial single-resolution one-hour history for each
preset: Low targets 15-second samples, Medium 5-second samples and High
1-second samples. Longer-term tiering and retention remain separate decisions;
the selected intervals are targets, not measured freshness guarantees.
The owner approved expiry without downsampling after one hour for this initial
single-resolution version, provided the UI explains its history limit. This
does not supersede the later tiered-retention downsampling direction.
After considering a 24-hour raw window, the owner confirmed one hour only for
the initial raw layer, with longer retention to be built on top later. Low,
Medium and High are performance-oriented settings, not a claim that High is
lightweight. The initial observability view should default to the last hour and
use a timeline synchronized with the charts; longer ranges and custom
timeframes are future advanced options. Display aggregation is distinct from
the retained sampling resolution.
The owner also requested historical logs correlated with usage and relevant
context in the same observability timeline for incident investigation. Log
sources, retention, synchronization and access remain to be refined.
For the initial single-resolution version, the owner chose early removal of
the oldest metrics with a clear warning when the metrics store reaches its
limit before normal expiry, rather than stopping collection as the first
response. How to account for and enforce the limit and what happens if space
cannot be recovered were initially open. The owner subsequently chose to pause
metric writes, clearly report the problem and retry when space is available if
space cannot be recovered, while keeping the rest of the app usable. Limit
accounting and enforcement still need verification.
The owner clarified that Low, Medium and High configure the entire metrics
feature, including a storage limit adjustable in the wizard, and individual
changes select Custom. Suggested limits should be informed by measurement;
their values are not yet selected. The preset/custom settings UI should be
reusable for other large features, without presuming their specific settings.
For activated-stack metrics when multiple discovered folders resolve to the
same Compose project name, the owner chose to collect the corresponding
Docker project's containers if any matching folder is activated, while making
the shared identity clear. The association does not establish ownership.
When a container leaves collection scope through deactivation or a settings
change, the owner chose to stop new collection while retaining existing
samples until normal expiry or storage pressure removes them.
The owner approved an initial collection scope of CPU utilization, memory usage
and limit, network receive/transmit byte rates, block read/write byte rates,
container running and health state, observed start/stop/exit/restart/OOM/health
events, and collector freshness, gaps, errors and shortened-history reporting.
The selected timeline should correlate charts with event markers and an event
list. Missing observations, counter resets and disconnected periods are gaps;
Docker event replay cannot guarantee complete history. Logs and additional
integrations remain later work, not part of this approval.
The owner subsequently clarified that missing values should be zero when a
known state supports it, for example when a container is stopped, and requested
that lifecycle events appear on the timeline and some charts. The exact
metric/state and time-boundary rules remain to be refined; this is not approval
to turn unknown or disconnected periods into zero.
After a reproduced tstorage write-failure issue, the owner selected a separate
SQLite metrics store for the initial one-hour implementation. A possible future
TimescaleDB backend, longer retention, and backend-specific limits or features
are deferred pending evidence. This replaces the earlier tstorage choice for
the initial implementation, not the approved storage-pressure behavior.
The owner asked whether metrics can be stored separately and later migrated to
TimescaleDB on upgrade, with the local metrics database removable afterward.
Migration, cutover and cleanup details are future work and are not implemented.
On 2026-11-23 the owner approved distinct Settings category navigation,
Custom-only scope and interval controls for collection profiles, and storage
target entry in B, KB, MB and GB. The UI uses decimal multiples and persists an
exact whole-byte limit. A new setup defaults the unit selector to MB and places
it inside the input's trailing edge. For crowded charts, retain all series with
bounded, individually switchable series controls; collection and retention
behavior are unchanged. These refinements are recorded under
[LIVE-003](../contracts/live-state.md#live-003-opt-in-container-metrics-and-historical-views).

On 2026-11-23 the owner requested a dashboard presentation change: show
aggregate and system metrics rather than per-container lines on Overview, while
keeping detailed per-container charts in Observability. The owner clarified
that the supplied dashboard references target a 1440p display. The current data
source contains opt-in container metrics, not host-level CPU, memory, storage
or network readings. The bounded overview slice aggregates only those existing
container samples, preserves missing values as gaps, and does not add a host
collector, mount or privilege. Host metrics remain a separate source and
security/collection decision. The overview's responsive presentation is
verified at 2560×1440 and narrower desktop/mobile widths; see the
[verification record](../verification.md#aggregate-overview-dashboard-presentation).

On 2026-11-23 the owner clarified that UniFi keeps navigation and sidebars fixed,
allows internal sidebar scrolling, and scrolls only the main content. This
supplies the next bounded shell-alignment requirement; screenshots alone did
not establish scroll ownership or exact CSS dimensions. Desktop pane scrolling
is implemented independently of compact-layout page scrolling.

The owner also supplied a recording and examples of a right-side details panel
opened from a selected list item, with separate detail, activity and settings
views. A similar project/container panel is a future UI direction; the examples
do not approve Compose action effects or additional management policies.

On 2026-11-23 the owner requested a single Overview chart with switchable metric
display, following the supplied dashboard recording. The bounded presentation
uses CPU, memory and combined network receive/transmit views over unchanged
last-hour aggregate samples; Observability retains separate per-container charts.

On 2026-11-23 the owner requested Observability's single-container layout and
interactions based on the supplied recording: a secondary-sidebar container
selection (defaulting to the top entry), vertical timeline, stacked charts and
bottom logs/records area, with no aggregate or overlapping-container view.
The recording was decoded locally; its Observability segment demonstrates
selected-entity charts, a newest-at-top time rail, synchronized time selection
and records beneath the charts. The bounded implementation uses retained
container identities and observed events. The owner further clarified the
separate filter sidebar and slimmer container-name sidebar, directing attention
to the actual Observability page rather than adjacent UniFi screens. A supplied
1434x768 screenshot is described as an unscaled 1080p desktop with Chrome taking
part of the screen. The implementation adds on-demand read-only snapshot names
and filter values, retaining historical-only IDs with unknown current metadata.
Historical log collection remains unavailable, not fabricated or inferred.

On 2026-11-23 the owner confirmed desktop-first work, with mobile alignment
later and basic controls/statistics rather than full desktop parity. Advanced
mobile views may eventually be simplified, truncated or disabled; the exact
feature set is not decided and existing views have not been disabled.
The owner requested container-log collection in the same local observability
SQLite database with one-hour retention, deferring TimescaleDB for longer log
and metric history. An aggregated dashboard log view is requested; a separate
logs navigation view with stack/container/network filtering remains a tentative
follow-up. At that stage, Docker stdout/stderr and an additional default-off
log-collection option were proposals, not settled source/consent policy. The
log opt-in question had not received an owner answer at that stage. Shared storage-pressure
handling currently evicts samples only and needs a bounded policy for logs
before production capture.

The owner subsequently agreed with the Docker API/original-message-plus-metadata
model and requested beginning implementation, with contract refinement as needed
throughout. This approves the initial stdout/stderr source, SQLite prototype and
optional later TimescaleDB direction, not the earlier unanswered consent choice.
The first bounded slice provides API decoding, SQLite persistence and shared
one-hour expiry without enabling capture or selecting managed pressure/replay
policy. Optional JSON parsing and production collector/web integration remain
pending. Dedicated log infrastructure such as Loki is a later option justified
by actual volume/search needs, not a new prototype dependency.

On 2026-11-23 the owner requested log settings in the Metrics category with
separate Off/Low/Medium/High/Custom source selectors matching Docker stats.
When expanded, each source's selector and controls should stay contained in its
own card/panel. This supplies the next bounded presentation direction and
replaces the earlier unapproved separate-checkbox proposal; it does not settle
log preset values or collection defaults. An overarching selector for Metrics
as a whole was suggested tentatively ("we could also"), not approved as a
particular master/preset mechanism. Its inclusion, whether a preset enables
previously disabled log collection, and whether it overwrites source preferences
need refinement before implementation. Existing code still has one four-choice
metrics profile plus a separate enabled checkbox, no independent log settings,
and no production log collector.

The owner subsequently answered "i choose A" to the reviewed overall-Low
scenario on 2026-11-23. The overall selector is approved as a category-wide
preset: Low/Medium/High sets both sources to the corresponding preset,
including enabling previously Off logs when saved; Off sets both sources Off.
Individual source changes make the overall selection Custom. Effects are
visible before Save, not applied immediately by slider movement. This settles
the earlier overall-selector inclusion/consent question, not log preset
parameters, initial/migration defaults, Off-history effects or shared pressure
policy. No runtime or settings implementation changed during this refinement.

The owner then refined the reusable Settings pattern on 2026-11-23: individual
settings use the same discrete slider/selector styling with their own meaningful
labels/options, not necessarily Off/Low/Medium/High/Custom. Off or Medium may
be omitted. Scope uses named scope choices within that presentation. Storage
uses preset amounts plus Custom, revealing the existing amount/unit input as a
reusable component; exact preset sizes remain unselected/unmeasured. Dependent
controls show/hide with parent enablement. The owner explicitly allows settings
that cannot reasonably conform. Hidden-child preservation/reset, custom-value
restoration and nested preset propagation remain undecided; no runtime/component
implementation is authorized merely by approval of this interaction pattern.

The owner answered "1 for sure" to preserving saved child preferences while a
parent is Off: stop collection, hide dependent controls without erasing values,
and restore them on returning to Custom. Explicit source presets apply their
own values. This does not settle retained-data deletion or preservation across
explicit preset changes. The owner also required visible descriptions of what
each preset selects. Subtext and visible-but-disabled Custom inputs were offered
as possible presentations, not a settled widget choice.

The owner then selected "visible-values": enabled preset settings keep their
actual value controls visible but noneditable, and Custom makes them editable.
Read-only fields are preferred where supported, readable summaries otherwise;
named scope choices use explanatory text without a redundant field. Off still
collapses dependent controls, with a "Not collecting" source summary. This
settles the earlier subtext-versus-value-control presentation proposal.

The owner selected "unlock-current" for returning to Custom after an explicit
preset: unlock the currently displayed preset values rather than restoring a
separate earlier custom configuration. Explicit preset selection replaces prior
custom values; disabling via Off alone continues to preserve preferences.
Nested individual-setting/source/category Custom propagation remains open.

The owner then approved upward Custom propagation without sibling changes:
editing one leaf marks its source and overall category Custom, leaving other
values untouched; selecting a category preset explicitly reapplies both source
presets. The owner clarified parent presets may map to differently named child
choices because some omit Medium or Off. Do not copy labels blindly or invent
missing choices. Overall Off preserves last child settings, and returning
directly to Custom restores them even where a child has no Off position.
Concrete per-setting preset mappings remain undecided.

The owner approved the first log preset mapping: Low/Medium use Activated
stacks, High uses All containers, and Custom exposes the two named scope
choices. Include readable backlog for stopped containers within that scope;
numeric stats continue to sample running containers. Off preserves scope
rather than inventing an Off scope position. Existing activation/resolved
project-name association is reused; no Compose effects, complete-history
guarantee or concrete log interval/budget is approved by this answer.

The owner then requested High use a live stream instead of polling, preferring
streaming for High where supported. High logs uses read-only Docker logs with
follow enabled, not interactive attach/stdin. Original Docker timestamps remain
authoritative. High's all-container scope still includes readable stopped
backlog; stopped EOF must not be presented as ongoing live continuity.
Low/Medium intervals, bounded stream/reconnect policies and any stats transport
change remain unapproved; this is not approval of a generic new streaming layer.

The owner selected "periodic-60-30": Low fetches available backlog every
60 seconds, Medium every 30 seconds, and High follows live logs. Low/Medium
keep the approved activated scope, High the all-container scope. Frequency
controls freshness, not timestamp resolution or intentional message sampling;
Docker retention/rotation still limits recovery. Custom transport options and
resource/recovery policies remain unresolved.

On 2026-11-23 the owner requested state tracking in a tab/section alongside logs
in Observability and on its timeline, inspired by offline-state reference views.
The owner wants sparse changes, not an identical event every second. Existing
code captures lifecycle/health events and saves discrete state snapshots each
sampling cycle; transition-only state history is not implemented. Lifecycle,
health and observation availability are distinct; taxonomy, reconciliation,
retention baselines and uncertainty presentation need refinement. Other
change-based information such as update status is tentative, not approval to
add registry checks or other sources. The owner further requested Docker
disconnects appear as a distinct grey timeline area, referring to the supplied
screenshots and offering another if needed. This identifies an observation-gap
presentation, not a container-down inference or approval of all proposed state
baseline/reconciliation mechanics. Exact visual alignment remains pending.

The owner clarified on 2026-11-23 that Up/Down/Stop/Restart/Update are distinct
overall action/activity records, while container Observability should show
container-level observed events such as Created, Started and health changes.
Do not conflate requested manager actions with observed state or manufacture
that sequence for every Up. Container-operation linking and activity lifetime
remain unresolved; the request does not authorize action execution. Existing
Docker event ingestion lacks Created events and no manager executor exists.

The owner clarified that the current Observability slice is container-specific;
stack-level Observability will come later. Shared system events (examples:
manager started, Docker lost/restored) may appear on container timelines but
must be stored once, never duplicated into every container's event history.
Container timelines project relevant shared records alongside their own events.
This resolves container-activity scope and shared record ownership, not system
catalog/retention or stack-view implementation.

The owner selected retaining one latest pre-window state baseline per container
plus in-window transitions. This supplies context for unchanged containers,
not repeated events or an assertion of uninterrupted state across shared gaps.
Raw logs/metrics retain their one-hour boundary; baseline removal/cleanup and
reconciliation remain unresolved. The owner reiterated TimescaleDB and possible
Grafana output/integration are deferred, not part of this slice.

The owner approved separate lifecycle and health dimensions, with different
presentations appropriate to different views. Unhealthy periods should be
conspicuous red striped/highlighted timeline areas, distinct from grey Docker
disconnect areas. The primary goal is quickly understanding container behavior
over time, not a status label alone. Color must not be the only explanation;
uncertainty/overlap precedence and exact reference alignment remain unresolved.

The owner selected "unknown-overrides": grey observation gaps override red
unhealthy areas until observation resumes. Last-known health remains context.
A healthy reconnect observation is labeled as observed on reconnect, never a
claimed exact recovery time within the gap. This resolves overlap precedence,
not exact rendering or all reconnect/retention mechanics.

The owner reframed the category as Observability, not merely Metrics, with all
observability options grouped logically by collection type and required API/
backend tracking. Exact independent source controls remain unresolved. Existing
stats collection depends on inspect requests and shared event continuity;
endpoint overlap must not silently remove required safety observations or
justify duplicate collector work. Prior preset/UI decisions apply to the
renamed category, not newly invented source mappings or defaults.

The owner approved the three-source split: stats, state/events and logs, with
shared required API work underneath and state-history persistence distinct from
stats safety observations. New state/event presets/defaults remain open.
For Settings navigation the owner proposed Observability > Containers, flat
Container Observability, or Containers > Observability; sidebar sections,
tabs or an additional sidebar layer were possible presentations, not selected
layouts. A UI-equivalent reference can be supplied; exact hierarchy is pending.

On 2026-11-23 the owner supplied three additional Settings navigation screenshots
and requested saving them with the existing private references. Byte-identical
copies are retained in ignored `scratch/ui-references/` as
`unifi-settings-threat-management.png`, `unifi-settings-content-filter.png` and
`unifi-settings-traffic-logging.png`; attachment originals were not modified.
Do not commit or publish the reference images.
They show the application rail, a Settings-category sidebar, a separate
category-specific subpage panel with selectable title/description cards, and
the selected page's content to the right. The subpage panel persists across
the three page selections and includes a contextual summary card. These are
visual navigation evidence, not proof of scrolling behavior or approval of
Containers-versus-Observability hierarchy, exact dimensions, summary content or
every displayed feature. At that stage, a container-category/subpage adaptation
remained to be selected with the owner; the subsequent decisions below settle
the organization and basic visual direction.

The owner then requested a broad future-settings inventory before selecting the
hierarchy. The assistant compared existing product/contract scope with
exploratory future candidates; no source-plugin capability was automatically
adopted. The owner responded "i like it" and requested a final design pass.
Record resource-first Settings > Containers > Observability as the accepted
organization direction, with shared Data & retention ownership and category
subpages where useful. The broader category list remains candidate scope, not
approval of every feature or a fixed permanent navigation tree.

Nine additional owner screenshots were saved byte-identically under the same
ignored private reference directory, without editing originals:
`unifi-settings-overview-top.png`, `unifi-settings-overview-lower.png`,
`unifi-settings-table-manage.png`, `unifi-settings-table-normal.png`,
`unifi-settings-network-controls.png`, `unifi-settings-subpage-controls.png`,
`unifi-settings-subpage-setup.png`, `unifi-settings-subpage-diagram.png` and
`unifi-settings-subpage-instructions.png`. Do not commit/publish these images.
They show collapsible summary tables, normal versus checkbox-selection table
presentations, compact label/control rows, dependent fields, and a persistent
title/description subpage panel across form, setup and explanatory-diagram
pages. These are visual evidence/inspiration, not copied product assets,
approval of pictured actions or proof of dynamic saving/scroll ownership.
The final pass consolidates settled design and marks exact dimensions,
disclosure/manage-mode details and new widgets as proposals. Runtime code and
browser verification were not changed by this documentation-only review.

During final visual signoff on 2026-11-23, the owner clarified:
"the generic basic layout is what we are going for for most things, with the
extras as ideas for things we can do if needed, where needed."
This approves the offered compact aligned rows, contained source cards,
restrained borders, bounded widths and contextual subpage panel where useful.
All screenshots remain a toolbox of optional patterns rather than a mandatory
widget inventory. Exact geometry, pictured actions, assets, automatic saving,
new capabilities and implementation fulfillment are not approved by this answer.

Protocol evidence consulted: Docker's
[logs API](https://docs.docker.com/reference/api/engine/version/v1.57/operations/ContainerLogs/)
and [stream format](https://docs.docker.com/reference/api/engine/version/v1.57/operations/ContainerAttach/),
plus Moby `v28.5.1`
[log-stream writer](https://github.com/moby/moby/blob/v28.5.1/api/server/httputils/write_log_stream.go).
Moby prefixes each non-TTY framed message with its timestamp even for partial
lines, so newline-based frame concatenation would alter messages. TTY streams
have no equivalent frame boundary. The product retains its existing v1.40 API
request version; these references are protocol evidence, not live-driver or
daemon-version compatibility verification.

2026-11-22: for the first read-only runtime snapshot the owner selected
associating discovered projects with Docker containers by their resolved Compose
project name, even for copies resolving to the same name. This is runtime
grouping for display, not proof of filesystem ownership or authorization to
perform actions. See [LIVE-002](../contracts/live-state.md#live-002-on-demand-read-only-runtime-snapshot).
Compose Specification `02-model.md` at `3a4a699` defines the resource project
label and allows an external project-name override; it does not prescribe
manager adoption. Source STK-ID distinguishes accepted name from runtime
evidence; its plugin-specific trust and migration policies remain unadopted.

2026-11-21: owner chose persistent project listing over a scan-only preview for
the next slice. For a previously discovered project absent on a successful scan,
the owner selected retaining its listing and activation state as unavailable;
scan failure is not proof of removal. See
[PROJ-010](../contracts/project-membership.md#proj-010-retain-previously-discovered-projects-that-disappear).
The owner later selected same-path restoration of saved activation state for
the prototype. Full comparison and historical changes for easier rollback are
long-term ideas; snapshots versus Git and detailed policies remain undecided.
For the first read-only discovery slice, the owner selected project-local `.env`
interpolation without inheriting the manager process environment; broader
environment policy remains open.
The owner also selected blocking activation when a project is missing or its
Compose inputs cannot be resolved, with a clear error; previously active state
is retained.
The owner also selected one saved discovery root initially, with existing entries
not removed merely by changing that setting, entered as an application-visible
path in the authenticated UI. Container mounts are configured separately; see
[PROJ-011](../contracts/project-membership.md#proj-011-one-saved-discovery-root-initially).
Removal and other lifecycle details remain open.

2026-11-21: owner selected the four standard Compose filenames for initial
discovery, deferring `.env`-only `COMPOSE_FILE` discovery. This does not change
native resolution of selected projects. The owner also
selected listing a newly discovered folder with malformed or unresolvable inputs
as inactive with a clear resolution error.
For initial root scans the owner chose to skip symlinked child directories and
report the limitation; other symlink cases remain open.
See [PROJ-005](../contracts/project-membership.md#proj-005-discover-compose-projects-rather-than-every-directory).
Compose Specification `03-compose-file.md` at `3a4a6995e4a971f6173dff9130cb3e38e6738651`
names the default and compatibility filenames; it does not prescribe manager
discovery. The selected model loader's `cli.DefaultFileNames` supplies these names
in its own precedence order.

2026-11-20: owner approved [AUTH-006](../contracts/authentication.md#auth-006-bootstrap-env-does-not-reset-persisted-credentials):
bootstrap env seeds initial auth state only; persisted credentials/setup state
remain authoritative across restarts, and recovery/reset is explicit and separate.
No database, recovery tool or interrupted/corrupt-state behavior was selected.

2026-11-19, direct owner conversation, recorded from product baseline `f4d6148`:

- Owner requested app login/auth among the first prototype steps, despite intended
  reverse-proxy/WAF/local-only deployment, with foundations for future methods
  such as OIDC. [AUTH-001/002](../contracts/authentication.md) record that scope
  and ownership direction. "Basic auth" remains ambiguous between HTTP Basic and
  simple form/session login; immediate OIDC was tentative, not approved delivery.
- Owner then selected option A: local username/password form login with
  server-managed sessions first, OIDC later.
  [AUTH-003](../contracts/authentication.md#auth-003-local-form-login-with-server-managed-sessions-first)
  resolves the initial ambiguity without selecting account count, bootstrap,
  recovery, storage/library or session security parameters.
- Owner then chose one administrator for the prototype, explicitly requiring
  correct future multi-account plumbing instead of a singleton implementation
  needing replacement. [AUTH-004](../contracts/authentication.md#auth-004-one-initial-admin-with-multi-user-capable-identity-plumbing)
  records stable internal identity, distinct sessions and one shared access
  boundary, without approving multiple-account UI, RBAC, an identity framework
  or immediate OIDC delivery.
- Owner chose browser setup with an explicit change from the proposed one-time
  setup credential: provision the initial user via Docker/deployment environment
  variables, then require a password update at first login through the setup
  wizard. [AUTH-005](../contracts/authentication.md#auth-005-environment-bootstrap-with-forced-first-login-password-change)
  records the amended flow. Env reapplication/restart and reset/recovery details
  remain unresolved; no real bootstrap secrets or variable names were requested.
- Owner clarified the overall product as a standalone counterpart/clone of the
  Unraid Compose Manager Plus plugin, with a modern interface instead of its
  Unraid-native presentation. This strengthens the plugin's role from case study
  to functional baseline; it does not approve every source draft, legacy naming
  behavior or all-feature first-prototype scope. Existing standalone decisions
  and the bounded local prototype remain in effect. Naming and database details
  are not approved merely by this broader product description.
- Owner subsequently explicitly clarified native naming: UI labels are
  display-only; explicit runtime naming belongs in Compose configuration, not a
  separate manager setting or injected project-name flag. Prototype support for
  an external override is not needed. [RES-001](../contracts/project-resolution.md)
  records this approval, excluding legacy normalization/pinning machinery while
  leaving existing custom-name adoption and native-name changes unresolved.
- Owner then selected native Compose input selection for the prototype over
  implementing ordered-file controls immediately. [RES-006](../contracts/project-resolution.md)
  records that bounded approval; native env/input mechanisms and SDK support
  are not assumed away, and env/profile scope remains separately reviewable.
- Owner preferred selecting a project folder during setup, with possible detected
  location suggestions. Suggestion mechanisms and exact root scope remain open.
- Owner selected automatic listing rather than per-project adoption for listing,
  while requiring activation before management.
- Owner explicitly selected option A in the follow-up manual-Stop scenario:
  activation gates both manual and automatic project changes. These bounded
  approvals are recorded as [PROJ-001/002](../contracts/project-membership.md).
- In the subsequent activation-effects scenario, the owner explicitly selected
  permission-only option A: no Compose run, project-file rewrites or schedule
  enablement. The stated rationale was avoiding unexpected automatic actions.
  [PROJ-003](../contracts/project-membership.md#proj-003-activation-grants-permission-without-applying-changes)
  records this approval without generalizing it into unreviewed automation policy.
- Owner explicitly selected direct-child rather than recursive project-folder
  discovery for now. Interest in nested discovery and UI grouping was not approval
  of either feature. This depth-only decision is recorded as
  [PROJ-004](../contracts/project-membership.md#proj-004-direct-child-project-discovery).
- Owner then selected recognized Compose projects only, not every child directory
  (PROJ-005). The following message added on-demand discovery, optionally during
  setup (PROJ-006), and selecting stack folders one at a time through browsing
  (PROJ-007). These decisions do not determine exact filenames, picker path limits
  or missing/invalid-input behavior. The earlier automatic-inclusion decision
  still applies when a requested scan runs, not to background scanning.
- Owner explicitly added Git-stack capability scope: pulling repository sources,
  private-repository credentials and CI-triggered deployment through webhooks or
  similar integration. [GIT-001](../contracts/git-stacks.md) records scope, not
  detailed authentication, activation, deployment or automation policy approval.
- Owner then clarified that the extra activation gate is only for discovery-added
  stacks. Explicit manual additions and cloning a repository as a new Git stack
  start active; there is no deactivate option. PROJ-002/007 were amended with
  supersession notes and PROJ-008/009 record the explicit entry-path and
  no-deactivation rules. This did not answer the pending per-deployment CI approval
  question or enable CI automation merely through active membership.
- Owner subsequently explicitly selected unattended CI deployment over per-run
  UI approval. [GIT-002](../contracts/git-stacks.md#git-002-enabled-ci-deployment-requires-no-per-run-ui-approval)
  records the scenario: CI deployment already enabled for an active stack, and
  an authenticated, authorized trigger. The stated rationale was that requiring
  per-deployment manual approval defeats CI deployment's purpose. No revision,
  credential protocol or job-concurrency decision was inferred.
- Owner then selected exact-commit CI deployment, asking to use existing provider
  conventions where available rather than invent a universal webhook schema.
  [GIT-003](../contracts/git-stacks.md#git-003-deploy-the-immutable-revision-selected-by-ci)
  records that outcome. Provider/event mapping and authorization remain open;
  research is qualified under [CI revision conventions](#ci-revision-conventions).
- Owner selected GitHub-first CI integration over initial GitHub plus GitLab
  support. [GIT-004](../contracts/git-stacks.md#git-004-github-first-for-ci-deployment-integration)
  records this scope choice, without inferring GitHub-only repository hosting,
  Enterprise support, credential types or a specific GitHub trigger event.
- Owner selected explicit deployment requests from CI over manager deployment
  triggered by configured workflow success alone.
  [GIT-005](../contracts/git-stacks.md#git-005-ci-explicitly-requests-deployment)
  records CI-controlled promotion intent; exact GitHub event/API, authentication
  and delivery semantics were not inferred.
- Owner requested considering the local Compose Specification during contract
  work. The three contract skills now include that evidence step, preserving
  the distinction between normative Compose semantics, implementation support
  and standalone manager/CI policy.
- Owner raised possible backend reuse with the plugin and requested an opinion
  before deciding. The supplied other-agent excerpt is architectural context,
  not fresh implementation evidence, an approved shared-service design or plugin
  migration authorization.
- In follow-up, the owner approved emphasizing maintainability and clear ownership
  rather than building for hypothetical reuse, and said they do not currently
  intend to rebuild the plugin. The bounded
  [planning direction](README.md#maintainability-and-ownership-direction) is clean
  boundaries now, extraction when justified; shared-backend delivery is not a
  product goal. The owner's comment about future native Unraid Compose support
  is context for that preference, not independently verified release evidence.

## Local Compose Contract Reading

Start under `/code/compose_plugin/docs/contracts/`:

- `README.md`: authority, status, ownership and workflow conventions; read.
- `inventory.md`: counts, discovery limitations and approval qualifications;
  opening inventory/progress sections read.
- `sources.md`: current opening source/approval register inspected; not a new
  retrieval of all referenced issues, PRs or forums.
- `project-identity.md`, `stack-registration.md`, `stack-files.md`,
  `stack-lifecycle.md`, `mutation-coordination.md`, `ui-operations.md`: bounded
  purpose, requirements and nearby relevant sections inspected for the reuse map,
  not full independent extraction or enforcement verification.
- `profiles.md`, `state-metadata.md`, `feature-outline.md`,
  `definition-priorities.md`, `compose-manager-plus-forum-research.md`: identified
  follow-up/dependency context; not fully reviewed in this phase.
- Update, automatic-update, retry, build-on-update, editor/history and bulk
  contracts remain outside the first proposed slice.
- Git follow-up: bounded GIT-LIFE/CHECK/DEPLOY/AUTO and CRED-VAULT/GIT sketches
  in `feature-outline.md`, plus Git/storage/credential inventory entries read.
  No Git implementation/caller trace, test execution, historical refresh or
  plugin webhook support verification was performed. These sources identify
  candidate ownership boundaries only; see the
  [Git reuse mapping](contract-reuse.md#git-and-deployment-candidates).

The local collection is not assumed published. The handoff warned that its
previous branch/HEAD was unavailable remotely; availability of the new source
HEAD was not established here. Do not replace these references with fabricated
GitHub blob links. The reuse map provides a self-contained bounded summary, not
a complete source snapshot. Before implementation on another machine, obtain
this pinned checkout or an owner-approved documentation snapshot; recheck
approval and supersession sections there. Do not silently use older origin/dev.

## Foundation Adoption And Live State

2026-11-19, following product checkpoint `68ab9d2`:

- Owner redirected work toward enough foundational contracts for a locally usable
  dashboard, basic Compose actions, manual Update and selected combinations.
  Git Deploy and broader features remain later slices. The owner authorized
  adopting obviously portable source rules and asking about material adaptations.
- [RES-002 through RES-005](../contracts/project-resolution.md) adopt selected
  approved STK-FILES-14/18/13/19 outcomes without plugin tier/path/mechanism scope.
  At that batch's creation RES-001/006 retained proposed identity/default-selection
  choices; subsequent RES-001 naming approval is recorded above. No blanket
  approval of source STK-ID drafts or standalone migration policy is inferred.
- Spec `03-compose-file.md` and `04-version-and-name.md` were read for native
  file/path and top-level project-name semantics. Those chapters do not select a
  standalone registration/pinning workflow or prove SDK invocation equivalence.
- Source `docs/ui-runtime-plan.md`, bounded runtime/transport/recovery passages,
  distinguishes runtime observations, operation results and metrics; it describes
  Nchan WebSocket metrics and proposes shared Docker events with debounced
  invalidations plus authoritative reconciliation. Bootstrap/reconnect refresh
  and rejection of obsolete responses are source design evidence, not a fully
  approved standalone live-state contract.
- Source `nchan/compose_info` producer boundaries were inspected: the CLI stats
  process is given Compose-managed container IDs and publications are coalesced
  instead of broadcasting the whole map per incoming stats line. This mechanism
  is observed evidence, not Go code to import or a selected sampling policy.
- [Official Engine API source](https://github.com/moby/moby/blob/master/api/swagger.yaml)
  was retrieved on 2026-11-19 from a moving upstream branch. The stats endpoint
  `/containers/{id}/stats` streams one container's resource statistics; `/events`
  streams lifecycle events, including container state and health events. No one
  all-container stats endpoint, event-replay guarantee or SDK support matrix was
  inferred. The plugin's one CLI stats process is not equivalent to one Engine
  API stats stream for every container.
- Owner prefers SSE, tentatively described fallback polling and wants later
  UniFi-inspired historical tracing of operations/resource incidents. [LIVE](../contracts/live-state.md)
  records scope/preferences while keeping heartbeat/fallback, collection budgets,
  retention, thresholds and idle collection proposed. Absence of lifecycle events
  is not itself proof of a failed stream. No history backend is selected now.

No source-plugin, Docker, stream/recovery or resource measurements were executed.
This is bounded adoption/research, not full source tracing, a new full-spec audit
or implementation authorization. Unrelated sibling work remains untouched.

## Auth Implementation Brief Research

2026-11-20, documentation inspection only, for the
[proposed first implementation slice](auth-implementation.md):

- The scaffold's Docker and release builds disable CGO and its container runs as
  non-root with read-only root filesystem. Persistent auth will need an explicitly
  writable, correctly owned state location, not a broad permission relaxation.
- `modernc.org/sqlite` package docs identify CGo-free operation; Go module proxy
  metadata returned `v1.62.0` as a candidate. Nothing was installed or compiled.
- SCS README/SQLite-store source documents server-side sessions and `*sql.DB`
  integration; Argon2id helper README documents configurable hashing/verification.
  These are candidate components, not an already-tested compatible dependency set.
- OWASP password-storage guidance lists the proposed 19 MiB/two-pass/single-lane
  Argon2id baseline. Parameters still require target-host benchmarking and bounded
  verification concurrency; the helper's defaults are not assumed suitable.

Links and concrete proposals are in the brief. Source branches/module metadata
are moving research references, not verified/pinned dependencies. Preparing the
brief and delivering the planning branch is authorized; implementation/default
acceptance is a separate explicit gate. No runtime tests were run for this research.

## Authentication Evidence

2026-11-19: scaffold route/configuration and security guidance were inspected;
there is no app login or session enforcement. The owner's reverse-proxy/WAF
deployment context does not select a trusted-header identity protocol or remove
the new [AUTH](../contracts/authentication.md) foundation requirement.

- [RFC 7617](https://www.rfc-editor.org/rfc/rfc7617): HTTP Basic sends Base64
  user/password credentials and is not secure without external transport
  protection such as TLS. Base64 is not encryption.
- [OWASP session management guidance](https://cheatsheetseries.owasp.org/cheatsheets/Session_Management_Cheat_Sheet.html):
  consulted session/cookie and lifecycle guidance when evaluating form/session
  login, including protected transport, cookie protections, renewal and expiry.
  This supports evaluating sessions, not claiming that merely using cookies is
  secure or selecting a session library/storage mechanism.

HTTP Basic is not the same as a simple username/password login page. After the
comparison, the owner approved local form login with server-managed sessions
(AUTH-003) and deferred OIDC. Account/bootstrap, session/security and later OIDC
protocol details remain open. No auth, proxy, browser or credential-security tests
were executed.

## Operation Queue Evidence

2026-11-19, bounded review preceding owner approval of
[OPS-001](../contracts/operation-coordination.md):

- Local Compose `02-model.md` and `deploy.md` at the pinned spec baseline:
  project identity and service rolling-update/rollback parallelism, not a manager
  request queue contract. No inspected passage chooses refusal versus queueing;
  this is not an exhaustive absence claim across all Compose documentation.
- Source `mutation-coordination.md`: distinguishes queue/admission from execution
  ownership and documents immediate busy errors, bounded shell waits and a
  separate background adapter queue. Source guarantees and participant gaps do
  not approve a universal standalone policy. No plugin runtime tests were run.
- [Microsoft queue-based load leveling](https://learn.microsoft.com/en-us/azure/architecture/patterns/queue-based-load-leveling):
  reviewed the published guidance on buffering demand, controlled processing,
  backlog/latency, durability, duplicates and ordering. The retrieved page reports
  source commit `d73cd1632ffa489eff78a16bba4d101c510a1810`. This is architecture
  guidance, not a requirement to use an Azure service or external broker.
- [GitHub Actions concurrency](https://docs.github.com/en/actions/how-tos/write-workflows/choose-when-workflows-run/control-workflow-concurrency):
  read official source/reusable sections describing serialized execution and
  pending work, including default pending replacement and version-qualified queue
  options. Those semantics are comparative evidence, not our ordering or
  cancellation defaults; no universal FIFO guarantee was inferred.

The initial recommendation favored busy refusal for implementation simplicity.
After the owner requested scenarios and evidence, the recommendation changed to
bounded queueing for unattended CI. The owner explicitly approved B, with detailed
queue policies left open. These sources were read, not deployment behavior tested.

## Compose Specification Evidence

Bounded reading on 2026-11-19 at the clean local spec revision above:

- `03-compose-file.md`: preferred/default Compose filenames, ordered merging and
  relative paths based on the first Compose file's parent directory.
- `14-include.md`: included models have their own project-directory/path and
  interpolation environment semantics; include recursion is not filesystem
  discovery recursion or a manager membership policy.
- `build.md`, opening sections through Git context syntax: Git URLs can identify
  a ref and subdirectory for build context. This is not a manager stack-cloning
  API, webhook revision protocol or guarantee of fully reproducible deployment.
- `deploy.md`, opening purpose/attributes: optional service resource/platform
  deployment settings, not an external CI trigger format or approval workflow.

No CI webhook contract was found in the inspected sections; this is not a claim
of exhaustive absence across every spec file. The selected SDK/CLI's actual
support still needs independent validation. No new filename, merge, include or
build policy is approved merely by reading these sections.

## CI Revision Conventions

Official sources retrieved on 2026-11-19; moving upstream branches, not pinned
provider/schema versions or runtime integration verification:

- [GitHub REST API schema](https://github.com/github/rest-api-description/blob/main/descriptions/api.github.com/api.github.com.json):
  `workflow-run.head_sha` identifies the head commit for the workflow run;
  `deployment.sha` is the commit and `deployment.ref` may be a branch, tag or SHA.
  These object schemas identify fields, not every webhook's envelope or proof
  that a workflow checked out and tested the same source revision.
- [GitLab webhook events](https://github.com/gitlabhq/gitlabhq/blob/master/doc/user/project/integrations/webhook_events.md#pipeline-events):
  pipeline events describe status changes; the example `object_attributes`
  includes `sha`, `ref` and `status`. A generic pipeline event is not by itself
  authorization or a command to deploy.

Use the supported integration's event-specific metadata rather than inventing
one universal provider payload. Bind the intended repository and immutable
revision, and separately define trusted sender, event/success selection and
authorization. Ordinary push events and arbitrary SHA fields do not establish
passing CI. This research supports GIT-003's existing owner-approved intent; no
provider adapter, webhook endpoint, protocol or automatic fallback was implemented.

## Network Evidence

Under `/code/docker.networks` inspected `README.md`, `docs/pages-inventory.md`
and bounded entrypoint/function evidence from
`source/docker.networks/include/ExecFunctions.php`, including action dispatch,
protection, deletion, connect/disconnect and template-persistence handoffs.
Identified `tests/template-persistence.php`; did not execute it or trace every
function/caller. The plugin documents network management, not a volume manager.
Current behavior is evidence, not standalone approval.

## SDK Research Baseline

Official entry points:

- <https://docs.docker.com/compose/compose-sdk/>
- <https://github.com/docker/compose>
- <https://github.com/compose-spec/compose-go>
- <https://docs.docker.com/reference/api/engine/>

The official documentation links above came from the handoff; they were not a
fresh full documentation audit. GitHub latest-release API and fixed upstream
Compose module manifest were checked directly on 2026-11-19:

- Compose latest normal release: `v5.8.0`, published 2026-11-17.
- compose-go latest normal release: `v2.17.0`, published 2026-11-16.
- <https://github.com/docker/compose/blob/v5.8.0/go.mod> declares module
  `github.com/docker/compose/v5`, Go `1.26.11`, compose-go/v2 `v2.17.0`,
  Docker CLI `v29.10.0+incompatible`, Moby API `v1.57.0`, Moby client `v0.6.3`
  and BuildKit `v0.33.1`.
- The scaffold uses Go `1.27.4`; satisfying the declared minimum does not prove
  third-party embedding, dependency resolution, helper availability or runtime fit.

Do not independently pick unrelated latest CLI/Engine module versions. During
the authorized spike, inspect official SDK examples and module replacement/vendor
constraints, choose coherent exact versions, inspect licensing/security, run
`go mod tidy` and actual compile/tests, and establish required external tooling
for the chosen operations (build, auth helpers, SSH, etc.). No Compose/Engine
dependency or SDK build was added/run in this phase. Recheck upstream before
the spike; these versions are research evidence, not permanent product pins.