---
name: refine-contract
description: 'Refine Compose Manager Plus behavioral contracts with the owner. Use when reviewing scenarios, resolving specification ambiguities, asking contract questions, approving or amending requirements, or continuing connected review. Separate proposals, source-plugin approvals, standalone decisions and verification; record narrowly scoped decisions without runtime changes.'
argument-hint: '<contract ID or connected workflow> | continue'
---

# Refine Behavioral Contracts

## Scope And Authority

Adapted from compose_plugin's refine-contract skill on 2026-11-19. This is
owner-guided specification refinement, not implementation or transcription of
current behavior. Default to documentation-only changes. Scenario approval does
not approve a whole contract or authorize runtime, test or dependency changes.

Start with the [contract index](../../../docs/contracts/README.md),
[product plan](../../../docs/planning/README.md), relevant contracts,
[reuse map](../../../docs/planning/contract-reuse.md) and
[source register](../../../docs/planning/sources.md).
The index owns evidence/status and ownership conventions. Product priorities
come from the product plan, not the plugin's review checkpoint or candidate count.
Use [extract-contract](../extract-contract/SKILL.md) for missing evidence rather
than repeating its full research workflow here.

For Compose-related decisions, consider relevant chapters in the local
`/code/compose-spec` checkout before framing options, following
[specification evidence](../extract-contract/SKILL.md#compose-specification-evidence).
Explain normative semantics and actual SDK/CLI support separately from proposed
manager behavior. Do not invent a custom rule where Compose already decides it,
or present a product/CI policy as something the spec mandates. Record consulted
sections and revision in the source register; no full-spec audit is required.

## Procedure

1. Recover the pending question, prior answers, stable IDs and exact scope. Read
   relevant source approvals/supersessions and enough implementation/callers to
   establish what exists. If management behavior is absent, say so; do not invent
   a current implementation. Do not ask for discoverable facts.
2. Select one consequential decision. Prefer connected ordinary workflows and
   easy wins, then failures and detailed edge cases before alignment. Do not
   exhaust one feature recursively. Define shared action effects before caller
   variants; keep effects, targeting/options and orchestration distinct.
3. Separate observed behavior, historical intent, proposals, source approvals,
   standalone approvals and verification. Source decisions do not transfer
   automatically. Favor Compose-native semantics; deliberate departures require
   explicit bounded review. Preserve intentional caller differences.
4. Present a concrete scenario, supported current/source behavior, meaningful
   options and a reasoned recommendation. State what changes and what stays
   undecided. Avoid leading language, false dichotomies and claims that passing
   tests establish intent. UI scope includes mobile/narrow, touch/keyboard,
   panels and confirmations; usability approval is not widget/layout approval.
5. Ask one focused question in ordinary final-response Markdown, then stop and
   wait. Do not use a question popup for contract refinement unless higher-priority
   instructions require it. This preference applies on continuations, too.
6. Interpret answers only within the stated scenario. Tentative intuition,
   concerns, added context, silence and deferral are not option approval. When an
   answer changes the framing, incorporate it and reoffer meaningful options for
   the same pending decision. Clarify before recording consequential ambiguity;
   do not answer for the owner or simply endorse their tentative suggestion.
7. Record settled decisions in the owning product contract with stable product
   ID, date, exact scope, rationale and approval status. Preserve source IDs and
   material supersession as provenance. Shared guarantees have one owner;
   consumers reference it, with reciprocal dependencies where real. Update the
   index, reuse map/source register or existing inventory only as facts change.
8. Validate links/anchors, stable-ID uniqueness, frontmatter, counts and whitespace.
   Use `make check-contracts` for changed product contracts. Documentation-only
   decisions need no app suite. Commit coherent validated changes per repository
   rules, preserve unrelated work and do not push without approval.
9. Keep review moving after recording a decision or answering a sidebar: briefly
   summarize, then offer the next focused question, or reoffer the updated pending
   question if unsettled. Stop without a question only for owner-requested pause,
   redirection, completed agreed scope or an explicit blocker.

## Question Pattern

Use ordinary Markdown, not a tool's question string. Keep it readable and concise:

- **Scenario:** concrete state and action, with relevant inputs.
- **Current/source behavior:** supported evidence; product behavior may not exist.
- **Decision:** meaningful A / B options, recommendation and reasoning.
- **Boundary:** what remains unchanged or undecided.
- **Question:** one focused question, then wait.

Do not reopen settled decisions merely because another caller uses them. Revisit
only for conflicting evidence, owner request or a genuine uncovered scope boundary,
explicitly identifying the earlier approval.

## Handoff To Alignment

Before [align-contract](../align-contract/SKILL.md), identify the approved subset,
unresolved dependencies and required final edge-case review. Do not require the
plugin's entire inventory or imply broad drafts authorize implementation. Skills
guide behavior; tests and actual outcomes supply verification, not approval.