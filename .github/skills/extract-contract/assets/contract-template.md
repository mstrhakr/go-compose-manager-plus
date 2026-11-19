# <ID>: <Capability>

**Status:** Draft / awaiting owner review

**Baseline:** <date, revision, worktree qualifications and research scope>

## Purpose And Scope

<Observable purpose, boundaries, non-goals and related owners. Distinguish
standalone proposals from source-plugin guarantees.>

## Ownership

<One authoritative owner per shared guarantee; consumers reference it. Separate
observable guarantees from current mechanisms. For async/persisted state, name
readers, publishers, invalidators, acknowledgers, cleanup and external owners;
include identity changes and deletion/recreation boundaries.>

## Requirements

### <ID>-001: <Independently Decidable Guarantee>

<Specific observable promise, inputs/outputs, pre/postconditions, permitted and
forbidden effects, error/recovery, compatibility and relevant resource bounds.>

**Approval:** Proposed. <Record exact approved scope/date only after a decision.>

**Source provenance:** <Source IDs, fixed revision or qualified local evidence;
source approval is not standalone adoption.>

**Enforcement:** <Current mechanism or not implemented; evidence and gaps.>

<Use uppercase alphabetic product prefixes and three-digit requirement numbers,
as required by the existing checker. Do not reuse source IDs as product IDs.>

## Dependencies

<Depends on and Consumed by, with reciprocal requirement references where real
dependencies exist. Label unextracted/unapproved dependencies instead of inventing
guarantees. Keep action effects separate from targeting and orchestration.>

## Acceptance Cases

<Observable normal, failure, forbidden-effect and concurrency/recovery scenarios
mapped to requirement IDs. Include UI desktop/mobile and touch/keyboard scope
where relevant; do not prescribe unapproved widgets or exact breakpoints.>

## Decisions

<Date, concrete scenario, exact owner decision/scope and rationale. Keep proposals,
approval, supersession, implementation mismatches and open questions distinct.
Neither passing tests nor a merged change approves a requirement.>

## Verification

<Actual checks, revision/environment, results, skips, access gaps and residual
limitations. Distinguish inspected tests from executed checks. No implementation
or compliance is implied by a draft or an approved scenario.>

<Optional visual references: relative images with descriptive alt text, status,
requirement IDs, date and revision/environment. Redact secrets; identify fixtures
versus live captures. Written guarantees remain authoritative.>