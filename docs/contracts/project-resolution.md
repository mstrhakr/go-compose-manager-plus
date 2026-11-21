+++
title = 'Project Identity And Inputs'
weight = 7
+++
# RES: Project Identity And Inputs

**Status:** Draft / RES-002 through RES-005 adopted as portable approved-source
outcomes; RES-001 native naming and RES-006 native prototype selection approved;
identity/adoption edge cases and env/profile details awaiting review

**Baseline:** 2026-11-19, product `68ab9d2`; source plugin
`10e9179cbb0331fb956a0b60ba671e477a74638e`. Source worktree has unrelated
documentation/e2e changes; the affected source contract was not locally modified.
Documentation-only adoption, not implementation or verification.

## Purpose And Scope

Resolve a local project's identity, location and inputs once for inspection and
actions. Reuse approved plugin outcomes without importing Unraid paths, legacy
migration, Basic/Advanced tiers or PHP/shell mechanisms.

## Ownership

RES owns the accepted project description, separate from PROJ membership, action
effects, OPS execution ownership and LIVE presentation. Native SDK loading should
implement Compose semantics, not a second manager-owned YAML engine. Actual SDK
support needs verification. The owner's reuse direction authorizes selected
portable approved rules, not whole source drafts or implementation defects.

## Requirements

### RES-001: Accepted Runtime Identity Is Not A Display Name

The UI name is display-only, independent of the runtime Compose project. Changing
it must not rename directories, rewrite Compose inputs or retarget Docker
operations. Do not apply a manager-specific Compose-name normalization scheme to
the display label.

Runtime naming follows native Compose resolution: use the Compose configuration's
top-level `name` where applicable, otherwise native defaults. The prototype has
no separate manager project-name setting, external project-name flag/override,
or manager-authored naming env override. Do not automatically rewrite the Compose
file to insert a name. Passing the natively resolved name to an SDK operation for
targeting is not a separate user-configured naming policy.

**Approval:** Owner approved display-only UI naming and clarified on 2026-11-19
that explicit naming belongs in Compose configuration, not a manager setting.
Support for independently supplied project-name flags is not required for the
prototype. This replaces the earlier optional-manager-override proposal, which
was not approved.

**Source provenance:** Source STK-ID-01/07/11 remain proposed source guarantees;
this naming decision is direct standalone approval, not adoption of legacy pins,
receipts or migration heuristics. Spec `04-version-and-name.md` defines native
top-level naming and default behavior, not manager identity-adoption policy.

**Still unresolved:** safe adoption of existing deployments launched under an
external custom name, collisions, changing native identity after registration,
and the process-environment/SDK-resolution boundary. No automatic guessing,
pinning, alias migration or takeover workflow is approved here.

### RES-002: Source Location And Input Selection Are Separate

A source directory is a location, not a separate Compose input-selection
algorithm. Keep location separate from native discovery versus deliberately
selected files. Selecting an exact Compose file is explicit input selection.

**Approval:** Portable outcome adopted under owner reuse authorization,
2026-11-19, from individually approved STK-FILES-14. Its separation principle
survives the reopened source exact-list design; no replacement UI is adopted.

### RES-003: Source Reference Changes Do Not Imply File Changes

Choosing or changing source location alone changes a reference, without copying,
moving, deleting or otherwise editing authored files. File operations require
explicit approval of intended operation/scope, which may be part of the same
workflow. Deliberate editor saves already authorize their disclosed target writes;
no extra confirmation for every low-level write is implied.

**Approval:** Portable outcome adopted under owner reuse authorization,
2026-11-19, from STK-FILES-18 (approved 2026-11-11, clarified 2026-11-13).
No source-transfer UI, overwrite policy or import mechanism is selected.

### RES-004: Missing Explicit Interpolation Env Blocks Execution

If an explicitly selected interpolation env file is missing, block affected
execution with an explicit error and preserve the selection for repair or a
deliberate change. Do not silently substitute a local `.env` file.

**Approval:** Portable outcome adopted under owner reuse authorization,
2026-11-19, from approved STK-FILES-13. This does not decide absent optional
defaults, missing extra YAML inputs or editor capabilities.

### RES-005: Interpolation Env Does Not Change File-Selection Mode

Interpolation env selection is independent of native versus explicit Compose-file
selection. Custom interpolation env must not silently disable native discovery.
Without explicit Compose files, honor applicable native `COMPOSE_FILE`; explicit
file selection retains native precedence instead of appending env-selected files.
Service-level `env_file` is a separate concern.

