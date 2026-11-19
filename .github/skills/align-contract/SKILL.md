---
name: align-contract
description: 'Align Compose Manager Plus with explicitly approved standalone requirements. Use when authorized to assess contract compliance, implement alignment, fix confirmed specification deviations, or add regression coverage. Trace shared owners and callers, preserve unapproved semantics, and validate actual outcomes before claiming fulfillment.'
argument-hint: '<approved requirement IDs or bounded workflow> [assess-only]'
---

# Align Implementation With Approved Contracts

## Scope And Entry Gate

Adapted from compose_plugin's align-contract skill on 2026-11-19, using this
product's Go/JS validation instead of plugin PHP/shell gates.
Read the [contract index](../../../docs/contracts/README.md), relevant contracts,
[product plan](../../../docs/planning/README.md),
[reuse map](../../../docs/planning/contract-reuse.md),
[source register](../../../docs/planning/sources.md) and
[development guide](../../../docs/content/development.html).
The index owns status/ownership conventions. Do not assume a product inventory
exists or substitute the plugin's inventory and approvals for standalone intent.

Loading a skill, approving a scenario or requesting extraction/refinement does
not authorize implementation. Confirm mode and record revision, dirty-file
qualifications and requirement IDs:

- **Assess-only:** inspect/report compliance and gaps; no runtime/test changes.
- **Implementation:** requires explicit authorization for a bounded approved
  standalone requirement set. A clear request to align/fix those requirements
  supplies that authorization, subject to the product's staging boundary.

Never implement ambiguous draft intent or rewrite intent to fit code. Route
missing evidence to [extract-contract](../extract-contract/SKILL.md) and unsettled
policy to [refine-contract](../refine-contract/SKILL.md).

## Assess And Plan

1. State observable outcomes, prohibited effects, input/output shapes,
   failure/recovery, lifecycle/scope and intentional caller differences. Include
   dependency approval and evidence limits before claiming compliance.
2. Trace the authoritative owner and every relevant caller. Implement shared
   action meaning once; separate effects from targeting/options/orchestration.
   Preserve approved caller differences without generalizing caller-only approval.
   For async/persisted state, enumerate readers, publishers, invalidators,
   acknowledgers, cleanup, external owners and deletion/recreation boundaries.
   Locks protect only participants; enforce runtime safety under ownership.
3. Classify each requirement independently as already compliant, confirmed
   mismatch, unresolved intent or not yet verified, with supporting evidence.
   No implementation is not proof of an existing behavioral regression.
4. Separate confirmed defects from design concerns and residual limits. Inspect
   siblings only when evidence supports the same failure mode. Review ownership,
   maintainability, hot-path Big-O, repeated I/O, resource bounds and races.
5. Choose the simplest complete correction at the shared boundary. Reuse existing
   implementations; do not add a facade over duplicated policy or speculative
   frameworks. Separate movement from behavior changes where practical.

## Implement And Verify When Authorized

1. Establish baseline behavior and outcome-focused regressions for approved
   scope. Exercise relevant callers and forbidden effects, not only new helpers.
   Use deterministic interleavings for stale checks, mutations, failure,
   deletion/recreation and acknowledgement boundaries where applicable.
2. For approved UI scope, verify desktop/mobile, touch/keyboard, overflow,
   panels/dialogs and shared state across alternate controls. Mockups/backend
   tests do not prove responsive interaction. Disclose unexecuted browser checks.
3. Preserve type safety, explicit errors and public compatibility. Stop for
   refinement rather than inventing additional lifecycle or destructive policy.
4. Run fast applicable lint and focused tests while iterating, then `make check`
   for completed app work. Run `make docs`, `make check-workflows`,
   `make check-security` and container checks when their surfaces change,
   following repository instructions. Inspect output; missing-tool skips and
   cross-compilation are not successful native-platform verification.
5. Recheck the end-to-end invariant across participants after fixes. A single
   passing wrapper or new module does not verify its callers. Repair introduced
   failures; disclose baselines and residual external-system limits.
6. Update requirement acceptance maps and directly related docs with actual
   checks, revision/environment, results, skips and unverified gates. Do not
   promote whole-contract fulfillment from a partial approved slice.
7. Commit coherent validated slices using Conventional Commits; preserve unrelated
   work. Use short-lived branches/PRs. Do not amend, push, publish or bypass
   protection without the required authorization.

## Handoff

Report approved scope, per-requirement classifications, affected callers, executed
checks, skips/baselines, commits and uncertainty. Distinguish reproduced guarantees
from assumptions. If blocked by intent, return to one focused refinement question.
Skills guide work; neither tests nor static checks substitute for owner approval.