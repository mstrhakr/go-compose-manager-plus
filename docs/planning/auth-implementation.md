# First Implementation Brief: Local Authentication And Persistence

**Status:** Proposed concrete implementation defaults, 2026-11-20. The owner
authorized preparing this brief and delivering the planning branch. AUTH-001
through AUTH-006 retain their recorded approvals; the choices below require
acceptance of this bounded implementation slice before runtime work starts.
Merging documentation does not independently approve new product semantics.

## Deliverable

A runnable local account/session foundation: one-time env provisioning, login,
forced first-login password change, logout, restart-safe saved credentials, and
a protected dashboard shell. Preserve proper user identities for later accounts
and OIDC. No Docker socket, Compose SDK, lifecycle actions, live collectors, real
metrics, Git, OIDC, account-management UI or historical tracing in this slice.
The dashboard shell must not display invented Docker status as real data.

## Proposed Defaults

| Area | Initial choice | Reason / boundary |
| --- | --- | --- |
| Persistence | SQLite via `database/sql` and `modernc.org/sqlite`; local disk only | No separate database service; CGo-free driver fits existing `CGO_ENABLED=0` container/release builds. SQLite is proposed here, not previously selected. |
| DB lifecycle | Versioned transactional migrations, foreign keys, WAL, 5-second busy timeout, one open DB connection initially | Short serialized writes; never hold a transaction while hashing or doing network work. Verify pragmas and store compatibility; no network-share DB files or multi-instance deployment claim. |
| State location | `APP_DATA_DIR`, default `./data` for native runs; `/data` in the container | Dedicated persistent state, not project folders. Add Git/Docker exclusions and a writable volume owned by the existing non-root UID; keep the rest of the container read-only. |
| Passwords | `github.com/alexedwards/argon2id`, explicit Argon2id parameters: 19 MiB, 2 passes, parallelism 1, 16-byte salt, 32-byte key | An OWASP-listed minimum configuration, not the helper's potentially heavier defaults. Benchmark on the target VM before claiming resource fit. No custom cryptographic implementation. |
| Sessions | `github.com/alexedwards/scs/v2` plus its SQL-backed SQLite store | Established opaque server-managed sessions, distinct per device and bound to stable internal user IDs. Confirm the store works with the selected driver. No JWT/local-storage authentication. |
| Session lifetime | 30-minute idle timeout, 12-hour absolute lifetime; no remember-me initially | Concrete prototype defaults subject to tests, not new universal product promises. |
| Cookies | Host-only, HttpOnly, SameSite=Lax; Secure for the configured HTTPS public origin | No domain-wide cookie. Plain HTTP is limited to explicit loopback development, not private-LAN access by assumption. |
| Request protection | Maintained CSRF middleware, proposed `github.com/gorilla/csrf`, and canonical-origin/Host validation | Protect login, setup, logout and future mutations; SameSite is defense in depth, not the only check. Verify the selected version's proxy/plaintext behavior and vulnerability status. |
| Login work | Generic failures, bounded request bodies, one concurrent password hash per instance, bounded throttling state | Avoid leaking account existence and uncontrolled Argon2 memory use. Initial throttle proposal: 5 login attempts/minute per normalized account/client-IP key; cap keys at 1,024 with bounded expiry and reject excess work. No permanent account lockout. |

Choose and pin compatible module versions in the implementation PR; inspect
transitive dependencies/licenses and run vulnerability/build checks before
claiming these candidates are accepted dependencies. No modules are installed
by this brief. Do not write a generic storage or auth-provider framework.

## Bootstrap And Account State

Proposed inputs: `APP_BOOTSTRAP_USERNAME` and `APP_BOOTSTRAP_PASSWORD`, with no
shared default values. Require both for genuinely new auth state; missing/partial
input must refuse initialization, not expose an unauthenticated dashboard.
Existing valid state does not require bootstrap inputs, and ignores their values
for account updates, preserving AUTH-006. Do not print secrets in logs/errors.

Use a stable opaque internal user ID, separate local credential material, an
admin permission, a persisted `must_change_password` state and an auth/session
version. The one-account limit belongs to transactional bootstrap/provisioning,
not every handler. Concurrent initializers must not create two initial admins.

Only store a salted password hash, never the supplied plaintext. Environment
values can remain in Docker configuration despite process-local clearing; docs
must explain that limitation and recommend removing bootstrap env after setup.
No secret should appear in committed examples or command-line arguments.

A bootstrap login issues a restricted session: only setup/password-change,
logout and necessary public resources are available. Direct dashboard, action,
output and stream access must not bypass the gate. Propose a new password of at
least 15 Unicode code points and at most 1,024 UTF-8 bytes, different from the
bootstrap password, without arbitrary character-composition rules.

Commit the new hash, cleared setup requirement and advanced auth version in one
transaction. Rotate the completing browser's session and reject older sessions
using the user version even if physical session cleanup fails. Never acknowledge
setup success before persistence commits. Logout destroys that session and clears
its cookie; it does not log out unrelated device sessions unless a separately
defined credential-security operation requires it.

