# Planning Sources

**Inspected:** 2026-11-19. This is a bounded local staging/research record, not
an exhaustive inventory, fresh forum audit or source implementation review.

## Fixed Baselines

| Source | Revision / scope | Qualifications |
| --- | --- | --- |
| Product scaffold | `mstrhakr/go-template`, published `main`, `f281859b8c912ab7e251c7b2a6ddcaf6e663393b` | Fetched main; exported tracked files without Git history or ignored outputs. Kept MIT and htmx notices. Current local template branch was not used. |
| Compose Manager | Local `/code/compose_plugin`, `feat/basic-advanced-foundation`, `5ba0c341c7e985004c8ef3802f80793ecbfae509` | Worktree clean at inspection. More recent than handoff HEAD `57b6993`; reviewed documents are local authority/evidence, not claims about origin/main or published availability. No plugin runtime changes or tests performed. |
| Docker Networks | Local `/code/docker.networks`, `main`, `d8d392f758250bdbfb181a196b283b14429f8a69` | Worktree clean; local main reported one commit behind origin/main. Used local checkout as supplied, not a claim to latest remote behavior. |
| Compose Specification | Local `/code/compose-spec`, `3a4a6995e4a971f6173dff9130cb3e38e6738651` | Clean worktree at the bounded 2026-11-19 reading of Compose files, include, build Git contexts and deploy purpose. Not a complete spec audit; network/volume chapters remain to inspect for those workstreams. No sibling files changed. |
| VM script | Pending upload to `scratch/` | Not supplied or inspected. No inferred storage format, credential handling, operations or deployment model. |

Template source at a published fixed revision:
<https://github.com/mstrhakr/go-template/tree/f281859b8c912ab7e251c7b2a6ddcaf6e663393b>.
The source repositories retain their own ownership; nothing in this product
bootstrap publishes their branches or modifies their worktrees.

## Standalone Owner Refinement

2026-11-19, direct owner conversation, recorded from product baseline `f4d6148`:

- Owner preferred selecting a project folder during setup, with possible detected
  location suggestions. Suggestion mechanisms and exact root scope remain open.
- Owner selected automatic listing rather than per-project adoption for listing,
  while requiring activation before management.
- Owner explicitly selected option A in the follow-up manual-Stop scenario:
  activation gates both manual and automatic project changes. These bounded
  approvals are recorded as [PROJ-001/002](../contracts/project-membership.md).
