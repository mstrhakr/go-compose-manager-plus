+++
title = 'Git Stacks And Deployment'
weight = 5
+++
# GIT: Git Stacks And Deployment

**Status:** Draft / capability scope, unattended enabled-CI deployment and
CI-selected immutable Git revision, GitHub-first CI scope and explicit CI request approved;
remaining behavioral/security policies awaiting review

**Baseline:** Owner scope addition on 2026-11-19, from product revision `557b9ec`
with pending PROJ discovery refinements. No Git implementation or dependencies
have been added. Source-plugin Git sketches were inspected, not fully extracted
or behaviorally verified.

## Purpose And Scope

Make Git-backed stacks a first-class product workstream, including acquiring
repository content, authenticating to private repositories, and deployment
triggered from CI through webhooks or an equivalent deliberate deployment API.
This is not approval of every Git-provider integration, a general CI runner,
arbitrary remote commands, or the plugin's entire Git feature set.

## Ownership

GIT owns repository acquisition/update and deployment sequencing. It consumes
shared project identity, activation, action policy, job coordination and outcome
publication rather than implementing a second set for webhook callers.

Credential lifecycle is a separate shared responsibility: selection, scoped use,
storage, redaction, rotation/revocation and deletion. Repository authentication,
webhook authentication and image-registry credentials are distinct authorities;
supporting private Git access does not by itself approve a generic secret vault.
These credential guarantees still need standalone extraction and approval.

Before implementation, trace UI/CI submitters, webhook delivery/retry, credential
readers, fetch/checkout preparation, validation, operation execution, status
readers, failure publication, cleanup and deletion/recreation.
The external Git host and CI system do not participate in local manager locks.

## Requirements

### GIT-001: Git-Backed Stacks With Private Access And CI Deployment

The product scope includes pulling stack sources from Git repositories, supporting
credentials for private repositories, and CI-initiated deployment through webhook
or deployment-trigger integration. A repository update and a workload deployment
are distinct effects whose sequencing and authorization must be specified.

**Approval:** Owner explicitly requested these capabilities on 2026-11-19.
This is capability-scope approval only, not approval of a deploy-on-pull default,
credential mechanism, webhook protocol, revision policy or implementation.

