+++
title = 'Project Membership And Activation'
weight = 4
+++
# PROJ: Project Membership And Activation

**Status:** Draft / two individually approved requirements; not wholly approved
or implemented.

**Baseline:** Owner refinement on 2026-11-19, from product revision `f4d6148`.
Documentation-only decision record; no management implementation inspected or
created because the product still contains only the runnable scaffold.

## Purpose And Scope

Separate automatic project visibility from permission to change a project.
The selected project root supplies discovery scope; activation grants eligibility
for management, not proof of runtime identity or permission to ignore action
policy. This contract does not define Compose inputs, naming, filesystem ownership,
specific action effects, automation schedules or plugin migration.

## Ownership

PROJ owns project membership and the activation prerequisite. Future setup and
discovery surfaces consume PROJ-001; every manager-initiated project mutation,
whether manual or automatic, consumes PROJ-002. Activation must not become a
browser-only affordance while another manager entry point bypasses the policy.
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
listing does not activate the project or authorize management changes.

**Approval:** Approved by the owner on 2026-11-19. Exact discovery eligibility,
scan depth, refresh cadence and treatment of missing/invalid files remain open.

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
the product supports. Effects of the activation action itself, subsequent
automation opt-ins and deactivation are unresolved below.

**Approval:** Owner explicitly selected option A on 2026-11-19: activation gates
all changes, not just automation. This is a standalone decision, not a claim
that the plugin already implements this distinction.

**Source provenance:** Standalone owner review recorded in the
[source register](../planning/sources.md#standalone-owner-refinement).

**Enforcement:** Not implemented. Runtime identity, action authorization and
coordination dependencies remain unresolved and separately required.

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

## Acceptance Cases

These are agreed-outcome scenarios, not tests already implemented or executed.

- PROJ-001: An eligible existing project beneath the selected root appears
  without per-project adoption; a later externally added project also appears
  when discovery runs. The discovery trigger/cadence is still undecided.
- PROJ-001/002: A running project is discovered and listed inactive. Listing
  does not alter its files, stop its containers or trigger an update.
- PROJ-002: Manual Stop of an inactive project is refused without calling the
  operation executor, including direct submissions that bypass the browser UI.
- PROJ-002: An automatic update must not execute against an inactive project.
  This acceptance case does not approve implementing automatic updates.
- PROJ-002: Satisfying activation alone does not bypass independently required
  identity, input, authorization or coordination checks for an action.

## Decisions

- 2026-11-19: Owner chose automatic inclusion, with activation separating listing
  from management. Earlier explicit adoption/registration was a proposal, now
  superseded for ordinary listing beneath the selected root.
- 2026-11-19: Owner selected activation option A over allowing manual changes to
  inactive projects, explaining that it keeps the distinction cleaner.
- Open: Root count, candidate-path suggestions, running-container evidence,
  discovery eligibility/depth, filesystem/path permissions and refresh behavior.
- Open: UI-created projects, outside-root projects, empty/invalid directories,
  duplicate runtime identities, activation confirmation/effects, persistence,
  deactivation, queued/running work and deletion/recreation semantics.
- Architecture planning prioritizes maintainability and clear ownership, not a
  plugin rebuild or shared-backend product goal; see the
  [planning direction](../planning/README.md#maintainability-and-ownership-direction).
  This does not change PROJ's approved scope or authorize runtime work.

## Verification

No management behavior or SDK integration exists; no behavioral acceptance tests,
native-host measurements or plugin tests were run for this decision record.
Document checks can validate structure, IDs and links, not enforcement or approval.
The whole draft remains unapproved despite its two individually approved entries.