# Planning Sources

**Inspected:** 2026-11-19. This is a bounded local staging/research record, not
an exhaustive inventory, fresh forum audit or source implementation review.

## Fixed Baselines

| Source | Revision / scope | Qualifications |
| --- | --- | --- |
| Product scaffold | `mstrhakr/go-template`, published `main`, `f281859b8c912ab7e251c7b2a6ddcaf6e663393b` | Fetched main; exported tracked files without Git history or ignored outputs. Kept MIT and htmx notices. Current local template branch was not used. |
| Compose Manager | Local `/code/compose_plugin`, `feat/basic-advanced-foundation`, `5ba0c341c7e985004c8ef3802f80793ecbfae509` | Worktree clean at inspection. More recent than handoff HEAD `57b6993`; reviewed documents are local authority/evidence, not claims about origin/main or published availability. No plugin runtime changes or tests performed. |
| Docker Networks | Local `/code/docker.networks`, `main`, `d8d392f758250bdbfb181a196b283b14429f8a69` | Worktree clean; local main reported one commit behind origin/main. Used local checkout as supplied, not a claim to latest remote behavior. |
| Compose Specification | Workspace checkout available at `/code/compose-spec` | Not audited or changed in this staging pass. Read applicable chapters and pin its revision when selecting network/volume semantics. |
| VM script | Pending upload to `scratch/` | Not supplied or inspected. No inferred storage format, credential handling, operations or deployment model. |

Template source at a published fixed revision:
<https://github.com/mstrhakr/go-template/tree/f281859b8c912ab7e251c7b2a6ddcaf6e663393b>.
The source repositories retain their own ownership; nothing in this product
bootstrap publishes their branches or modifies their worktrees.

## Local Compose Contract Reading

Start under `/code/compose_plugin/docs/contracts/`:

- `README.md`: authority, status, ownership and workflow conventions; read.
- `inventory.md`: counts, discovery limitations and approval qualifications;
  opening inventory/progress sections read.
- `sources.md`: current opening source/approval register inspected; not a new
  retrieval of all referenced issues, PRs or forums.
- `project-identity.md`, `stack-registration.md`, `stack-files.md`,
  `stack-lifecycle.md`, `mutation-coordination.md`, `ui-operations.md`: bounded
  purpose, requirements and nearby relevant sections inspected for the reuse map,
  not full independent extraction or enforcement verification.
- `profiles.md`, `state-metadata.md`, `feature-outline.md`,
  `definition-priorities.md`, `compose-manager-plus-forum-research.md`: identified
  follow-up/dependency context; not fully reviewed in this phase.
- Update, automatic-update, retry, build-on-update, editor/history and bulk
  contracts remain outside the first proposed slice.

The local collection is not assumed published. The handoff warned that its
previous branch/HEAD was unavailable remotely; availability of the new source
HEAD was not established here. Do not replace these references with fabricated
GitHub blob links. The reuse map provides a self-contained bounded summary, not
a complete source snapshot. Before implementation on another machine, obtain
this pinned checkout or an owner-approved documentation snapshot; recheck
approval and supersession sections there. Do not silently use older origin/dev.

## Network Evidence

Under `/code/docker.networks` inspected `README.md`, `docs/pages-inventory.md`
and bounded entrypoint/function evidence from
`source/docker.networks/include/ExecFunctions.php`, including action dispatch,
protection, deletion, connect/disconnect and template-persistence handoffs.
Identified `tests/template-persistence.php`; did not execute it or trace every
function/caller. The plugin documents network management, not a volume manager.
Current behavior is evidence, not standalone approval.

## SDK Research Baseline

Official entry points:

- <https://docs.docker.com/compose/compose-sdk/>
- <https://github.com/docker/compose>
- <https://github.com/compose-spec/compose-go>
- <https://docs.docker.com/reference/api/engine/>

The official documentation links above came from the handoff; they were not a
fresh full documentation audit. GitHub latest-release API and fixed upstream
Compose module manifest were checked directly on 2026-11-19:

- Compose latest normal release: `v5.8.0`, published 2026-11-17.
- compose-go latest normal release: `v2.17.0`, published 2026-11-16.
- <https://github.com/docker/compose/blob/v5.8.0/go.mod> declares module
  `github.com/docker/compose/v5`, Go `1.26.11`, compose-go/v2 `v2.17.0`,
  Docker CLI `v29.10.0+incompatible`, Moby API `v1.57.0`, Moby client `v0.6.3`
  and BuildKit `v0.33.1`.
- The scaffold uses Go `1.27.4`; satisfying the declared minimum does not prove
  third-party embedding, dependency resolution, helper availability or runtime fit.

Do not independently pick unrelated latest CLI/Engine module versions. During
the authorized spike, inspect official SDK examples and module replacement/vendor
constraints, choose coherent exact versions, inspect licensing/security, run
`go mod tidy` and actual compile/tests, and establish required external tooling
for the chosen operations (build, auth helpers, SSH, etc.). No Compose/Engine
dependency or SDK build was added/run in this phase. Recheck upstream before
the spike; these versions are research evidence, not permanent product pins.