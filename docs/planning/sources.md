# Planning Sources

**Inspected:** 2026-11-19. This is a bounded local staging/research record, not
an exhaustive inventory, fresh forum audit or source implementation review.

## Fixed Baselines

| Source | Revision / scope | Qualifications |
| --- | --- | --- |
| Product scaffold | `mstrhakr/go-template`, published `main`, `f281859b8c912ab7e251c7b2a6ddcaf6e663393b` | Fetched main; exported tracked files without Git history or ignored outputs. Kept MIT and htmx notices. Current local template branch was not used. |
| Compose Manager | Refreshed local HEAD `10e9179cbb0331fb956a0b60ba671e477a74638e`; original staging baseline `5ba0c341c7e985004c8ef3802f80793ecbfae509` | Refreshed 2026-11-19: worktree has unrelated planning/source-register/development/e2e changes. Read affected unchanged source contracts, runtime-plan passages and metrics-publisher boundaries; no sibling edits/tests or assumption of remote publication. Original clean staging evidence retains its original scope. |
| Docker Networks | Local `/code/docker.networks`, `main`, `d8d392f758250bdbfb181a196b283b14429f8a69` | Worktree clean; local main reported one commit behind origin/main. Used local checkout as supplied, not a claim to latest remote behavior. |
| Compose Specification | Local `/code/compose-spec`, `3a4a6995e4a971f6173dff9130cb3e38e6738651` | Clean worktree at the bounded 2026-11-19 reading of Compose files, include, build Git contexts and deploy purpose. Not a complete spec audit; network/volume chapters remain to inspect for those workstreams. No sibling files changed. |
| VM script | Pending upload to `scratch/` | Not supplied or inspected. No inferred storage format, credential handling, operations or deployment model. |

Template source at a published fixed revision:
<https://github.com/mstrhakr/go-template/tree/f281859b8c912ab7e251c7b2a6ddcaf6e663393b>.
The source repositories retain their own ownership; nothing in this product
bootstrap publishes their branches or modifies their worktrees.

## Standalone Owner Refinement

