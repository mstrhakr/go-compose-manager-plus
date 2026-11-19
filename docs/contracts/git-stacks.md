+++
title = 'Git Stacks And Deployment'
weight = 5
+++
# GIT: Git Stacks And Deployment

**Status:** Draft / capability scope approved; behavioral policies awaiting review

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
readers, failure publication, cleanup, deactivation and deletion/recreation.
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

## Dependencies

- Depends on [PROJ-002/003](project-membership.md): activation precedes project
  management changes and activation alone does not run Compose or enable
  automation. How explicitly requested Git acquisition interacts with inactive
  projects needs review; no pre-activation mutation exception is assumed.
- Depends on unextracted standalone identity/input and lifecycle-action contracts:
  exact project name, selected files/env/profiles, working directory and permitted
  pull/build/recreate effects cannot be invented by the Git adapter.
- Depends on shared credentials, path/storage safety, authentication/authorization,
  coordination and jobs/state-publication policies not yet approved here.
- Consumed by the [planned Git workstream](../planning/README.md#git-stacks-and-ci-deployment).
  Future UI and CI entry points must reference this owner rather than duplicate
  Git deployment sequencing.

## Acceptance Cases

Candidate scenarios for refinement, not approved detailed outcomes or run tests:

- Acquire a public repository and a private repository using an explicitly
  assigned credential; choose repository revision and Compose subdirectory/inputs.
- Distinguish initial clone, fetch/check, source pull/checkout and deployment.
  Include invalid inputs, dirty local files, missing refs and credential failures.
- Receive a CI trigger and bind it to the intended project, allowed repository
  and revision; distinguish accepted work from completed deployment.
- Reject unauthenticated/unauthorized triggers and handle duplicate, replayed,
  delayed and out-of-order deliveries without inventing success or safe retries.
- Exercise inactive/deactivated projects, same-name recreation, simultaneous
  manual/CI actions, failed preparation, partial Compose effects and restart.
- Keep Git secrets and secret-bearing URLs out of ordinary output. Specify
  repository-host trust, request/network scope and secret lifecycle before use.

## Decisions

- 2026-11-19: Owner added full Git-stack support to product scope, specifically
  pulling from repositories, private-repository credentials and webhook/CI
  deployment options. The previous blanket deferral of Git deployment is replaced
  by this workstream, without expanding the first prototype automatically.
- Open: Clone versus adopting an existing checkout, permitted paths, refs/subdirs,
  local-file preservation, Git library versus executable, submodules and LFS.
- Open: Credential types/providers, storage, host verification and rotation;
  webhook/API authentication, network exposure and authorization boundaries.
- Open: Explicit deployment opt-in, exact revision versus moving branch,
  replay/idempotency and ordering, approvals, job contention, cancellation,
  retained revision history and truthful partial-failure/recovery semantics.
- Open: Relationship of clone/setup to activation, and when repository changes
  may be applied. No background Git polling or automatic deployment defaults are
  approved merely by requesting Git-stack support.

## Verification

No Git/CI behavior, security property or SDK integration has been implemented or
tested. Source sketches identify responsibility boundaries, not runtime evidence.
Documentation structure/ID/link checks cannot verify credential safety, webhook
authorization, deployment outcomes or compliance with unresolved dependencies.