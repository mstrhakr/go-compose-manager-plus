# Product Plan

**Status:** Proposed architecture and delivery sequence, not implementation
authorization. Updated 2026-11-19. This repository will contain the end product.

## Owner Decisions

- Start from the published main branch of `mstrhakr/go-template`.
- Initialize and push `mstrhakr/go-compose-manager-plus` as the product repository.
  The owner approved public visibility on 2026-11-19 so the full GitHub
  protection baseline can be configured without a private-repository plan upgrade.
- Keep the MIT license. The repository name is selected; public product branding
  remains open.
- Build toward a lightweight standalone counterpart of the Unraid Compose Manager
  Plus plugin for constrained Ubuntu Docker hosts. The plugin is the functional
  baseline, not merely a case study: reuse useful workflows and portable approved
  business rules, adapting host integration and explicit standalone differences.
- Use a modern standalone UI, with UniFi as the design/interaction inspiration,
  rather than reproducing the plugin's Unraid-native presentation. This product
  direction does not require the entire plugin feature set in the first prototype
  or approve unresolved source behavior and legacy compatibility machinery.
- Include network and volume management in product planning. Their exact feature
  set, safety policy and delivery order are not yet approved.
- Include first-class Git stacks: repository acquisition/pulls, private-repository
  credentials and CI-triggered deployment through webhooks or deployment APIs.
  [GIT-001](../contracts/git-stacks.md) approves capability scope only; security,
  sequencing and delivery order still need refinement. New Git-stack creation
  starts active under PROJ-008; that does not itself request deployment.
- Offer on-demand discovery, optionally during setup, of recognized Compose
  projects in direct child directories of the selected root. Requested scans
  automatically list matches, initially inactive; there is no continuous
  background discovery. Also allow selecting stack folders one at a time through
  a folder browser without scanning first. Discovery-added stacks require
  explicit activation before manual or automatic project changes; manual additions
  and cloned new Git stacks start active. There is no deactivate option.
  Activation grants permission only: no Compose execution,
  project-file rewriting or schedule enablement. See
  [PROJ-001 through PROJ-009](../contracts/project-membership.md). Exact input
  recognition, picker path policy, other remaining discovery details and
  activation validation/persistence remain unresolved; no implementation is
  authorized by these decisions.
- This phase is planning and staging only, not a plugin rewrite, shared-backend
  migration or authorization to implement the entire source roadmap.
