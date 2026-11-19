+++
title = 'Operation Coordination'
weight = 6
+++
# OPS: Operation Coordination

**Status:** Draft / bounded same-stack queueing approved; detailed queue and
lifecycle policies awaiting review

**Baseline:** Owner refinement on 2026-11-19, from product revision `7c42ef1`.
Documentation-only decision record; no operation executor or queue exists.

## Purpose And Scope

Coordinate competing manager-initiated mutations of the same stack, whether
requested manually or through CI. Separate accepting waiting work from owning
execution and from completing an operation. This does not define global
concurrency, cross-stack/shared-resource coordination or external-client safety.

## Ownership

OPS owns shared admission/waiting and execution coordination, not Git deployment
sequencing, lifecycle-action effects, activation, credentials or UI layout. Manual
and CI callers consume one policy instead of implementing competing queues.
No broker, database, package structure or persistence mechanism is selected here.

Before implementation, trace submitters, admission/capacity decisions, queue
writers/readers, executors, output/status publishers, completion/failure reporting,
registration identity changes, removal/recreation, cleanup and startup/shutdown.
External Docker/Compose clients do not automatically participate in this boundary.

## Requirements

### OPS-001: Bounded Queueing For Same-Stack Mutations

Only one manager mutation executes for a stack at a time. When another otherwise
eligible manual or CI mutation arrives for that same stack and waiting capacity
is available, accept it as queued work rather than refusing solely because the
stack is busy. Waiting capacity must be bounded; no unbounded backlog is approved.

Queued work is visibly distinguishable from running work and from completion.
The caller can learn that a request is waiting; queue acceptance must not claim
that the requested changes have executed or succeeded. Queueing alone does not
authorize overlapping preparation or source changes to the same stack.

This settles bounded queueing instead of immediate busy refusal, not queue size,
ordering, replacement/coalescing, cancellation, expiry, full-capacity response,
predecessor-failure policy or persistence/restart recovery. Execution eligibility
must be defined at the execution boundary, not inferred forever from admission.
The exact revalidation/ownership protocol remains to refine.

**Approval:** Owner explicitly approved option B on 2026-11-19 for competing
same-stack manual and CI mutations, after reviewing busy refusal versus bounded,
visible queueing. Detailed queue policies were explicitly left for later review.

**Source provenance:** Source STATE-LOCK distinguishes project exclusion from
queue/admission ownership and documents mixed busy-refusal/waiting mechanisms.
Compose's inspected service-update settings do not prescribe manager queueing.
External queue/concurrency guidance and limitations are recorded under
[operation queue evidence](../planning/sources.md#operation-queue-evidence).

**Enforcement:** Not implemented. No runtime guarantee, queue durability,
ordering, exactly-once execution or complete participant coverage is claimed.

## Dependencies

- Depends on [PROJ-002/008/009](project-membership.md) for entry-path-specific
  active membership and the absence of a deactivate action. Identity/removal
  lifetime and queued-work handling still require connected review.
- Depends on [RES](project-resolution.md) for identity/inputs, whose identity
  acceptance remains proposed. Lifecycle-action contracts still need adoption;
  a folder/display name is not proof of a safe runtime target.
- Consumed by [LIVE](live-state.md) for queued/executing presentation. Metrics and
  viewer transport are not execution ownership or completion evidence.
- Consumed by [GIT-002/003/005](git-stacks.md): CI deployment is unattended after
  explicit enablement/request and retains its selected Git commit while waiting.
  Queue policy cannot silently replace the commit within an accepted request.
- Consumed by proposed manual lifecycle mutations in the
  [product plan](../planning/README.md#safety-and-ownership-before-execution).
  Shared-resource operations and external clients need separate participation
  analysis; this is not a whole-host or every-path coordination guarantee.

## Acceptance Cases

Owner-approved outcome scenarios, not implemented or executed tests:

- A stack mutation is running; another eligible manual or CI mutation is
  admitted with waiting capacity available. It is visibly queued, not refused
  merely as busy, and does not execute concurrently with the first mutation.
- A queued deployment still targets its requested Git commit under GIT-003;
  waiting does not authorize retargeting it to a newer branch head.
- A waiting receipt or queue status is not reported as running or successful
  completion of the requested operation.

Pending scenarios include full capacity, concurrent admissions, multiple pending
requests, predecessor failure, cancellation/expiry, removed/recreated stacks,
changed inputs, lost credentials and server interruption/restart. None has an
approved detailed outcome from choosing queueing alone.

## Decisions

- 2026-11-19: The initial recommendation favored busy refusal to reduce initial
  implementation complexity. After scenarios/source review, the recommendation
  changed to bounded queueing for this unattended-CI product: refusal moves retry
  responsibility to each caller, while queues absorb temporary contention.
- 2026-11-19: Owner approved B, bounded same-stack queueing, for manual and CI
  mutations. This is a standalone decision, not adoption of every plugin lock
  timeout or GitHub pending-run replacement behavior.
- Open: Numerical capacity, ordering/coalescing, fairness, cancellation, expiry,
  full-queue behavior, predecessor failure, durability/restart and resource limits.
- Open: Exact stack/runtime identity domain, safe execution-time revalidation,
  source/configuration changes, shared resources and lifecycle cleanup.

## Verification

No operation queue, management behavior or concurrency acceptance tests exist.
Source documentation was inspected, not plugin enforcement reproduced. Document
checks validate structure, IDs and references, not fairness, durability, safe
execution, cross-client coordination or fulfillment of this draft.