Unreadable/corrupt state or migration failure must refuse startup rather than
overwrite/reseed an installation. Complete loss/replacement of the state volume
cannot be distinguished reliably from a new install solely from missing files;
document that operational limitation and backup expectations. No automatic
recovery/reset, password-reset endpoint or silently env-driven reset is included.

## Public Origin And Proxy Boundary

Propose `APP_PUBLIC_URL` as a single canonical origin, without subpath hosting
in this slice. Native development defaults to loopback; non-loopback deployment
requires an explicit HTTPS public URL. Use that origin for cookie policy, form
origin checks and generated redirects; do not infer security from arbitrary
`Host`, `Forwarded` or `X-Forwarded-*` headers.

Trust forwarding metadata only from explicitly configured immediate-proxy CIDRs
through `APP_TRUSTED_PROXIES`; default is no trusted proxies. Validate the public
origin/Host and proxy boundary explicitly, including backend health checks and
direct-access tests. Do not accept forwarded-user headers as authentication.
The operator must prevent untrusted direct backend access. This is not a WAF
replacement, arbitrary proxy-chain support or an internet-exposure certification.

Public exceptions: login assets/form, authenticated restricted setup flow, and
minimal `/healthz` liveness without account, project or DB details. Setup is not
an unauthenticated signup route. All other app views default to protection;
unsafe operations use POST and CSRF checks. Define and test normal navigation
versus htmx denied-access responses; redirects stay within the canonical app
and never replay a failed mutation automatically. No SSE endpoint is added just
to demonstrate a placeholder auth guard; future streams must consume the guard.

## Ownership And Implementation Shape

Keep account/session application policy separate from SQL persistence and HTTP
rendering. Reuse existing config, server lifecycle, templates and test conventions;
add only concrete packages needed by this slice. Keep SQL migrations with their
storage owner. Do not create a service/interface layer per requirement ID.
Shared authenticated-request identity and setup gating are the future OIDC and
multi-account boundary, not a second implementation left beside a singleton.

Follow the existing Go/escaped-template/htmx approach. Login and setup must work
on desktop/mobile and with keyboard navigation, report errors without reflecting
credentials, support password managers, and require no explanatory landing page.
Auth UI may use the restrained UniFi-inspired direction without copying assets.

## Acceptance And Verification Gate

| Contract / risk | Required executed evidence before handoff |
| --- | --- |
| AUTH-001/002/003 | Protected default route behavior, credential verification, login/logout, cookie settings, expiry/rotation, ordinary and htmx denied access; no duplicate auth rules in business handlers. |
| AUTH-004 | Stable user/session associations and independent device sessions; one-account bootstrap policy exercised at its owner, without hard-coded username checks elsewhere. |
| AUTH-005 | Forced-change sessions cannot access protected routes; password validation and failed writes retain the gate; successful completion rotates the session and rejects prior credentials. |
| AUTH-006 | Restart after completed and incomplete setup, stale/changed bootstrap env, concurrent provisioning, failed migration/write/read and rollback cases. No env reset or reseed on invalid state. |
| Request/proxy security | Cross-origin/CSRF failures for login/setup/logout, spoofed Host/forwarding headers, configured trusted proxy, HTTP loopback versus HTTPS proxy, bounded credential-check work and secret-safe logs. |
| Persistence/packaging | SQL-store/driver integration with WAL/foreign-key/timeout checks, `CGO_ENABLED=0` build, native tests, persistent non-root container volume and restart smoke tests when Docker is available. |
| UI | Playwright desktop/mobile, keyboard, errors, expired sessions, password-change flow and no-JS form behavior; screenshots for actual implemented views. |

Run focused tests during each change, then `make check`, `make check-security`,
`make docs`, and relevant container checks for completed app work. Inspect actual
output and disclose unavailable/skipped checks. Update inherited no-dependency
docs, configuration examples and deployment instructions when those facts change.
Use only a disposable dedicated state directory; do not request the owner's real
passwords or touch existing Docker stacks for this slice.

## Research And Authorization Boundary

On 2026-11-20: modernc SQLite package docs identify CGo-free operation; module
metadata returned `v1.62.0` as a research candidate. SCS's SQLite store source
accepts `*sql.DB`; that does not prove compatibility, token storage, cleanup or
transaction behavior under our driver/configuration. The Argon2id helper documents
explicit parameters and its verification API. OWASP lists the chosen 19 MiB,
two-pass, single-lane configuration; no benchmark or security test has run here.

References: [SQLite driver](https://pkg.go.dev/modernc.org/sqlite),
[SCS](https://github.com/alexedwards/scs),
[SCS SQLite store](https://github.com/alexedwards/scs/tree/master/sqlite3store),
[Argon2id helper](https://github.com/alexedwards/argon2id),
[OWASP password storage](https://cheatsheetseries.owasp.org/cheatsheets/Password_Storage_Cheat_Sheet.html),
[AUTH contract](../contracts/authentication.md).

No dependency/build/runtime evidence is claimed from those documentation reads.
Before implementation, obtain owner acceptance of the bounded slice and proposed
defaults, then record that approval without promoting deferred or untested work.