**Source provenance:** Source GIT-LIFE, GIT-CHECK, GIT-DEPLOY, GIT-AUTO,
GIT-STORAGE, CRED-VAULT and CRED-GIT are evidence candidates in the
[reuse map](../planning/contract-reuse.md#git-and-deployment-candidates).
They are sketches, not adopted standalone guarantees. Webhook/CI scope comes
from the standalone owner request; no existing plugin webhook support is claimed.

**Enforcement:** Not implemented. No clone store, credential store, Git transport,
webhook endpoint or deployment executor exists.

### GIT-002: Enabled CI Deployment Requires No Per-Run UI Approval

For an active stack with CI deployment explicitly enabled, an authenticated,
authorized deployment trigger submits work through the shared deployment job
flow without waiting for another UI approval for that deployment. A required
per-delivery human approval is not the selected CI behavior.

Active membership alone does not enable CI integration. This approval does not
remove identity/input validation, action eligibility, coordination or security
checks, guarantee immediate execution when busy, or make receipt/acceptance equal
successful completion. Those policies and outcomes require separate refinement.

**Approval:** Owner explicitly selected option A on 2026-11-19: deployment without
further UI approval once CI deployment is enabled. Rationale: requiring manual
approval for each deployment defeats the intended unattended CI workflow.

**Source provenance:** Direct standalone owner decision, not an inferred plugin
automation default. See the [source register](../planning/sources.md#standalone-owner-refinement).

**Enforcement:** Not implemented. Trigger authentication/authorization, enablement
mechanism, permitted repository/ref validation, idempotency, detailed queue policy and
completion reporting remain unresolved. GIT-003 defines revision-selection intent.

### GIT-003: Deploy The Immutable Revision Selected By CI

CI deployment targets the immutable source commit selected by the authorized
CI/deployment request, not the latest branch head at execution time. If the
request selects commit X and the branch advances to Y, the job still targets X;
the manager must not silently replace it with Y.

Use the supported provider/event's existing revision metadata where applicable,
rather than assuming every webhook has an identical payload or interpreting a
branch/tag name as an immutable commit. The provider mapping and authorization
must identify the intended source repository and commit; an arbitrary event's
SHA is not automatically proof of what a workflow checked out or tested.

**Approval:** Owner selected exact-commit option A on 2026-11-19, emphasizing
that the CI integration should supply the revision using existing conventions.
Approval is for that outcome, not a bespoke webhook schema or every provider.

**Source provenance:** Direct owner decision plus bounded research of GitHub's
official REST schema and GitLab's pipeline-hook documentation, recorded under
[CI revision conventions](../planning/sources.md#ci-revision-conventions).

**Compose specification boundary:** The inspected build Git-context syntax and
service `deploy` settings do not define a manager webhook or CI-selected checkout
protocol. See the [bounded spec reading](../planning/sources.md#compose-specification-evidence).
GIT-003 is manager integration policy, not a claimed Compose requirement.

**Enforcement:** Not implemented. GIT-004 selects GitHub first; event mapping, missing or unusable
revision handling, allowed refs/repositories and validation rules remain open;
none permits a silent latest-head fallback. This requirement pins Git source,
not mutable image tags, external dependencies or overall build reproducibility.

### GIT-004: GitHub First For CI Deployment Integration

The initial CI/deployment integration targets GitHub. GitLab and other provider
integrations are outside that initial slice, not permanently excluded or promised
for a particular release. Keep GIT-002/003's shared deployment outcomes distinct
from provider-specific event handling rather than building multiple adapters now.

This selects the first CI integration, not a GitHub-only restriction on repository
hosting. It does not choose Git transport/authentication, the triggering GitHub
event, GitHub.com versus Enterprise support, or the inbound authentication scheme.

**Approval:** Owner selected GitHub-first option A on 2026-11-19, rather than
including GitHub and GitLab in the initial integration scope.

**Source provenance:** Direct standalone owner scope decision; provider revision
research identifies available fields, not a reason to require multiple providers.

**Enforcement:** Not implemented. Exact GitHub event/API contract and security
boundaries still require refinement. No adapter framework is authorized here.

### GIT-005: CI Explicitly Requests Deployment

CI decides when to promote its selected immutable commit and sends an explicit
deployment request. A workflow finishing successfully is not by itself a request
to deploy, even when that workflow belongs to the configured repository.

The requested deployment consumes GIT-002's unattended enabled-CI flow and
GIT-003's commit binding. This selects deployment intent, not a bespoke webhook
format or specific GitHub event/API. Authentication, repository/ref authorization
and action prerequisites remain required and separately specified.

**Approval:** Owner selected explicit-request option A on 2026-11-19 over having
the manager react to a configured workflow's success. CI retains control over
when its tested commit is promoted.

**Source provenance:** Direct standalone owner decision. Existing provider
conventions should still be used where applicable; no exact wire protocol was
approved by this choice.

**Enforcement:** Not implemented. Request mapping, trusted callers, delivery
validation, replay handling and job outcomes still need refinement.

## Dependencies

- Depends on [PROJ-002/003/008/009](project-membership.md): the activation gate is
  for discovery-added stacks; explicit creation of a new Git stack by cloning
  starts active. Activation itself does not run Compose or enable automation,
  and there is no deactivate action. Active status is not CI deployment opt-in
  or a substitute for shared identity, input and action checks.
- Depends on [RES](project-resolution.md) for shared identity/input outcomes;
  native naming/selection are approved for the local prototype, while adoption,
  env/profile details and Git-specific input policy remain
  unresolved. Lifecycle-action contracts still need adoption: the Git adapter
  cannot invent project names, files/env/profiles or pull/build/recreate effects.
- Depends on shared credentials, path/storage safety, authentication/authorization,
  and jobs/state-publication policies not yet approved here.
- Depends on [OPS-001](operation-coordination.md) for bounded queueing and
  serialization of same-stack mutations across manual and CI callers. Detailed
  admission, ordering, recovery and execution-time revalidation remain unresolved.
- Consumed by the [planned Git workstream](../planning/README.md#git-stacks-and-ci-deployment).
  Future UI and CI entry points must reference this owner rather than duplicate
  Git deployment sequencing.

## Acceptance Cases

Owner-approved outcome scenarios for GIT-002 through GIT-005, not implemented or executed tests:

- An active stack has CI deployment explicitly enabled; a valid authenticated,
  authorized trigger submits deployment without a per-run UI confirmation.
- Active membership alone does not enable the integration. GIT-002's unattended
  path requires CI deployment to have been explicitly enabled.
- Lack of a UI approval step does not waive independent action prerequisites or
  convert an accepted job into a claim of completed deployment.
- GIT-003: CI selects X; the configured branch advances to Y before execution.
  The deployment still targets X, not Y. Missing/unusable revision behavior needs
  refinement, but must not silently substitute the latest branch head.
- GIT-004: The first CI integration is defined and validated for its selected
  GitHub flow. That evidence does not claim GitLab, other provider or Enterprise
  support. Repository-host support remains separately specified.
- GIT-005: A workflow completes successfully but sends no deployment request;
  the manager does not deploy merely because the workflow succeeded.
- GIT-005: CI explicitly requests deployment of commit X; the enabled,
  authenticated and authorized flow uses GIT-002/003 without another UI approval.

Remaining candidate scenarios for refinement, not approved detailed outcomes:

- Acquire a public repository and a private repository using an explicitly
  assigned credential; choose repository revision and Compose subdirectory/inputs.
- Distinguish initial clone, fetch/check, source pull/checkout and deployment.
  Include invalid inputs, dirty local files, missing refs and credential failures.
- Receive a CI trigger and bind it to the intended project, allowed repository
  and revision; distinguish accepted work from completed deployment.
- Reject unauthenticated/unauthorized triggers and handle duplicate, replayed,
  delayed and out-of-order deliveries without inventing success or safe retries.
- Exercise inactive discovery-added projects, same-name recreation, simultaneous
  manual/CI actions, failed preparation, partial Compose effects and restart.
- Keep Git secrets and secret-bearing URLs out of ordinary output. Specify
  repository-host trust, request/network scope and secret lifecycle before use.

## Decisions

- 2026-11-19: Owner added full Git-stack support to product scope, specifically
  pulling from repositories, private-repository credentials and webhook/CI
  deployment options. The previous blanket deferral of Git deployment is replaced
  by this workstream, without expanding the first prototype automatically.
- 2026-11-19: Owner clarified that cloning a repository as a new Git stack starts
  active, with no second activation step or later deactivate option. PROJ-008/009
  own this policy; the former clone-before-activation question is resolved for
  new Git-stack creation. Partial failures and existing-checkout adoption remain
  separate. The subsequent CI approval-mode decision is recorded in GIT-002.
- 2026-11-19: Owner selected unattended enabled-CI deployment, rejecting per-run
  UI approval as defeating its purpose. GIT-002 records that conditional behavior;
  it does not enable CI for stacks automatically or settle credential/security,
  revision or concurrency policy.
- 2026-11-19: Owner selected the exact CI-selected commit (GIT-003), asking to use
  existing provider conventions rather than invent choices already supplied by
  CI. Provider research confirms available SHA/ref fields, not one universal
  deployment webhook format or proof that any event represents passing CI.
- 2026-11-19: Owner selected GitHub first for CI integration (GIT-004), with
  other providers possible later. This does not resolve repository-host scope,
  credential choices, event selection or GitHub Enterprise compatibility.
- 2026-11-19: Owner selected explicit deployment requests from CI (GIT-005),
  rather than manager-triggered deployment merely on configured workflow success.
  Exact GitHub event/API and security choices remain unresolved.
- 2026-11-19: Owner approved bounded queueing for competing same-stack manual
  and CI mutations. OPS-001 owns that shared policy; GIT does not define a second
  queue or inherit GitHub's pending-run replacement semantics.
- Open: Clone versus adopting an existing checkout, permitted paths, refs/subdirs,
  local-file preservation, Git library versus executable, submodules and LFS.
- Open: Credential types/providers, storage, host verification and rotation;
  webhook/API authentication, network exposure and authorization boundaries.
- Open: Deployment opt-in mechanism, provider/event revision mapping and permitted refs,
  replay/idempotency and ordering, detailed OPS queue policy, cancellation,
  retained revision history and truthful partial-failure/recovery semantics.
- Open: Failed/partial creation, adopting an existing checkout, and when later
  repository changes may be applied. Background Git polling and general
  deploy-on-source-change defaults remain unapproved. GIT-002 separately approves
  unattended execution for an explicitly enabled, authorized CI integration.

## Verification

No Git/CI behavior, security property or SDK integration has been implemented or
tested. Source sketches identify responsibility boundaries, not runtime evidence.
Documentation structure/ID/link checks cannot verify credential safety, webhook
authorization, deployment outcomes or compliance with unresolved dependencies.