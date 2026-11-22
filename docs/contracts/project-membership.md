+++
title = 'Project Membership And Activation'
weight = 4
+++
# PROJ: Project Membership And Activation

**Status:** Draft / eleven individually approved requirements; not wholly approved
or implemented.

**Baseline:** Owner refinement on 2026-11-19, from product revision `f4d6148`.
Documentation-only decision record; no management implementation inspected or
created because the product still contains only the runnable scaffold.

## Purpose And Scope

Separate project inclusion from permission to change a project. Requested
discovery uses the selected root; individual folder selection is another entry
path. Discovery-added stacks require activation; explicit manual addition and
new Git-stack creation establish active membership immediately. Active status
grants eligibility for management, not proof of runtime identity or permission to ignore action
policy. This contract does not define Compose inputs, naming, filesystem ownership,
specific action effects, automation schedules or plugin migration.

## Ownership

PROJ owns project membership and the activation prerequisite. Future setup,
discovery and folder-selection surfaces consume PROJ-001 and PROJ-004 through
PROJ-007; every manager-initiated project mutation,
whether manual or automatic, consumes PROJ-002. Activation must not become a
browser-only affordance while another manager entry point bypasses the policy.
Activation surfaces consume PROJ-003's permission-only effects.
Explicit-add and Git-creation surfaces consume PROJ-008; PROJ-009 rules out a
deactivate action without defining removal or deletion.
No package, API, persistence format or lock mechanism is selected here.

Discovery readers, action submitters, background executors and activation-state
writers will need one consistent policy. Persisted identity, restart/recreation,
in-flight work and publication/acknowledgement boundaries still need
refinement before implementation. External Docker/Compose clients do not become
participants in manager policy merely because a project is listed.

## Requirements

### PROJ-001: Automatic Listing Within The Selected Root

Projects discovered beneath the selected project root appear automatically,
without a separate per-project import or adoption confirmation merely to list
them. This also applies when projects are added there outside the UI. Automatic
listing applies when discovery is explicitly requested under PROJ-006, not as a
promise of continuous scanning. It does not activate the project or authorize
management changes. PROJ-007 separately allows one-at-a-time folder selection.

**Approval:** Approved by the owner on 2026-11-19. PROJ-004 settles depth,
PROJ-005 eligibility at a high level, and PROJ-006 the discovery trigger.
Initial recognized filenames are bounded under PROJ-005; missing/invalid-file
handling remains open.

**Source provenance:** Source STK-LIFE-01/18 distinguish directory discovery and
membership. The standalone owner selected automatic inclusion after discussing
a setup-selected root, rather than the earlier explicit-registration proposal.
See the [reuse map](../planning/contract-reuse.md).

**Enforcement:** Not implemented. No discovery, registry or activation store exists.

### PROJ-002: Activation Before Managing A Discovery-Added Stack

A newly discovered project is listed but inactive. Until explicitly activated,
the manager may expose inspection, but must not perform project-changing actions
either manually or automatically. For example, manual Stop is unavailable before
activation, as are automatic updates. Discovery must not implicitly activate an
already-running workload or stop it because it is inactive in this manager.

Activation is a prerequisite, not a definition of which actions or automations
the product supports. PROJ-003 defines activation's effects. PROJ-008 makes
explicitly added/newly cloned stacks active from creation, without this extra
gate; PROJ-009 excludes a deactivate action. Automation opt-ins remain separate.

**Approval:** Owner explicitly selected option A on 2026-11-19: activation gates
all changes, not just automation. This is a standalone decision, not a claim
that the plugin already implements this distinction. The owner subsequently
clarified on the same date that this gate applies only to discovery-added stacks,
not explicit manual additions or creation of new Git stacks.

