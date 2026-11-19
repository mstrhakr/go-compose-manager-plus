+++
title = 'Project Membership And Activation'
weight = 4
+++
# PROJ: Project Membership And Activation

**Status:** Draft / seven individually approved requirements; not wholly approved
or implemented.

**Baseline:** Owner refinement on 2026-11-19, from product revision `f4d6148`.
Documentation-only decision record; no management implementation inspected or
created because the product still contains only the runnable scaffold.

## Purpose And Scope

Separate project inclusion from permission to change a project. Requested
discovery uses the selected root; individual folder selection is another entry
path. Activation grants eligibility
for management, not proof of runtime identity or permission to ignore action
policy. This contract does not define Compose inputs, naming, filesystem ownership,
specific action effects, automation schedules or plugin migration.

## Ownership

PROJ owns project membership and the activation prerequisite. Future setup,
discovery and folder-selection surfaces consume PROJ-001 and PROJ-004 through
PROJ-007; every manager-initiated project mutation,
whether manual or automatic, consumes PROJ-002. Activation must not become a
browser-only affordance while another manager entry point bypasses the policy.
Activation surfaces consume PROJ-003's permission-only effects.
No package, API, persistence format or lock mechanism is selected here.

Discovery readers, action submitters, background executors and activation-state
writers will need one consistent policy. Persisted identity, restart/recreation,
deactivation, in-flight work and publication/acknowledgement boundaries still need
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
Exact filenames and missing/invalid-file handling remain open.

**Source provenance:** Source STK-LIFE-01/18 distinguish directory discovery and
membership. The standalone owner selected automatic inclusion after discussing
a setup-selected root, rather than the earlier explicit-registration proposal.
See the [reuse map](../planning/contract-reuse.md).

**Enforcement:** Not implemented. No discovery, registry or activation store exists.

### PROJ-002: Activation Before Any Management Change

A newly discovered project is listed but inactive. Until explicitly activated,
the manager may expose inspection, but must not perform project-changing actions
either manually or automatically. For example, manual Stop is unavailable before
activation, as are automatic updates. Discovery must not implicitly activate an
already-running workload or stop it because it is inactive in this manager.

Activation is a prerequisite, not a definition of which actions or automations
the product supports. PROJ-003 defines activation's effects; subsequent
automation opt-ins and deactivation remain unresolved below.

**Approval:** Owner explicitly selected option A on 2026-11-19: activation gates
all changes, not just automation. This is a standalone decision, not a claim
that the plugin already implements this distinction.

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

This settles depth only. PROJ-005 excludes ordinary non-project directories and
PROJ-006 defines the trigger. Filenames, symlink policy and grouping presentation
remain separate decisions; this depth rule does not decide the folder picker's
permitted paths.

**Approval:** Owner explicitly selected direct-child option A on 2026-11-19
for the current scope. Recursive discovery and UI grouping were discussed as
possible future ideas, not approved requirements or permanently prohibited ones.

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

**Approval:** Owner selected option A on 2026-11-19. This defines discovery
eligibility at a high level, not exact supported filenames or syntactic validity
rules. UI-created empty projects, custom inputs and already-listed projects whose
files disappear remain unresolved.

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
first running a discovery scan. This entry path adds a project for listing; it
does not implicitly activate it, deploy it or change its Compose inputs.
PROJ-002/003 remain authoritative for activation and permitted management effects.

**Approval:** Owner requested the one-at-a-time folder-browsing flow on
2026-11-19. Allowed locations, outside-root support, validation and duplicate
handling remain unresolved, as does activation-state treatment for reselection
of an already-known project.

**Source provenance:** Direct standalone owner refinement. The root scan and
folder picker are distinct entry paths, not competing activation policies.

**Enforcement:** Not implemented. No picker, registration storage or path policy
has been added; browser-local paths are not assumed to be daemon-host paths.

## Dependencies

- Depends on unextracted standalone identity/input contracts before any action:
  listing and activation cannot establish which runtime project is safe to target.
  Source STK-ID/STK-FILES are candidates, not automatically adopted requirements.
- Depends on future authoritative mutation coordination and activation-state
  lifecycle rules. Deactivation versus queued/running actions is not settled.
- Consumed by proposed setup/discovery and manual/automatic project workflows in
  the [product plan](../planning/README.md). Those consumers reference PROJ rather
  than defining another meaning of activation.
- Standalone network/volume actions, shared-resource ownership and external
  clients are not granted project-management permission by this contract.
- Consumed by [GIT-001](git-stacks.md) for PROJ-002/003's activation prerequisite
  and permission-only activation effects. Git acquisition/setup before activation
  needs an explicit decision; Git scope does not grant an implicit exception.

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
- PROJ-005: A requested scan lists `jellyfin/compose.yaml` as a detected project,
  not sibling `appdata/` or `backups/` directories without recognized Compose
  inputs. The example filename does not settle the complete recognition rules.
- PROJ-006: Complete setup without running discovery, then request it later.
  Adding a folder on disk alone does not trigger a background discovery scan.
- PROJ-007: Without scanning a root, browse for one eligible stack folder and
  add it for listing without deployment or activation. Exact path eligibility
  is a dependency, not an already-approved arbitrary-path guarantee.

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
- 2026-11-19: Owner selected recognized Compose projects only (PROJ-005), rather
  than every child directory. This approval was pending documentation at the next
  message and is recorded here with its original scope.
- 2026-11-19: Owner added on-demand/optional-setup discovery (PROJ-006) and
  individual folder browsing (PROJ-007). PROJ-001 retains automatic inclusion
  when a requested scan runs; no continuous discovery is implied. PROJ-004's
  depth limit applies to scans, not an unreviewed restriction on picker paths.
- Open: Root count, candidate-path suggestions, running-container evidence,
  exact discovery eligibility/filenames, symlinks, filesystem/path permissions
  and scan error/results presentation. Runtime refresh is a separate boundary.
  Handling a Compose file directly in the root remains open.
- Open: UI-created projects, outside-root projects, empty/invalid directories,
  duplicate runtime identities, activation validation/confirmation, persistence,
  deactivation, queued/running work and deletion/recreation semantics.
- Architecture planning prioritizes maintainability and clear ownership, not a
  plugin rebuild or shared-backend product goal; see the
  [planning direction](../planning/README.md#maintainability-and-ownership-direction).
  This does not change PROJ's approved scope or authorize runtime work.

## Verification

No management behavior or SDK integration exists; no behavioral acceptance tests,
native-host measurements or plugin tests were run for this decision record.
Document checks can validate structure, IDs and links, not enforcement or approval.
The whole draft remains unapproved despite its seven individually approved entries.