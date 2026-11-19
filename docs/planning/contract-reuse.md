# Contract Reuse Map

**Status:** Candidate mapping, 2026-11-19. Not a standalone approved contract,
compliance assessment, full extraction or implementation authorization.

The local source collection reports 55 candidates, 17 detailed/partial drafts,
zero wholly approved contracts and zero fully verified contracts. Individual
approved entries inside drafts retain their source scope. Source approval and
standalone adoption are separate decisions. No standalone behavioral requirement
is approved merely by appearing here.

The [source register](sources.md) pins the inspected checkouts. Source IDs remain
unchanged in this map; adopted standalone requirements should receive their own
stable IDs and link back here. Preserve Observed, Documented intent, Proposed,
Approved and Verified as distinct labels. Do not make unpublished GitHub links.

## First Connected Slice

| Source boundary / IDs | Inspected source status | Portable candidate / proposed acceptance evidence | Standalone adaptation or unresolved choice |
| --- | --- | --- | --- |
| STK-ID-01/02/07/11 | Draft; 02's saved-pin trust policy individually approved in plugin | Distinguish registration/display/path/runtime identity; preserve explicit identity across display rename; unresolved identity cannot silently retarget an action. Test two projects, rename, daemon failure and missing inputs. | No legacy-plugin naming/migration rules imported. Decide name precedence, explicit adoption, duplicates and retargeting/active-work policy. Saved metadata is not proof of Docker ownership. |
| STK-LIFE-01/02/03/05/08/11/14/18 | 01/02/03/05/08 proposed; 11/14/18 approved in plugin | Discovery/listing is distinct from deployment and ownership of existing files. Source-approved 11 says no silent placeholder in an external folder; 14 keeps an unconfigured registration visible; 18 uses directory discovery for plugin membership. | Standalone [PROJ-001/002/003](../contracts/project-membership.md) approve automatic listing beneath the selected root, initially inactive, with permission-only activation before manual or automatic changes. Activation does not run Compose, rewrite project files or enable schedules. Exact discovery/storage rules remain open. Decide empty-directory support, collisions and removal separately from plugin Delete's Down-plus-folder-delete policy. |
| STK-FILES-01/02/03/07/11/13/14/18/19 | 01/02/03/07/11 proposed; 13/14/18/19 approved with source qualifications | Keep source location separate from input selection; preserve native path anchors, file order, explicit env selection and precedence. Missing explicitly selected env blocks action rather than falling back; changing a reference alone does not edit sources. Test spaces, external paths, overrides, COMPOSE_FILE, missing files and independent interpolation env. | Do not import Git env precedence, `/mnt`/`/boot` allowlists, Basic/Advanced mode or managed label layers. Decide exact configuration fields and native default behavior before coding. Do not freeze discovery into an allegedly equivalent manual list. |
| STK-PROFILES | Draft; not fully reviewed in this staging pass | Distinguish available, configured, invocation-selected and historical profiles. Acceptance needs profile-disabled services and no silent expansion of action scope. | Full source requirement selection is pending; no invented profile IDs/defaults or automatic selection. Review this boundary before approving the first action contract. |
| OPS-LIFE-01/02/03/04/05/06/07/10/14 | Draft candidate wording, with subsequent source decisions | Give Up and Stop deliberate, native meanings; distinguish Stop, Down, Restart, recreate and Update. Assert selected-project effects and absence of unexpected teardown/volume deletion. | Up may pull/build under native Compose policy: decide this explicitly, do not claim a universal no-network guarantee. Initial proposed actions are Up/Stop only, not Update or Down. Inspect full affected requirements before action implementation. |
| STATE-LOCK-01/02/03/09/10/11 | Draft proposals with documented participation limits | One mutation owner per coordination domain; recheck identity/eligibility under ownership; stale checks cannot publish as current. Test same-project contention, different-project policy, failure and removal/recreation interleavings. | Exact lock/storage/queue/restart model is open. Project locks alone do not coordinate shared networks/volumes/files or external clients. Plugin locks/epochs are mechanisms, not mandatory Go APIs. |
| UI-OPS-01/02/05/07/09/11 | Draft, not whole-contract approval | Accepted, queued, running, completed, failed and unknown are not interchangeable. Viewer closure/timeouts do not manufacture completion. Test reconnect, delayed callbacks, rejected dispatch, restart and same-name recreation. | SDK progress is not a shell exit-code/PTY protocol. Define owned jobs and terminal outcomes appropriate to the SDK; retention/cancellation/durability remain decisions. |
| STATE-META | Partial draft with individual source decisions | Keep authoritative registration/job state distinct from cached runtime projection; unavailable is not empty. Enumerate publishers, readers, invalidators and cleanup. | Select actual requirements only after deciding what state exists. Plugin icon caches and cleanup ratios are not MVP requirements. |

Only bounded portions were inspected. This is not a claim that every row's full
document, implementation, callers, tests or approval history was traced. In
particular profiles and state metadata remain dependency-review gates. Source
tests were not executed as part of this staging task.

Standalone follow-up in [PROJ-005/006/007](../contracts/project-membership.md)
narrows root-scan eligibility to recognized Compose projects, makes discovery
on-demand or optional during setup, and adds individual folder selection without
a prerequisite scan. PROJ-001's automatic inclusion applies when a requested
scan runs, not continuously. The plugin's directory membership is not adopted
as a rule to list arbitrary data folders. Exact recognition and picker path
policies remain open.

Standalone [OPS-001](../contracts/operation-coordination.md) now owns approved
bounded same-stack queueing and serialized execution for manual/CI mutations.
Source STATE-LOCK's mixed busy errors and waits are evidence, not adopted
timeouts or queue mechanisms. Detailed ordering, cancellation, full-capacity,
restart and participant/revalidation rules remain open.