- Prioritize maintainability and clear ownership boundaries within the standalone
  app. Possible future reuse is a by-product, not a cross-product requirement;
  no plugin rebuild or shared-backend delivery is planned. See the
  [architecture direction](#maintainability-and-ownership-direction).
- The VM manager script is unavailable on this machine. The owner can upload a
  redacted copy to `scratch/`; inspecting it remains pending, not completed.

The handoff's explicit-registration-only suggestion is replaced by requested
discovery plus individual folder selection under PROJ-001/006/007. Suggested
single-host scope, embedded UI, low background
activity and bounded concurrency remain proposals below. Neither
the scaffold nor source-plugin approval silently approves standalone behavior.

## Current Stage

The runnable Go/htmx template is staged, with its module and delivery metadata
renamed. The greeting page remains a template demonstration, not a manager UI.
No Docker dependency, daemon socket, registration store, credential store,
management route, job runner or migration has been added. Publishing stays off.

Read [contract reuse](contract-reuse.md), [source evidence](sources.md), and
[verification](../verification.md) alongside this plan. The template's development,
architecture and deployment pages describe the scaffold, not proven management
capabilities.

## Proposed Architecture

One Go service, embedded escaped HTML, self-hosted htmx and small local JS modules.
Keep the inherited frontend rather than add a competing client framework.
Propose one local Docker host per installation; multiple VM installations need
not imply a centralized controller. Persistence and deployment choices remain open.

```text
Web UI / application API
          |
Registration + resolved project description
          |
Operation policy + owned jobs + coordination + state publication
          |
Compose SDK adapter                 Docker Engine client adapter
(project orchestration)             (network/volume inspection and approved actions)
          |                                      |
          +------------- local Docker Engine ----+
```

These are responsibility boundaries, not instructions to create a package,
interface or service for each box. Start with concrete use cases and add only
the structure they need. SDK adoption does not own registration, permissions,
credential handling, locks, job identity, recovery or truthful UI state.

Compose-go loads and validates project models; it is not the orchestration
engine. Engine API does not provide a Compose-project REST service. The Compose
SDK embeds orchestration in this process; any application HTTP API is ours.
Prefer native Compose semantics over reimplementing orchestration or silently
changing input selection, profiles, pull/build policy or resource ownership.

### Maintainability And Ownership Direction

**Owner-approved planning direction, 2026-11-19:** clean boundaries now,
extraction only when justified by real needs. Keep project policy and operations
independent of HTTP/HTML and platform-specific integration, with one authoritative
owner for each shared responsibility. Consumers use that owner rather than
duplicating policy or adding a facade over competing implementations.

Prefer one application and concrete implementations. Add interfaces, packages or
services only when actual complexity or callers justify them, not to prepare a
hypothetical second product. No separate shared-library repository, versioned
plugin protocol, extension framework or compatibility promise is approved.

The owner does not currently intend to rebuild the Unraid plugin. Shared-service
packaging and plugin migration are not product goals or prerequisites for the
standalone application; any future proposal requires separate evidence and
approval. This direction authorizes planning, not implementation.

## Delivery Gates

No schedule or effort estimate is committed. Contract adoption/refinement is the
current work; management implementation still requires bounded authorization.

| Stage | Bounded deliverable | Exit evidence / approval gate |
| --- | --- | --- |
| 0: Foundation | Independent product repo, runnable template, source provenance, decisions and proposed backlog | Scaffold checks and public GitHub protection readback; no management behavior |
| 1: Definition | Inspect the uploaded script; choose representative Ubuntu VM, workflows, resource budgets and security/exposure boundary | Owner approves the next slice and individual standalone requirements |
| 2: Read-only spike | Resolve an existing project's exact identity, ordered inputs, interpolation env and profiles; optionally inspect its runtime state | SDK dependency/build/tooling evidence; no file changes, pulls, builds or Docker mutations; compare results with native Compose fixtures |
| 3: First vertical slice | Discover and list an existing project, inspect it, explicitly activate it before deliberate Up or Stop | Consume PROJ-001/002/003; separately approve discovery details, activation validation/persistence, action options, auth, job ownership, contention and restart semantics; deterministic tests plus disposable-host evidence |
| 4: Resource inspection | Network/volume list and details, attachments and Compose ownership/external-use evidence | Clear unavailable/unknown states; no sensitive options leaked; bounded daemon requests and refresh activity |
| 5: Deliberate resource actions | Selected network/volume create/remove and any approved attachment workflow | Separate destructive-action contracts, coordination, fresh identity/use checks, confirmations, partial-failure tests and recovery evidence |
| 6: Deployment | Native Ubuntu service and container evaluation | Path parity, permissions, credentials, data lifecycle, self-management/recovery and measured manager overhead on the target VM |

The original stage table is a dependency overview, not the current prototype
scope or an instruction to complete every workstream before local use. The
near-term triage below takes priority. No SDK dependency is installed now.

## Local Prototype Contract Triage

**Owner direction, 2026-11-19:** do enough foundational contract work to start a
locally usable dashboard/action loop, then refine using implementation and host
feedback. Reuse portable source business rules, preserve approval/supersession
qualifications, and ask about consequential adaptations or uncertain behavior.
This does not authorize runtime implementation or sibling-plugin changes.

The standalone product should remain recognizably Compose Manager Plus in its
useful workflows, without preserving implementation defects or Unraid-specific
constraints merely for resemblance. Functional reuse and a redesigned UI do not
imply a shared backend, plugin rewrite or drop-in plugin migration compatibility.
Already approved standalone deviations remain authoritative.

Prototype target: status dashboard home, manual addition/on-demand discovery,
established activation rules, Up/Down/Restart, manual Update and an explicitly
selected small set of combined actions. Exact combinations and action details
still need the lifecycle/update adoption batch. Git Deploy/acquisition/private
credentials/webhooks, network/volume management and history are later delivery
slices, not removed product goals.

| Order | Small batch | Ready-to-implement outcome |
| --- | --- | --- |
| 1 | Identity/inputs, [RES](../contracts/project-resolution.md) | Adopt safe source/env rules; settle identity acceptance and initial selection/profile scope; map to native SDK loading. |
| 2 | Lifecycle/manual Update | Reuse approved action meanings, preparation barriers and stopped/never-deployed safety; decide exact prototype combinations and remaining proposals. |
| 3 | Jobs/[OPS](../contracts/operation-coordination.md) | Adopt browser-independent outcomes; settle minimum queue order/capacity/full and interruption behavior before mutations, not every future CI variant. |
| 4 | Dashboard/[LIVE](../contracts/live-state.md) | Agree visible runtime/operation states and bounded live/fallback behavior; measure collection cost. |
| 5 | Local access/persistence | Establish Docker/path scope, loopback versus authenticated remote access, request protection and saved membership/identity recovery. |

Use the read-only SDK/resolution/dashboard slice as the first implementation
checkpoint, then add deliberate actions on a disposable Docker target. The
unauthenticated starter binds all interfaces by default; local installation alone
is not a secure exposure boundary. Implementation needs approval of its bounded
slice. Inspect the VM script if supplied, without blocking unrelated fixture work
or inventing script behavior.

### Live State And Future History

UniFi is the owner's visual/interaction reference: an operational dashboard,
coherent live views and later correlation of operations with resource incidents.
Do not make a marketing home page or import branding/assets. Go/htmx rendering
remains the foundation.

SSE is preferred for browser updates; LIVE keeps fallback/reconnect details
proposed. Docker events plus authoritative reconciliation serve runtime state;
metrics and operation outcomes are distinct inputs. No events may mean nothing
changed, not a dead connection. The Engine stats API is per container; shared SSE
delivery does not remove collection cost. Missing metrics do not prove stopped
containers. Do not duplicate complete collectors per tab; viewer-aware collection,
limits and any safety polling require measurement/review, not constant discovery.

For later history, distinguish identity/lifetime, operation intent/outcome and
timestamps from disposable current projections and metric samples. These are
proposed boundaries, not a mandate for an event store, time-series service,
retention policy or always-on idle collection now. History/incident detection
must be separately approved and measured.

## Git Stacks And CI Deployment

Git-backed stacks are owner-requested product scope, not a blanket-deferred
capability. Their delivery order relative to the first Compose slice and resource
management is open. Do not equate adding this workstream with implementing it now.

The [GIT draft](../contracts/git-stacks.md) separates repository acquisition,
private authentication, source update/validation and deliberate deployment.
Manual and CI/webhook callers must consume shared project/action policy,
coordination and jobs rather than become competing deployment owners.

**Owner-approved CI behavior:** once CI deployment is explicitly enabled for an
active stack, an authenticated, authorized trigger submits deployment without
another per-run UI approval ([GIT-002](../contracts/git-stacks.md)). Active status
alone does not enable CI. Other action/security checks remain required;
acceptance is not completion. OPS-001 selects bounded same-stack queueing;
detailed queue behavior is still unresolved.

CI deployment targets the immutable source commit selected by CI, not a newer
branch head when execution starts ([GIT-003](../contracts/git-stacks.md)). Use
existing provider/event metadata where applicable. The initial CI integration
targets GitHub ([GIT-004](../contracts/git-stacks.md)); GitLab/other providers
are potential later extensions, not part of that initial slice. This is not a
GitHub-only repository-hosting rule. Exact GitHub event/API mapping,
repository/ref authorization and event-success rules still need definition.
This pins Git source only, not mutable image tags or all build dependencies.

CI explicitly requests deployment when it decides to promote its selected commit
([GIT-005](../contracts/git-stacks.md)). Workflow success alone does not trigger
deployment. This selects intent, not the exact GitHub event/API or a custom
payload format; authentication and deployment eligibility remain separate gates.

Before implementation, refine repository/ref/subdirectory and input selection,
clone ownership and local changes, private credential types/storage/redaction,
host trust, trigger authentication/authorization, provider revision mapping,
replay/idempotency, job contention and partial failure/recovery. Credential storage
and network/command/path safety require explicit treatment; a Docker-capable
deployment trigger is an administrative interface, not a generic shell hook.
These are decision gates, not silently approved mechanisms or defaults.

Activation remains permission-only for discovery-added stacks; new Git stacks
start active through explicit cloning, as do manual additions. There is no
deactivate action. Partial creation/adoption policies still need refinement.
Active status alone is not automation opt-in. Repository
credentials are not webhook or image-registry authorization. No scheduled polling,
blanket deploy-on-push default for all stacks, provider-specific OAuth flow or
general-purpose credential vault is approved by the broad capability request.
GIT-002 separately settles unattended behavior for explicitly enabled CI delivery.

## Network And Volume Workstreams

| Area | Candidate initial scope | Decisions before mutation |
| --- | --- | --- |
| Networks | Inspect driver, scope, IPAM, labels, attached containers, system/Compose/external classification | Which drivers/options; duplicate names and immutable IDs; protected resources; attachment persistence versus temporary runtime drift; create/remove ownership and connected-resource refusal |
| Volumes | Inspect driver, labels, mount references, running and stopped container consumers, Compose/external classification | Named/anonymous/plugin-backed scope; fresh in-use checks; irreversible deletion warning; no default force removal; driver option secrets; backup expectations |
| Shared resources | Show cross-project/external references without claiming exclusive ownership | Coordinate manager participants across projects and resource actions; recheck under ownership; external Docker/Compose users remain outside manager locks |

Propose inspection before create/remove. No blanket prune, forced disconnect,
volume-content browser, backup engine, implicit volume removal with project Down,
or automatic changes to Compose-authored resources are in the initial slice.
Bind-mount data is host filesystem data, not a named-volume deletion target.
Absence of containers or Compose labels does not prove a resource is safe to delete.
Container filesystem paths are not portable host paths or permission to expose data.

The network plugin is useful evidence for protected networks, attachment outcomes,
pending versus effective state and actionable errors. Its Unraid XML templates,
version checks, Post Arguments handling and `/boot` paths are not standalone
requirements. Volume-management behavior needs fresh definition: that repository
does not provide an equivalent volume manager.

## Safety And Ownership Before Execution

**Owner-approved contention policy:** competing same-stack manual and CI
mutations use bounded, visible queueing rather than immediate busy refusal when
waiting capacity is available. Only one mutation executes for that stack at a
time. [OPS-001](../contracts/operation-coordination.md) owns this shared policy;
queue size, ordering, cancellation, failure, full-capacity and restart behavior
remain open. No external broker or persistence technology is selected.

Proposed invariant: accepted work is bound to the selected registration lifetime,
exact runtime project and input description; only its current owner may publish
its outcome. Viewer closure is not cancellation or successful completion. A failed
or interrupted operation reports known partial effects instead of inventing rollback.

Before building jobs or persisted runtime state, enumerate every participant:
HTTP submitter, server executor, status/output readers, refresh publisher,
registration/input editor, removal/recreation, shutdown/startup recovery and
acknowledgement/retention. Future CLI, schedulers and other tabs must consume the
same authoritative policy, not become competing owners. External editors and
Docker clients do not participate in our locks. Restart policy and retention are
open; do not silently promise durable resumption.

Docker socket access is effectively host-administrative. Decide authentication,
authorization, CSRF/origin protection, exposure/TLS and secret handling before
adding reachable mutating routes. The starter is unauthenticated and native
execution binds all interfaces by default; use loopback during staging. Reverse
proxy TLS alone is not authentication. Never commit scripts, env contents, Docker
credentials or resolved secret-bearing configuration as test evidence.

Container deployment must distinguish daemon-host bind sources from paths inside
the manager, including local build contexts, env files and credential helpers.
Evaluate least-privilege socket access deliberately; do not blindly add root or a
socket mount to the starter Compose file. Self-container recreation and native
service interruption both require recovery decisions before claiming support.

## Measurement Plan

Agree numerical pass/fail targets before describing the product as lightweight.
Record VM vCPU/RAM, Ubuntu/kernel, Engine/API versions, project/service/resource
counts, packaging, toolchain, warm/cold state and measurement duration.

Measure idle RSS/CPU, binary/image size, startup latency/peak memory, daemon
requests with no viewers, refresh requests and latency with one/multiple viewers,
and operation-time manager RSS/CPU. Include missing-daemon, slow-daemon and
operation-contention behavior. Compare against the same host/workload without
the manager. Separate manager overhead from daemon/build/pull/extraction costs.

Propose on-demand or viewer-aware refresh, batched Docker reads, bounded output
retention, request deadlines and bounded operation concurrency. Values, cadence,
storage and detailed queue policy are undecided; bounded same-stack queueing is
approved under OPS-001. SDK import/build success alone is not
resource verification.

## Deferred Unless Separately Approved

Centralized multi-host control, Kubernetes, clustering, marketplace, automatic
image updates/pulls/builds, a general-purpose credential vault, editor/history/restore,
bulk operations, terminal emulation, plugin replacement, migration compatibility
and shared-plugin Go backend are not part of the proposed first slice. Keep them
as source context rather than importing a platform-sized roadmap. Git acquisition,
private credentials and CI deployment are separately requested workstreams above;
their defaults and exact policies still require approval.

## Next Owner Review

1. Inspect the redacted script when uploaded; identify the few workflows worth
   preserving without assuming its format or credentials are appropriate.
2. Select a representative VM and measurable resource targets.
3. Agree whether the read-only spike includes runtime and network/volume inspection
   or only configuration resolution; approve standalone acceptance cases.
4. Decide security/exposure, persistence and packaging only as needed for that
   slice. Auth and destructive-resource questions must precede mutation delivery.