+++
title = 'Contract Guide'
weight = 1
+++
# Behavioral Contracts

PROJ has eleven individually approved requirements; GIT has capability-scope,
enabled-CI unattended-deployment and exact CI-selected Git revision approval,
plus GitHub-first CI scope and explicit CI requests, with other policies awaiting
review. OPS records approved bounded same-stack queueing. No management draft is wholly approved
or implemented; a bounded discovery and activation slice is in progress. The inherited REPO document records
the template's delivery baseline and historical decisions;
EXAMPLE is only an example. See the [product plan](../planning/README.md) and
[reuse map](../planning/contract-reuse.md) before selecting standalone requirements.
Plugin requirements and their individual approvals do not transfer automatically.

Contracts describe intended observable behavior, ownership, acceptance cases,
and evidence. They help humans and AI keep changes aligned. They are not a
description of every implementation detail or a substitute for tests.

| Contract | Purpose | Status |
| --- | --- | --- |
| [REPO](repo.md) | Inherited repository/delivery baseline | Draft; template decisions, product setup not wholly verified |
| [EXAMPLE](example-feature.md) | Small example showing the format | Example, not an approved product promise |
| [PROJ](project-membership.md) | On-demand discovery, activation and retention of missing discovered projects | Draft; PROJ-001 through PROJ-011 owner-approved within stated scopes; bounded discovery/activation slice in progress |
| [GIT](git-stacks.md) | Git-backed stacks, private-repository credentials and CI/webhook deployment | Draft; GIT-001 through GIT-005 approved within stated scope; remaining behavior/security unresolved, not implemented or verified |
| [OPS](operation-coordination.md) | Shared same-stack manual/CI mutation coordination | Draft; OPS-001 bounded queueing approved, detailed queue/lifecycle policies unresolved; not implemented or verified |
| [RES](project-resolution.md) | Runtime identity and Compose inputs | Draft; RES-001/006 native naming/selection approved, RES-002 through RES-005 adopted portable source outcomes; bounded discovery resolution in progress, with environment and identity limits |
| [LIVE](live-state.md) | Dashboard home and live delivery | Draft; dashboard scope, SSE preference, LIVE-002 snapshot and LIVE-003 metrics/log model plus contained per-source Settings controls approved within stated scopes; log foundation implemented, overall profile/presets/defaults and capture consent/pressure/recovery remain unresolved |
| [AUTH](authentication.md) | App login and shared authentication boundary | Draft; AUTH-001 through AUTH-006 approved within stated scope; local-auth/dashboard-shell slice partially implemented and verified on `feat/local-auth`; OIDC, recovery, detailed security cases and Compose operations remain deferred or unverified |

## Shared Format

Use the sections shown in the example: Purpose And Scope, Ownership,
Requirements, Dependencies, Acceptance Cases, Decisions, Verification.
Give independently decidable requirements stable IDs such as REPO-001.
Do not silently reuse, remove, or change the meaning of an existing ID.

Distinguish observed behavior, documented intent, proposals, explicit owner
approvals, and executed verification. Approval and verification are separate.
A merged implementation or passing test does not approve the whole contract.
Record dates, exact scope, executed commands, environment, and verification gaps.
Use only as many documents and requirements as the behavior needs; full versus
partial approval is a review decision, not a document-count target.

## Change Workflow

Before a behavior change, identify affected requirements and consumers.
Discuss unsettled intent rather than silently making the contract match code.
Update behavior, acceptance cases, and tests together. Shared guarantees have
one owner; consumers link to its IDs and owners link back to consumers.
Record Depends on and Consumed by in both directions when real dependencies exist.
Examples do not impose new application features on projects using this template.

Run `make check-contracts` for document structure and stable-ID checks.
This check does not prove truth, approval, dependency completeness, or fulfillment.
Contract-only changes skip app jobs; shared tooling changes run affected checks.

## Publication

This folder is the canonical source and is excluded from Hugo by default.
Preview its inclusion with `make docs-contracts`. Set `PUBLISH_CONTRACTS=true`
only after reviewing the content for public publication. Do not copy contracts
into a second content folder. GitHub repository visibility is separate from
documentation-site publication.