- In the subsequent activation-effects scenario, the owner explicitly selected
  permission-only option A: no Compose run, project-file rewrites or schedule
  enablement. The stated rationale was avoiding unexpected automatic actions.
  [PROJ-003](../contracts/project-membership.md#proj-003-activation-grants-permission-without-applying-changes)
  records this approval without generalizing it into unreviewed automation policy.
- Owner explicitly selected direct-child rather than recursive project-folder
  discovery for now. Interest in nested discovery and UI grouping was not approval
  of either feature. This depth-only decision is recorded as
  [PROJ-004](../contracts/project-membership.md#proj-004-direct-child-project-discovery).
- Owner then selected recognized Compose projects only, not every child directory
  (PROJ-005). The following message added on-demand discovery, optionally during
  setup (PROJ-006), and selecting stack folders one at a time through browsing
  (PROJ-007). These decisions do not determine exact filenames, picker path limits
  or missing/invalid-input behavior. The earlier automatic-inclusion decision
  still applies when a requested scan runs, not to background scanning.
- Owner explicitly added Git-stack capability scope: pulling repository sources,
  private-repository credentials and CI-triggered deployment through webhooks or
  similar integration. [GIT-001](../contracts/git-stacks.md) records scope, not
  detailed authentication, activation, deployment or automation policy approval.
- Owner then clarified that the extra activation gate is only for discovery-added
  stacks. Explicit manual additions and cloning a repository as a new Git stack
  start active; there is no deactivate option. PROJ-002/007 were amended with
  supersession notes and PROJ-008/009 record the explicit entry-path and
  no-deactivation rules. This did not answer the pending per-deployment CI approval
  question or enable CI automation merely through active membership.
- Owner subsequently explicitly selected unattended CI deployment over per-run
  UI approval. [GIT-002](../contracts/git-stacks.md#git-002-enabled-ci-deployment-requires-no-per-run-ui-approval)
  records the scenario: CI deployment already enabled for an active stack, and
  an authenticated, authorized trigger. The stated rationale was that requiring
  per-deployment manual approval defeats CI deployment's purpose. No revision,
  credential protocol or job-concurrency decision was inferred.
- Owner then selected exact-commit CI deployment, asking to use existing provider
  conventions where available rather than invent a universal webhook schema.
  [GIT-003](../contracts/git-stacks.md#git-003-deploy-the-immutable-revision-selected-by-ci)
  records that outcome. Provider/event mapping and authorization remain open;
  research is qualified under [CI revision conventions](#ci-revision-conventions).
- Owner selected GitHub-first CI integration over initial GitHub plus GitLab
  support. [GIT-004](../contracts/git-stacks.md#git-004-github-first-for-ci-deployment-integration)
  records this scope choice, without inferring GitHub-only repository hosting,
  Enterprise support, credential types or a specific GitHub trigger event.
- Owner selected explicit deployment requests from CI over manager deployment
  triggered by configured workflow success alone.
  [GIT-005](../contracts/git-stacks.md#git-005-ci-explicitly-requests-deployment)
  records CI-controlled promotion intent; exact GitHub event/API, authentication
  and delivery semantics were not inferred.
- Owner requested considering the local Compose Specification during contract
  work. The three contract skills now include that evidence step, preserving
  the distinction between normative Compose semantics, implementation support
  and standalone manager/CI policy.
- Owner raised possible backend reuse with the plugin and requested an opinion
  before deciding. The supplied other-agent excerpt is architectural context,
  not fresh implementation evidence, an approved shared-service design or plugin
  migration authorization.
- In follow-up, the owner approved emphasizing maintainability and clear ownership
  rather than building for hypothetical reuse, and said they do not currently
  intend to rebuild the plugin. The bounded
  [planning direction](README.md#maintainability-and-ownership-direction) is clean
  boundaries now, extraction when justified; shared-backend delivery is not a
  product goal. The owner's comment about future native Unraid Compose support
  is context for that preference, not independently verified release evidence.

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
- Git follow-up: bounded GIT-LIFE/CHECK/DEPLOY/AUTO and CRED-VAULT/GIT sketches
  in `feature-outline.md`, plus Git/storage/credential inventory entries read.
  No Git implementation/caller trace, test execution, historical refresh or
  plugin webhook support verification was performed. These sources identify
  candidate ownership boundaries only; see the
  [Git reuse mapping](contract-reuse.md#git-and-deployment-candidates).

The local collection is not assumed published. The handoff warned that its
previous branch/HEAD was unavailable remotely; availability of the new source
HEAD was not established here. Do not replace these references with fabricated
GitHub blob links. The reuse map provides a self-contained bounded summary, not
a complete source snapshot. Before implementation on another machine, obtain
this pinned checkout or an owner-approved documentation snapshot; recheck
approval and supersession sections there. Do not silently use older origin/dev.

## Compose Specification Evidence

Bounded reading on 2026-11-19 at the clean local spec revision above:

- `03-compose-file.md`: preferred/default Compose filenames, ordered merging and
  relative paths based on the first Compose file's parent directory.
- `14-include.md`: included models have their own project-directory/path and
  interpolation environment semantics; include recursion is not filesystem
  discovery recursion or a manager membership policy.
- `build.md`, opening sections through Git context syntax: Git URLs can identify
  a ref and subdirectory for build context. This is not a manager stack-cloning
  API, webhook revision protocol or guarantee of fully reproducible deployment.
- `deploy.md`, opening purpose/attributes: optional service resource/platform
  deployment settings, not an external CI trigger format or approval workflow.

No CI webhook contract was found in the inspected sections; this is not a claim
of exhaustive absence across every spec file. The selected SDK/CLI's actual
support still needs independent validation. No new filename, merge, include or
build policy is approved merely by reading these sections.

## CI Revision Conventions

Official sources retrieved on 2026-11-19; moving upstream branches, not pinned
provider/schema versions or runtime integration verification:

- [GitHub REST API schema](https://github.com/github/rest-api-description/blob/main/descriptions/api.github.com/api.github.com.json):
  `workflow-run.head_sha` identifies the head commit for the workflow run;
  `deployment.sha` is the commit and `deployment.ref` may be a branch, tag or SHA.
  These object schemas identify fields, not every webhook's envelope or proof
  that a workflow checked out and tested the same source revision.
- [GitLab webhook events](https://github.com/gitlabhq/gitlabhq/blob/master/doc/user/project/integrations/webhook_events.md#pipeline-events):
  pipeline events describe status changes; the example `object_attributes`
  includes `sha`, `ref` and `status`. A generic pipeline event is not by itself
  authorization or a command to deploy.

Use the supported integration's event-specific metadata rather than inventing
one universal provider payload. Bind the intended repository and immutable
revision, and separately define trusted sender, event/success selection and
authorization. Ordinary push events and arbitrary SHA fields do not establish
passing CI. This research supports GIT-003's existing owner-approved intent; no
provider adapter, webhook endpoint, protocol or automatic fallback was implemented.

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