2026-11-21: owner chose persistent project listing over a scan-only preview for
the next slice. For a previously discovered project absent on a successful scan,
the owner selected retaining its listing and activation state as unavailable;
scan failure is not proof of removal. See
[PROJ-010](../contracts/project-membership.md#proj-010-retain-previously-discovered-projects-that-disappear).
The owner later selected same-path restoration of saved activation state for
the prototype. Full comparison and historical changes for easier rollback are
long-term ideas; snapshots versus Git and detailed policies remain undecided.
For the first read-only discovery slice, the owner selected project-local `.env`
interpolation without inheriting the manager process environment; broader
environment policy remains open.
The owner also selected one saved discovery root initially, with existing entries
not removed merely by changing that setting, entered as an application-visible
path in the authenticated UI. Container mounts are configured separately; see
[PROJ-011](../contracts/project-membership.md#proj-011-one-saved-discovery-root-initially).
Removal and other lifecycle details remain open.

2026-11-21: owner selected the four standard Compose filenames for initial
discovery, deferring `.env`-only `COMPOSE_FILE` discovery. This does not change
native resolution of selected projects. The owner also
selected listing a newly discovered folder with malformed or unresolvable inputs
as inactive with a clear resolution error.
For initial root scans the owner chose to skip symlinked child directories and
report the limitation; other symlink cases remain open.
See [PROJ-005](../contracts/project-membership.md#proj-005-discover-compose-projects-rather-than-every-directory).
Compose Specification `03-compose-file.md` at `3a4a6995e4a971f6173dff9130cb3e38e6738651`
names the default and compatibility filenames; it does not prescribe manager
discovery. The selected model loader's `cli.DefaultFileNames` supplies these names
in its own precedence order.

2026-11-20: owner approved [AUTH-006](../contracts/authentication.md#auth-006-bootstrap-env-does-not-reset-persisted-credentials):
bootstrap env seeds initial auth state only; persisted credentials/setup state
remain authoritative across restarts, and recovery/reset is explicit and separate.
No database, recovery tool or interrupted/corrupt-state behavior was selected.

2026-11-19, direct owner conversation, recorded from product baseline `f4d6148`:

- Owner requested app login/auth among the first prototype steps, despite intended
  reverse-proxy/WAF/local-only deployment, with foundations for future methods
  such as OIDC. [AUTH-001/002](../contracts/authentication.md) record that scope
  and ownership direction. "Basic auth" remains ambiguous between HTTP Basic and
  simple form/session login; immediate OIDC was tentative, not approved delivery.
- Owner then selected option A: local username/password form login with
  server-managed sessions first, OIDC later.
  [AUTH-003](../contracts/authentication.md#auth-003-local-form-login-with-server-managed-sessions-first)
  resolves the initial ambiguity without selecting account count, bootstrap,
  recovery, storage/library or session security parameters.
- Owner then chose one administrator for the prototype, explicitly requiring
  correct future multi-account plumbing instead of a singleton implementation
  needing replacement. [AUTH-004](../contracts/authentication.md#auth-004-one-initial-admin-with-multi-user-capable-identity-plumbing)
  records stable internal identity, distinct sessions and one shared access
  boundary, without approving multiple-account UI, RBAC, an identity framework
  or immediate OIDC delivery.
- Owner chose browser setup with an explicit change from the proposed one-time
  setup credential: provision the initial user via Docker/deployment environment
  variables, then require a password update at first login through the setup
  wizard. [AUTH-005](../contracts/authentication.md#auth-005-environment-bootstrap-with-forced-first-login-password-change)
  records the amended flow. Env reapplication/restart and reset/recovery details
  remain unresolved; no real bootstrap secrets or variable names were requested.
- Owner clarified the overall product as a standalone counterpart/clone of the
  Unraid Compose Manager Plus plugin, with a modern interface instead of its
  Unraid-native presentation. This strengthens the plugin's role from case study
  to functional baseline; it does not approve every source draft, legacy naming
  behavior or all-feature first-prototype scope. Existing standalone decisions
  and the bounded local prototype remain in effect. Naming and database details
  are not approved merely by this broader product description.
- Owner subsequently explicitly clarified native naming: UI labels are
  display-only; explicit runtime naming belongs in Compose configuration, not a
  separate manager setting or injected project-name flag. Prototype support for
  an external override is not needed. [RES-001](../contracts/project-resolution.md)
  records this approval, excluding legacy normalization/pinning machinery while
  leaving existing custom-name adoption and native-name changes unresolved.
- Owner then selected native Compose input selection for the prototype over
  implementing ordered-file controls immediately. [RES-006](../contracts/project-resolution.md)
  records that bounded approval; native env/input mechanisms and SDK support
  are not assumed away, and env/profile scope remains separately reviewable.
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

## Foundation Adoption And Live State

2026-11-19, following product checkpoint `68ab9d2`:

- Owner redirected work toward enough foundational contracts for a locally usable
  dashboard, basic Compose actions, manual Update and selected combinations.
  Git Deploy and broader features remain later slices. The owner authorized
  adopting obviously portable source rules and asking about material adaptations.
- [RES-002 through RES-005](../contracts/project-resolution.md) adopt selected
  approved STK-FILES-14/18/13/19 outcomes without plugin tier/path/mechanism scope.
  At that batch's creation RES-001/006 retained proposed identity/default-selection
  choices; subsequent RES-001 naming approval is recorded above. No blanket
  approval of source STK-ID drafts or standalone migration policy is inferred.
- Spec `03-compose-file.md` and `04-version-and-name.md` were read for native
  file/path and top-level project-name semantics. Those chapters do not select a
  standalone registration/pinning workflow or prove SDK invocation equivalence.
- Source `docs/ui-runtime-plan.md`, bounded runtime/transport/recovery passages,
  distinguishes runtime observations, operation results and metrics; it describes
  Nchan WebSocket metrics and proposes shared Docker events with debounced
  invalidations plus authoritative reconciliation. Bootstrap/reconnect refresh
  and rejection of obsolete responses are source design evidence, not a fully
  approved standalone live-state contract.
- Source `nchan/compose_info` producer boundaries were inspected: the CLI stats
  process is given Compose-managed container IDs and publications are coalesced
  instead of broadcasting the whole map per incoming stats line. This mechanism
  is observed evidence, not Go code to import or a selected sampling policy.
- [Official Engine API source](https://github.com/moby/moby/blob/master/api/swagger.yaml)
  was retrieved on 2026-11-19 from a moving upstream branch. The stats endpoint
  `/containers/{id}/stats` streams one container's resource statistics; `/events`
  streams lifecycle events, including container state and health events. No one
  all-container stats endpoint, event-replay guarantee or SDK support matrix was
  inferred. The plugin's one CLI stats process is not equivalent to one Engine
  API stats stream for every container.
- Owner prefers SSE, tentatively described fallback polling and wants later
  UniFi-inspired historical tracing of operations/resource incidents. [LIVE](../contracts/live-state.md)
  records scope/preferences while keeping heartbeat/fallback, collection budgets,
  retention, thresholds and idle collection proposed. Absence of lifecycle events
  is not itself proof of a failed stream. No history backend is selected now.

No source-plugin, Docker, stream/recovery or resource measurements were executed.
This is bounded adoption/research, not full source tracing, a new full-spec audit
or implementation authorization. Unrelated sibling work remains untouched.

## Auth Implementation Brief Research

2026-11-20, documentation inspection only, for the
[proposed first implementation slice](auth-implementation.md):

- The scaffold's Docker and release builds disable CGO and its container runs as
  non-root with read-only root filesystem. Persistent auth will need an explicitly
  writable, correctly owned state location, not a broad permission relaxation.
- `modernc.org/sqlite` package docs identify CGo-free operation; Go module proxy
  metadata returned `v1.62.0` as a candidate. Nothing was installed or compiled.
- SCS README/SQLite-store source documents server-side sessions and `*sql.DB`
  integration; Argon2id helper README documents configurable hashing/verification.
  These are candidate components, not an already-tested compatible dependency set.
- OWASP password-storage guidance lists the proposed 19 MiB/two-pass/single-lane
  Argon2id baseline. Parameters still require target-host benchmarking and bounded
  verification concurrency; the helper's defaults are not assumed suitable.

Links and concrete proposals are in the brief. Source branches/module metadata
are moving research references, not verified/pinned dependencies. Preparing the
brief and delivering the planning branch is authorized; implementation/default
acceptance is a separate explicit gate. No runtime tests were run for this research.

## Authentication Evidence

2026-11-19: scaffold route/configuration and security guidance were inspected;
there is no app login or session enforcement. The owner's reverse-proxy/WAF
deployment context does not select a trusted-header identity protocol or remove
the new [AUTH](../contracts/authentication.md) foundation requirement.

- [RFC 7617](https://www.rfc-editor.org/rfc/rfc7617): HTTP Basic sends Base64
  user/password credentials and is not secure without external transport
  protection such as TLS. Base64 is not encryption.
- [OWASP session management guidance](https://cheatsheetseries.owasp.org/cheatsheets/Session_Management_Cheat_Sheet.html):
  consulted session/cookie and lifecycle guidance when evaluating form/session
  login, including protected transport, cookie protections, renewal and expiry.
  This supports evaluating sessions, not claiming that merely using cookies is
  secure or selecting a session library/storage mechanism.

HTTP Basic is not the same as a simple username/password login page. After the
comparison, the owner approved local form login with server-managed sessions
(AUTH-003) and deferred OIDC. Account/bootstrap, session/security and later OIDC
protocol details remain open. No auth, proxy, browser or credential-security tests
were executed.

## Operation Queue Evidence

2026-11-19, bounded review preceding owner approval of
[OPS-001](../contracts/operation-coordination.md):

- Local Compose `02-model.md` and `deploy.md` at the pinned spec baseline:
  project identity and service rolling-update/rollback parallelism, not a manager
  request queue contract. No inspected passage chooses refusal versus queueing;
  this is not an exhaustive absence claim across all Compose documentation.
- Source `mutation-coordination.md`: distinguishes queue/admission from execution
  ownership and documents immediate busy errors, bounded shell waits and a
  separate background adapter queue. Source guarantees and participant gaps do
  not approve a universal standalone policy. No plugin runtime tests were run.
- [Microsoft queue-based load leveling](https://learn.microsoft.com/en-us/azure/architecture/patterns/queue-based-load-leveling):
  reviewed the published guidance on buffering demand, controlled processing,
  backlog/latency, durability, duplicates and ordering. The retrieved page reports
  source commit `d73cd1632ffa489eff78a16bba4d101c510a1810`. This is architecture
  guidance, not a requirement to use an Azure service or external broker.
- [GitHub Actions concurrency](https://docs.github.com/en/actions/how-tos/write-workflows/choose-when-workflows-run/control-workflow-concurrency):
  read official source/reusable sections describing serialized execution and
  pending work, including default pending replacement and version-qualified queue
  options. Those semantics are comparative evidence, not our ordering or
  cancellation defaults; no universal FIFO guarantee was inferred.

The initial recommendation favored busy refusal for implementation simplicity.
After the owner requested scenarios and evidence, the recommendation changed to
bounded queueing for unattended CI. The owner explicitly approved B, with detailed
queue policies left open. These sources were read, not deployment behavior tested.

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