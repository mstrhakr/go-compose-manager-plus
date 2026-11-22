+++
title = 'Dashboard And Live State'
weight = 8
+++
# LIVE: Dashboard And Live State

**Status:** Draft / prototype scope and SSE preference recorded; detailed live
state, recovery and metrics policies proposed

**Baseline:** 2026-11-19, product `68ab9d2`, documentation-only planning. Plugin
runtime-plan passages and metrics-publisher boundaries were inspected, not
adopted wholesale or reproduced on an Unraid host.

## Purpose And Scope

Provide a dashboard home with live project/container/operation status and resource
visibility within an agreed measurement budget. UniFi is the owner's design and
interaction inspiration, not a request to copy assets or architecture. Historical
tracing is later scope, not a prerequisite for the prototype.

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

**Approval:** Owner approved the bounded read-only status slice and clarified
on 2026-11-22 that same-name copies should use the Compose runtime name without
special adoption handling for this view. Activation remains a separate
management permission, not evidence of filesystem ownership.

**Source provenance:** Compose Specification `02-model.md` at `3a4a699`
defines project-name resource grouping and its label. Source STK-ID distinguishes
runtime names and evidence, but its saved-pin and migration policies are not
adopted. See [RES-001](project-resolution.md#res-001-accepted-runtime-identity-is-not-a-display-name).

**Enforcement:** Not yet implemented or verified.

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

Proposed cases requiring review and eventual execution-based checks:

- External lifecycle changes reconcile dashboard status independently of metrics.
- Missing metrics do not mean stopped containers or completed operations.
- A quiet healthy connection is not failed merely because no state event occurred.
- Bootstrap/reconnect obtain an authoritative snapshot rather than assuming no
  events were missed. Reject stale/out-of-order updates across views.
- Multiple tabs do not cause duplicate complete collector sets; connection
  recovery, hidden views and subscription cleanup respect measured resource bounds.
- Desktop/mobile and keyboard/touch retain readable status, usable actions and
  recoverable connection feedback. No exact widget/layout is approved yet.

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
- Later history: distinguish operation records, runtime transitions and metric
  samples, with identities/timestamps/outcomes and redaction boundaries. Retention,
  downsampling, incident thresholds and idle collection remain unapproved. No
  metrics database, event-sourced core or continuous no-viewer collector is
  mandated merely to preserve future options.

## Verification

Bounded source-plan/producer inspection and Engine API documentation research
only. No live stream, resource-overhead, browser recovery or tracing behavior was
implemented or tested. See the [source register](../planning/sources.md).