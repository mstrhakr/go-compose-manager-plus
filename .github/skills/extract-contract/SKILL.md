---
name: extract-contract
description: 'Inventory feature contracts and recover intended behavior for Compose Manager Plus. Use when enumerating contract candidates, extracting a contract, recovering specifications, defining invariants, reviewing completeness, or updating coverage. Compare source-plugin contracts, code, callers, tests, docs and historical evidence without treating implementation or plugin approval as standalone specification.'
argument-hint: 'inventory | extract <candidate ID or capability> | refresh <contract>'
---

# Recover Behavioral Contracts

## Scope And Authority

Adapted from compose_plugin's extract-contract skill on 2026-11-19. Preserve its
evidence-first workflow, not its Unraid scope, inventory counts or PHP/shell gates.
Work only in this product repository unless the owner explicitly authorizes more.
Default to documentation-only research; extraction does not authorize runtime,
test, dependency or execution-policy changes.

Start with the [contract index](../../../docs/contracts/README.md),
[product plan](../../../docs/planning/README.md),
[reuse map](../../../docs/planning/contract-reuse.md),
[source register](../../../docs/planning/sources.md), and
[contract template](./assets/contract-template.md).
The contract index owns status labels, format and ownership conventions.
Use an existing product inventory if present; do not assume the plugin inventory
is this product's inventory or create one merely to satisfy a workflow link.

Recover intended guarantees, not a transcription of code. Code, tests, docs,
merged PRs and forum statements are evidence, not individually authoritative.
Preserve source requirement IDs and approval qualifications in provenance;
standalone requirements need their own IDs and explicit adoption decisions.
Use [refine-contract](../refine-contract/SKILL.md) for owner decisions and
[align-contract](../align-contract/SKILL.md) for authorized compliance work.

## Inventory Mode

1. Read current contracts, planning and repository instructions. Record date,
   revision, dirty-file qualifications and scope examined.
2. Enumerate relevant capabilities and entry surfaces: HTTP/UI, future CLI,
   background work, persisted state, external owners, packaging and delivery.
   Separate implemented surfaces from proposed ones; do not import the plugin's
   whole roadmap or assume a manager script's contents before inspection.
3. Use docs, entry points, test names and source provenance for breadth discovery,
   not exhaustive tracing of every candidate.
4. Give candidates stable IDs, bounded responsibilities, evidence pointers and
   status. Shared invariants have one owner; consumers reference that owner.
5. Record gaps, uncertain boundaries and split/merge provenance. Keep discovery,
   draft, approval and verification counts separate; explain denominator changes.
   Do not present guesses about unknown scope as measured completion.

## Extraction Mode

1. Select one bounded capability, purpose, non-goals, dependencies and pending
   decisions. Recover shared action effects before caller variants; keep effects,
   target selection, invocation options and orchestration distinct.
2. Trace existing policy/execution and all relevant readers, writers,
   acknowledgements, failures and cleanup. Inspect siblings when evidence points
   to the same invariant. For unimplemented behavior, say no enforcement exists.
3. Compare tests, docs and help without assuming tests encode desirable behavior.
   For UI scope, inspect desktop/mobile, touch/keyboard and overflow evidence;
   missing evidence stays a gap, not an inferred usability guarantee.
4. Follow relevant history, issues, PRs and discussions using the source register.
   Prefer primary sources and fixed revisions; refresh relevant evidence instead
   of downloading every discussion. Distinguish retrieval, screening, detailed
   inspection and verification. Paginate before claiming complete retrieval.
5. Source-plugin approvals remain source-scoped. Do not invent public links to
   unpublished contracts. Record unavailable evidence without bypassing access
   restrictions or silently substituting an older branch.
6. Separate Observed, Documented intent, Proposed, Approved and Verified. Route
   consequential choices to refinement, one question at a time; do not ask for
   discoverable facts or resolve policy by copying implementation.
7. Use the template selectively: inputs/outputs, pre/postconditions, permitted
   and forbidden effects, error/recovery, compatibility and resource bounds.
   For async/persisted state, enumerate every participant and define publication,
   invalidation, acknowledgement, deletion/recreation and external-owner limits.
8. Give requirements stable product IDs with source provenance, approval scope,
   acceptance cases, enforcement and verification gaps. Prefer outcome checks and
   deterministic interleavings over wrapper/source-string assertions.
9. Update the index, reuse map and source register only where facts change.
   Stop at a reviewable draft; extraction is not whole-contract approval.

## Refresh And Handoff

Preserve requirement IDs and material superseded wording. Compare new evidence
and decisions without weakening intent to fit code. Record access gaps and exact
research scope. Screenshots, when useful, must be redacted and labeled Current,
Proposed, Approved reference or Verified capture with date, revision/environment
and requirement IDs. Fixture renders do not prove live behavior or backend safety.

Validate links/anchors, Markdown/frontmatter, stable IDs, counts and whitespace.
Use `make check-contracts` for product contract documents; it checks structure,
not approval or fulfillment. Follow repository validation and commit rules;
documentation-only research needs no app suite. Never count missing-tool skips
or historical tests as executed verification. Do not push without approval.
Keep raw research out of committed API/forum dumps. Skills guide work; they are
not deterministic enforcement or implementation evidence.