**Approval:** Portable outcome adopted under owner reuse authorization,
2026-11-19, from approved STK-FILES-19. Multiple env files, process-environment
sourcing and exact SDK options remain to review/verify; no Git-specific env
precedence is inherited.

### RES-006: Native Compose Inputs By Default

Resolve local projects using native discovery/filename precedence, merge/include
and interpolation semantics. Do not freeze native discovery into a generated
explicit list or silently append extra inputs. The prototype does not provide a
manager UI for selecting an ordered list of Compose files; those controls are
deferred rather than an implementation prerequisite.

Deferring those controls does not mean ignoring native input mechanisms such as
applicable `COMPOSE_FILE` selection under RES-005. Where an input arrangement is
not safely supported, report that limitation rather than guess another config.
Exact interpolation-env/profile controls and SDK option mapping remain separate.

**Approval:** Owner selected native-selection option A on 2026-11-19 over adding
explicit ordered-file controls immediately. This settles the prototype's default
and manager-control scope, not every env/profile behavior or compatibility case.

**Source provenance:** Source STK-FILES-15 approves native defaults with qualified
plugin tier/settings behavior; that entire feature set is not copied. Spec text,
CLI precedence and actual SDK support are distinct evidence; implementation
equivalence still requires executed checks.

## Dependencies

- Consumed by [PROJ](project-membership.md) for identifying selected projects;
  membership and active state stay PROJ-owned.
- Consumed by [OPS-001](operation-coordination.md) for target/input descriptions
  and execution-time revalidation. Preview is not an immutable source snapshot.
- Consumed by proposed local actions and the [LIVE dashboard](live-state.md).
  Action/update effects are not owned by resolution.
- Also consumed by [GIT](git-stacks.md) for shared identity/input outcomes;
  future Git acquisition/revision/credential policy stays with GIT. No Git
  default, source ownership or plugin migration compatibility is adopted here.

## Acceptance Cases

Adopted outcomes, not yet implemented as product behavior:

- RES-001: UI display name is `Home Entertainment`, while Compose `name` is
  `media`; changing the UI label alone preserves runtime targeting and source
  bytes. No manager project-name override is offered or synthesized.
- RES-001: Without an applicable explicit native name, resolve Compose's native
  fallback rather than derive a runtime override from the UI label. Exact SDK
  equivalence still needs tests.
- RES-002: Changing source location does not itself change selection algorithm.
- RES-003: Selecting an existing folder leaves authored bytes unchanged absent
  an explicitly requested file operation.
- RES-004: Missing selected interpolation env causes refusal, not `.env` fallback.
- RES-005: Custom env does not disable native selection; explicit Compose files
  do not acquire extra files from `COMPOSE_FILE`.
- RES-006: A standard Compose project resolves using native file-selection and
  merge behavior without a manager-authored file list. Its dashboard and actions
  use the same resolved inputs; exact SDK parity needs execution-based fixtures.
- RES-006: The prototype offers no ordered-file selector. Unsupported arrangements
  produce an explicit limitation, not fallback to a different project config or
  suppression of applicable native `COMPOSE_FILE` behavior.

Pending: identity ambiguity/collision, native-name changes, overrides/path anchors,
include/environment resolution, profiles, changed queued inputs and host paths.

## Decisions

- 2026-11-19: Owner authorized source-first reuse and requested review of material
  adaptations, not repeated interviews for portable approved clauses. RES-002
  through RES-005 retain those outcomes and their source qualifications.
- 2026-11-19: Owner approved native/file-based runtime naming and display-only UI
  labels, explicitly excluding a separate manager name override for the prototype
  (RES-001). Existing external-name adoption and native-name changes remain open.
- 2026-11-19: Owner selected native input selection first, deferring manager
  ordered-file controls (RES-006). Env sourcing, profiles, SDK support and identity
  adoption/retargeting edge cases remain open.
- No source runtime code or tests were imported, and no management
  implementation is authorized by this record.

## Verification

Source approval text and local Compose chapters were read. A separately approved
test-only proof of concept exercises selected resolution cases using the Compose
model loader, not product callers or Docker operations. These fixtures provide
partial evidence for RES-001 and RES-004 through RES-006, plus unchanged fixture
files, but do not establish any requirement's end-to-end compliance. RES-002 and
RES-003 source-reference workflows are not implemented or verified. See
[verification](../verification.md) for exact executed cases and limitations and
the [source register](../planning/sources.md) for provenance.