**Source provenance:** Standalone owner review recorded in the
[source register](../planning/sources.md#standalone-owner-refinement).

**Enforcement:** Not implemented. Runtime identity, action authorization and
coordination dependencies remain unresolved and separately required.

### PROJ-003: Activation Grants Permission Without Applying Changes

Activation changes management eligibility only. It must not run Compose,
reconcile or recreate containers, start or stop the workload, rewrite project
files, or enable schedules. Management actions and any future automation opt-ins
remain separate deliberate choices. A running workload remains untouched; an
inactive stopped workload is not started merely by activation.

Recording the user's activation choice in manager state is intrinsic to the
permission change; this requirement does not select a persistence mechanism.
Activation still cannot bypass PROJ-002's independent action prerequisites.

**Approval:** Owner explicitly selected permission-only option A on 2026-11-19,
emphasizing that users should not encounter unexpected automatic actions.
This settles activation effects, not every future automation policy.

**Source provenance:** Standalone owner review recorded in the
[source register](../planning/sources.md#standalone-owner-refinement).

**Enforcement:** Not implemented. Activation validation, persistence and failure
reporting remain to be defined and verified.

### PROJ-004: Direct-Child Project Discovery

Requested project-folder discovery considers immediate child directories of the
selected project root, not recursively nested project folders. Given root
`/srv/compose`, an eligible project in `/srv/compose/jellyfin` is in scope;
`/srv/compose/media/sonarr` is not automatically discovered as a nested project.

PROJ-005 excludes ordinary non-project directories and PROJ-006 defines the
trigger. For the initial scan, skip symlinked child directories and clearly
report that limitation. This does not decide symlinks within a project, root
symlinks, grouping presentation or the folder picker's permitted paths.

**Approval:** Owner explicitly selected direct-child option A on 2026-11-19
for the current scope. Recursive discovery and UI grouping were discussed as
possible future ideas, not approved requirements or permanently prohibited ones.
On 2026-11-21, the owner selected skipping and reporting symlinked child
directories for the first scan.

**Source provenance:** Source STK-LIFE-01 observes immediate-directory discovery;
the standalone owner independently approved the bounded depth choice. See the
[source register](../planning/sources.md#standalone-owner-refinement).

**Enforcement:** Not implemented. Detailed eligibility and traversal/path safety remain
unresolved; this approval does not establish a safe filesystem scanning algorithm.

### PROJ-005: Discover Compose Projects Rather Than Every Directory

Discovery lists folders with recognized Compose inputs, rather than turning
every immediate child directory into a project row. Ordinary data, backup or
grouping directories without recognized Compose inputs are not included solely
because they are beneath the selected root.

**Approval:** Owner selected option A on 2026-11-19. On 2026-11-21, the owner
selected standard Compose filenames for initial discovery: `compose.yaml`,
`compose.yml`, `docker-compose.yml`, and `docker-compose.yaml`. Discovering a
folder solely because `.env` sets `COMPOSE_FILE` is deferred. This is a discovery
eligibility boundary, not a change to native input resolution for a selected
project under RES-005/006. The owner also selected listing a newly discovered
folder with malformed or unresolvable Compose inputs as inactive, with a clear
resolution error, instead of hiding it. Recognition and successful resolution
are separate. Other symlink handling, UI-created empty projects, custom inputs and other
already-listed project lifecycle cases remain unresolved. PROJ-010 covers an
absent previously discovered project.

**Source provenance:** Standalone owner decision, deliberately narrower than
source STK-LIFE-01/18's directory-based plugin membership. See the
[source register](../planning/sources.md#standalone-owner-refinement).

**Enforcement:** Not implemented; input recognition and validation need refinement.

### PROJ-006: Discovery Is Requested, Not Continuous

Discovery is an on-demand user action and may optionally be run during initial
setup. Setup must not require discovery; ongoing automatic/background project
discovery is not part of this approved flow. A project added outside the UI is
found on a subsequent requested scan, not by an implicit continuous scan.

This controls discovery triggers, not runtime status refresh, scans' internal
execution model or resource limits. A requested scan consumes PROJ-001's automatic
listing rule and does not activate its results.

**Approval:** Owner specified on-demand discovery or optional setup discovery on
2026-11-19, alongside individual folder selection.

**Source provenance:** Direct standalone owner refinement; not adopted from
plugin refresh/scanner behavior.

**Enforcement:** Not implemented. Scan error handling and interaction remain open.

### PROJ-007: Individual Folder Selection Without A Discovery Scan

Users can browse for a stack folder and select projects one at a time without
first running a discovery scan. A successful explicit addition starts active
under PROJ-008, without a separate activation step. Active status alone does not
request deployment or changes to its Compose inputs; other action prerequisites
and explicit automation choices still apply.

**Approval:** Owner requested the one-at-a-time folder-browsing flow on
2026-11-19. Allowed locations, outside-root support, validation and duplicate
handling remain unresolved, as does activation-state treatment for reselection
of an already-known project. The earlier wording that manual additions did not
activate a stack is superseded by the owner's explicit clarification in PROJ-008.

**Source provenance:** Direct standalone owner refinement. The root scan and
folder picker are distinct entry paths, not competing activation policies.

**Enforcement:** Not implemented. No picker, registration storage or path policy
has been added; browser-local paths are not assumed to be daemon-host paths.

### PROJ-008: Explicitly Added Stacks Start Active

Successful manual addition of a new stack and successful creation of a new Git
stack by cloning a repository establish active membership from the start.
Neither flow requires a second activation action. The inactive-until-activated
gate in PROJ-002 is for stacks added through discovery only.

Explicit Git creation necessarily acquires repository files as part of the user's
requested creation operation; it is not an implicit effect of activating a
discovery result. Active status by itself requests no deployment or automation
enablement. Any combined creation/deployment flow needs separate refinement,
as do failed/partial creation and duplicate-add behavior.

**Approval:** Owner explicitly clarified on 2026-11-19 that cloned new Git stacks
and manually added stacks start active, unlike discovery-added stacks.

**Source provenance:** Direct standalone owner clarification; supersedes the
earlier generalization of the discovery activation gate to every entry path.

**Enforcement:** Not implemented. Creation, membership persistence and partial
failure semantics remain unresolved.

### PROJ-009: No Deactivate Action

Once a stack is active, there is no deactivate option that returns it to an
inactive/discovery-only state. This applies both to activated discovery results
and stacks created active through PROJ-008.

This requirement does not define unregister/delete behavior, per-feature
automation controls, or which invalid-input conditions block otherwise active
stacks. Those are not automatically equivalent to deactivation.

**Approval:** Owner explicitly stated on 2026-11-19 that there is no deactivate
option after activation.

**Source provenance:** Direct standalone owner clarification, replacing the
previous open question about a possible deactivation workflow.

**Enforcement:** Not implemented. No activation/deactivation controls exist.

### PROJ-010: Retain Previously Discovered Projects That Disappear

When a previously discovered project is no longer found because its folder or
recognized Compose file has disappeared before a successful scan, retain its
saved listing and activation state and mark it unavailable. A failed scan is
not proof that projects were removed.

**Approval:** Owner selected this behavior on 2026-11-21 for persistent
discovery listing, rather than removing the entry after a successful scan.
This does not decide explicit removal, identity after replacement or recreation,
partial scan handling, validation or how unavailable entries recover.

**Source provenance:** Direct standalone owner decision, not adoption of plugin
removal or deletion semantics.

**Enforcement:** Not implemented or verified.

### PROJ-011: One Saved Discovery Root Initially

The initial persistent discovery implementation supports one saved discovery
root, entered as an application-visible path in the authenticated UI. For a
container deployment, the administrator configures a read-only bind mount
separately. Changing the setting does not itself remove existing project entries.
Multiple roots, duplicate paths, root-change reconciliation and detailed
path-access policy remain separate decisions.

**Approval:** Owner selected one saved root for the prototype and authenticated
UI path entry on 2026-11-21.

**Source provenance:** Direct standalone owner decision, not an adoption of
plugin storage or path policy.

**Enforcement:** Not implemented or verified.

## Dependencies

- Depends on [RES](project-resolution.md) before actions: listing and active state
  cannot establish a safe target. RES has approved native naming/selection and
  adopted portable input rules; identity adoption and env/profile details remain
  unresolved, not proof of safe execution.
- Consumed by [LIVE](live-state.md) for membership/active-state presentation;
  runtime refresh is not automatic discovery or activation.
- Depends on [OPS-001](operation-coordination.md) for bounded queueing and
  serialization of same-stack mutations. Activation-state lifecycle rules,
  including removal/recreation and queued/running actions, need further review.
  PROJ-009 excludes a deactivate action; it does not settle those other transitions.
- Consumed by proposed setup/discovery and manual/automatic project workflows in
  the [product plan](../planning/README.md). Those consumers reference PROJ rather
  than defining another meaning of activation.
- Standalone network/volume actions, shared-resource ownership and external
  clients are not granted project-management permission by this contract.
- Consumed by [GIT-001](git-stacks.md) for PROJ-008's active-on-creation behavior,
  PROJ-002/003's discovery-only activation gate and effects, and PROJ-009's absence
  of a deactivate action. Git callers must still honor separate action policies.

## Acceptance Cases

These are agreed-outcome scenarios, not tests already implemented or executed.

- PROJ-001: An eligible existing project beneath the selected root appears
  without per-project adoption; a later externally added project also appears
  when the user requests another discovery scan under PROJ-006.
- PROJ-001/002: A running project is discovered and listed inactive. Listing
  does not alter its files, stop its containers or trigger an update.
- PROJ-002: Manual Stop of an inactive project is refused without calling the
  operation executor, including direct submissions that bypass the browser UI.
- PROJ-002: An automatic update must not execute against an inactive project.
  This acceptance case does not approve implementing automatic updates.
- PROJ-002: Satisfying activation alone does not bypass independently required
  identity, input, authorization or coordination checks for an action.
- PROJ-003: Activate an already-running project; management eligibility changes
  without Compose execution, container changes or project-file rewrites.
- PROJ-003: Activate a stopped project; it remains stopped until a separate
  deliberate action starts it.
- PROJ-003: Activation does not enable schedules or trigger automatic updates.
- PROJ-004: With root `/srv/compose`, discover eligible project folder
  `/srv/compose/jellyfin`, but do not recursively discover the separate nested
  project `/srv/compose/media/sonarr`. Do not infer that the grouping directory
  `/srv/compose/media` itself meets project eligibility rules.
- PROJ-004: A symlinked direct child directory is skipped and the scan reports
  the limitation; it is not followed even if its target is outside the root.
- PROJ-005: A requested scan lists `jellyfin/compose.yaml` as a detected project,
  not sibling `appdata/` or `backups/` directories without recognized Compose
  inputs. The initial recognized names also include `compose.yml`,
  `docker-compose.yml`, and `docker-compose.yaml`; `.env`-only `COMPOSE_FILE`
  discovery is deferred. Path handling remains a separate question.
- PROJ-005: A newly discovered folder with a standard Compose filename and
  malformed or unresolvable inputs is listed inactive with a clear error.
- PROJ-006: Complete setup without running discovery, then request it later.
  Adding a folder on disk alone does not trigger a background discovery scan.
- PROJ-007: Without scanning a root, browse for one eligible stack folder and
  explicitly add it as active without a second activation step. Active status
  alone does not deploy it. Exact path eligibility
  is a dependency, not an already-approved arbitrary-path guarantee.
- PROJ-008: Successfully create a new Git stack by cloning a repository; the
  resulting stack is active without a separate activation click. Source
  acquisition does not by itself approve a deployment or CI automation policy.
- PROJ-009: An active stack offers no deactivate action in UI or other supported
  manager entry points. Removal/deletion behavior is not inferred from this case.
- PROJ-010: A previously discovered and activated project whose folder or
  recognized file is absent on a successful scan remains listed as unavailable
  with activation state preserved. A failed scan is not evidence of removal.
- PROJ-011: The initial discovery root is saved, and changing it does not itself
  remove previously listed project entries.
- PROJ-011: An authenticated user can enter the application-visible root path;
  for a container, access depends on an independently configured bind mount.

## Decisions

- 2026-11-19: Owner chose automatic inclusion, with activation separating listing
  from management. Earlier explicit adoption/registration was a proposal, now
  superseded for ordinary listing beneath the selected root.
- 2026-11-19: Owner selected activation option A over allowing manual changes to
  inactive projects, explaining that it keeps the distinction cleaner.
- 2026-11-19: In the separate activation-effects scenario, owner selected option A
  over activation-plus-reconciliation: permission only, without Compose execution,
  project-file rewrites or schedule enablement. Rationale: no unexpected automatic
  actions. PROJ-003 records this bounded approval.
- 2026-11-19: Owner selected direct-child discovery for now (PROJ-004), while
  expressing interest in possible recursive discovery/UI grouping. No grouping
  behavior or recursive scope was approved. Organizational grouping does not
  itself require independent projects to share a Compose file or lifecycle.
- 2026-11-21: Owner selected skipping symlinked child directories in the first
  scan, with the limitation reported. Other symlink cases remain open.
- 2026-11-19: Owner selected recognized Compose projects only (PROJ-005), rather
  than every child directory. This approval was pending documentation at the next
  message and is recorded here with its original scope.
- 2026-11-21: Owner selected the four standard Compose filenames as the initial
  discovery recognition boundary, deferring `.env`-only `COMPOSE_FILE` discovery.
  This does not disable native `COMPOSE_FILE` resolution for selected projects
  or settle malformed-file and path-safety behavior.
- 2026-11-21: Owner selected listing newly discovered folders whose Compose
  inputs are malformed or unresolvable as inactive with a clear error; this
  does not decide the remaining input resolution or path policies.
- 2026-11-19: Owner added on-demand/optional-setup discovery (PROJ-006) and
  individual folder browsing (PROJ-007). PROJ-001 retains automatic inclusion
  when a requested scan runs; no continuous discovery is implied. PROJ-004's
  depth limit applies to scans, not an unreviewed restriction on picker paths.
- 2026-11-19: Owner clarified that only discovery-added stacks need activation;
  manual additions and cloned new Git stacks start active (PROJ-008). The earlier
  PROJ-007 statement that manual selection did not activate a stack is superseded,
  and PROJ-002's scope is narrowed explicitly rather than silently generalized.
- 2026-11-19: Owner ruled out a deactivate option (PROJ-009). This settles the
  earlier deactivation question, not removal/deletion or CI deployment approval.
- 2026-11-21: Owner selected persistent listing rather than a scan-only preview
  for the next slice, and chose to retain an absent previously discovered entry
  as unavailable with its activation state intact (PROJ-010). Other state and
  path rules remain to be decided before implementation.
- 2026-11-21: Owner selected one saved discovery root initially (PROJ-011);
  changing the setting does not itself remove existing project entries. The
  root is entered in the authenticated UI using its application-visible path;
  an administrator separately configures any container bind mount.
- Open: Root-change reconciliation, candidate-path suggestions, running-container evidence,
  remaining discovery eligibility, other symlinks, filesystem/path permissions
  and scan error/results presentation. Runtime refresh is a separate boundary.
  Handling a Compose file directly in the root remains open.
- Open: UI-created projects, outside-root projects, empty/invalid directories,
  duplicate runtime identities, activation validation/confirmation, persistence,
  queued/running work, failed/partial creation and deletion/recreation semantics.
- Architecture planning prioritizes maintainability and clear ownership, not a
  plugin rebuild or shared-backend product goal; see the
  [planning direction](../planning/README.md#maintainability-and-ownership-direction).
  This does not change PROJ's approved scope or authorize runtime work.

## Verification

No management behavior or SDK integration exists; no behavioral acceptance tests,
native-host measurements or plugin tests were run for this decision record.
Document checks can validate structure, IDs and links, not enforcement or approval.
The whole draft remains unapproved despite its eleven individually approved entries.