Later owner clarification in PROJ-008/009 limits the extra activation gate to
discovery-added stacks. Explicit manual additions and cloning a repository as a
new Git stack start active, and there is no deactivate option. This supersedes
earlier generalized pre-activation language, not separate automation opt-ins.

## Git And Deployment Candidates

Owner-requested [GIT-001](../contracts/git-stacks.md) adds product capability scope,
not approval of the source plugin's detailed behavior or an implementation date.
Source feature-outline sketches and inventory entries were inspected in this
pass; Git implementation/callers/tests and historical evidence were not traced.

| Source boundary | Candidate responsibility | Standalone decisions still required |
| --- | --- | --- |
| GIT-LIFE / GIT-STORAGE | Clone/source ownership, adoption, removal and filesystem safety | PROJ-008 makes successful new Git-stack creation active immediately. Managed clone versus existing checkout, root/picker paths, local files, subdirectories, deletion and failed/partial creation remain open. Unraid mount/array rules do not transfer automatically. |
| GIT-CHECK / GIT-DEPLOY | Revision preview/validation versus source preparation and deployment | Branch/tag/commit selection, Compose inputs, image policy, dirty files, partial effects and recovery; no assumed atomic runtime rollback. |
| GIT-AUTO | Separate checking/notification from authorized deployment | CI/webhook triggers are standalone requested scope, not proof of existing plugin support. Decide opt-in, auth, revision binding, delivery ordering/replay and shared job ownership; no polling default adopted. |
| CRED-VAULT / CRED-GIT | Private repository authentication and scoped credential lifecycle | HTTPS/SSH/provider mechanisms, storage/redaction, host trust, rotation/revocation and reference failures; Git credentials are not deployment-trigger authorization. |

GIT consumes PROJ-008/009's explicit-creation and no-deactivation semantics, plus
future shared identity/input/action, coordination, credentials and publication
owners. Active membership does not authorize unconfigured CI automation or
duplicate runtime policy in a webhook handler. Full source extraction and standalone policy refinement remain
pending; no source-plugin Git approval or enforcement is claimed.

Standalone [GIT-002](../contracts/git-stacks.md) now approves deployment without
per-run UI confirmation for an explicitly enabled CI integration receiving an
authenticated, authorized trigger for an active stack. Source plugin notification
or deployment modes are not adopted wholesale. Standalone GIT-003 selects the
immutable commit supplied by CI rather than latest branch head at execution;
existing provider metadata should be used where applicable. Enablement mechanism,
provider/event mapping, repository/ref authorization, authentication details,
replay/order and contention remain open.

Standalone GIT-004 selects GitHub as the first CI integration. The GitLab research
above remains comparative evidence, not an initial-delivery requirement. This
provider priority does not define repository-host restrictions or add a generic
multi-provider framework; exact GitHub event mapping still needs refinement.

GIT-005 further requires explicit deployment intent from CI rather than treating
workflow success as a deployment request. This is a standalone owner decision,
not an adopted plugin scheduling rule or a chosen GitHub wire protocol.

## Network And Volume Candidates

No existing volume-management contract was supplied. Network-plugin behavior is
observed/documented evidence, not an approved standalone contract. Proposed new
standalone boundaries below have no requirement IDs yet; assign stable IDs during
actual extraction/refinement rather than inventing approved contracts here.

| Boundary | Evidence and proposal | Acceptance scenarios to agree |
| --- | --- | --- |
| Network inspection/protection | Network README and ExecFunctions describe system/default protection, current attachments and meaningful errors. Consider immutable resource IDs plus names/labels/ownership and fresh checks. | Docker unavailable versus zero networks; connected/protected resource; delete/recreate with same name; shared/external Compose network; secret-bearing options redaction. |
| Attachment policy | Unraid persists Additional Networks, with stopped-container changes pending recreation; direct Engine attachment is not equivalent to editing Compose configuration. | Runtime-only attachment later undone by Compose; stopped container; aliases/static IP; container ID change; external owner; persistence failure after runtime success. Decide whether this is supported at all initially. |
| Volume inspection/removal | Compose identity evidence includes labelled volumes; Engine volume APIs and spec provide mechanisms, not product deletion policy. | Running/stopped container consumers; anonymous/named/external/shared/plugin-backed volumes; same-name replacement; daemon errors; concurrent external use; irreversible partial effects. |
| Shared-resource coordination | Project operations and host-resource changes can intersect multiple owners. Centralize policy; no independent resource-action bypass around project safety. | A project Up races resource remove; two projects share a network/volume; use changes after preview; external Docker clients race after our check. Define conservative refusal and residual limits, not an impossible atomic host-wide guarantee. |

System-style Unraid network-name heuristics, template XML, version thresholds,
native Additional Networks and Post Arguments conflict rules are adapter evidence.
Do not carry them over as generic Ubuntu rules. Do not claim a safe-to-delete
resource from a label or stale attachment count alone.

## Before A Contract Becomes Implementable

1. Read the selected source entries and their latest approval/supersession sections.
2. Recover the smallest connected standalone behavior; label deliberate differences.
3. Obtain owner decisions on material ambiguities; record date and exact scope.
4. Add a standalone contract under `docs/contracts/`, using the inherited format
   and stable IDs, with one owner per shared guarantee and reciprocal dependencies.
5. Convert the selected outcomes into implementation-independent acceptance tests,
   including negative effects and deterministic concurrency/restart interleavings.
6. Implement only the authorized slice, then record actual checks, revision,
   environment and residual limits separately from approval.

Source PHP/shell code is a case study, not code to port. Scope extraction must
not silently authorize a shared Go service for the existing plugin or changes
in any sibling repository.