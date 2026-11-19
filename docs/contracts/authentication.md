+++
title = 'Application Authentication'
weight = 9
+++
# AUTH: Application Authentication

**Status:** Draft / early app-auth scope and extensible ownership direction
approved; local form/session login selected, OIDC deferred; detailed account,
session and security policies awaiting review. AUTH-004 limits initial accounts
to one admin with multi-user-capable identity; AUTH-005 selects env-provisioned
credentials and a forced first-login password change in the setup wizard.

**Baseline:** 2026-11-19, product `d47835b`, documentation-only refinement.
The scaffold has public demo/health routes, no app login or sessions. No auth
dependency, configuration or runtime guard has been added.

## Purpose And Scope

Establish app authentication as a first-prototype foundation for a standalone
Docker manager, including deployment behind a local reverse proxy/WAF. Do not
inherit the plugin's host-login assumption. Transport, identity proof, sessions
and permission to administer Docker are distinct boundaries.

## Ownership

AUTH owns app authentication and authenticated identity supplied to shared access
checks. Dashboard/htmx requests, action submission, job/output readers and SSE
consume consistent policy rather than inventing separate login paths. Credential
verification or future OIDC identity sources must not own Compose business policy.

Before implementation, trace bootstrap, login, credential storage, session
issuance/validation, logout/expiry, streaming connections, reverse-proxy trust,
request/origin protection and denied access. Permissions, public exceptions,
active-stream expiry and queued-work authorization changes still need review.

## Requirements

### AUTH-001: Application Login Is An Early Prototype Foundation

Include app-level authentication/login among the first prototype foundations.
The owner's reverse proxy/WAF and local-only exposure are deployment context,
not approval to omit app authentication or blindly trust forwarded user identity.
Loopback-only access was proposed, not selected as a substitute for app login.

**Approval:** Owner explicitly requested basic login/auth in the first steps on
2026-11-19. AUTH-003 subsequently selects local form/session login rather than
HTTP Basic; AUTH-004/005 select initial account and bootstrap scope. Bind addresses,
TLS/proxy trust and exact route protections remain unresolved.

**Source provenance:** Direct standalone owner decision. Plugin host authorization
is context, not a standalone login design.

**Enforcement:** Not implemented. The starter is not a secured Docker manager.

### AUTH-002: Authentication Methods Share One Application Boundary

Keep authentication-method details separate from shared access checks and app
policy, allowing future identity methods such as OIDC without duplicated
enforcement throughout business handlers. Start with the concrete mechanisms
selected for the prototype, not a speculative plugin framework or identity platform.

**Approval:** Owner requested foundations for additional auth types, including
OIDC, on 2026-11-19. AUTH-003 defers OIDC from the initial login scope;
no provider, library, session representation or future fallback mode chosen.

**Source provenance:** Direct owner direction, consistent with the approved
maintainability/clear-ownership planning direction.

**Enforcement:** Not implemented. OIDC account mapping, issuer/audience checks,
redirect trust, recovery and local/OIDC coexistence need review before support.

### AUTH-003: Local Form Login With Server-Managed Sessions First

The initial app login is a username/password form with server-managed sessions,
not an HTTP Basic browser challenge. After local credential verification, the
session supplies authenticated identity through AUTH-002's shared application
access boundary. Dashboard, htmx, action, output and SSE access consume that
boundary rather than implementing independent credential checks.

OIDC is deferred from the initial prototype; an external identity provider is
not a prerequisite for local use. Keep the future integration boundary without
implementing a provider framework now. Authentication/session state does not
replace authorization, request/origin protection or transport security.

**Approval:** Owner selected option A on 2026-11-19: local username/password
login with sessions now, OIDC later, rather than immediate local-plus-OIDC support
or literal HTTP Basic. This does not select single-admin versus multi-user scope,
session storage technology, expiry values or a credential provisioning mechanism.
AUTH-004 subsequently settles the account scope, not those storage/security details.

**Source provenance:** Direct standalone owner decision after the HTTP Basic
versus session tradeoff discussion. See the
[authentication evidence](../planning/sources.md#authentication-evidence).

**Enforcement:** Not implemented. Credential storage, bootstrap/recovery,
session issuance/rotation/expiry/logout, cookie/proxy settings and access checks
still need bounded decisions and execution-based security verification.

### AUTH-004: One Initial Admin With Multi-User-Capable Identity Plumbing

The prototype provisions/supports one administrator account. Multiple local admin
accounts and their management UI are deferred, not permanently excluded. One
account may have independent sessions on multiple devices; limiting account count
must not collapse all sessions into a global authenticated flag.

Sessions reference a stable internal user identity, separate from mutable login
names, local credential verification and provider-specific details. Shared access
checks receive that authenticated identity rather than assuming an anonymous
singleton administrator. Future attribution can reference the same identity
without treating a display/login name as a permanent key.

Keep the initial one-account limit in provisioning/account policy, not scattered
through business handlers, job ownership or streaming access. Adding multiple
accounts later should extend the account/provisioning capability rather than
replace the session/identity contract. This does not promise zero future schema
migrations or preselect a user-ID format, database, permissions matrix, identity
provider framework or account-management screens now.

**Approval:** Owner selected one admin for the prototype on 2026-11-19, explicitly
requiring plumbing ready for the future multi-admin mode rather than a disposable
single-user implementation needing a redesign. Roles/read-only users remain out
of this decision; OIDC remains deferred under AUTH-003.

**Source provenance:** Direct standalone owner scope/maintainability decision,
not an Unraid account model imported from the source plugin.

**Enforcement:** Not implemented. Bootstrap, recovery, storage, session lifecycle,
and later account-management/revocation workflows remain to define and test.

### AUTH-005: Environment Bootstrap With Forced First-Login Password Change

Provision the initial administrator through deployment environment variables,
including Docker environment configuration, rather than an unauthenticated
browser claim of the first account or a locally generated setup-token flow.
The initial user signs in with the supplied bootstrap credentials and must
change the password through the first-run setup wizard before normal app use.

Authentication with bootstrap credentials is not permission to bypass the forced
password-change step through direct routes, action submissions, output viewers
or SSE. Initial account identity remains AUTH-004-owned, and browser login and
sessions remain AUTH-003-owned. No default shared credentials are selected.

**Approval:** Owner selected the browser setup direction with an explicit
amendment on 2026-11-19: initial user supplied through Docker env vars and forced
password update at first login, as part of setup. The prior proposed locally
obtained one-time setup credential is replaced, not independently approved.

**Source provenance:** Direct standalone owner decision. This does not select
variable names, plaintext versus other supported provisioning formats, secret-file
alternatives, persistence technology or native-service packaging details.

**Enforcement:** Not implemented. First-run detection, validation of missing or
partial bootstrap input, atomic setup/password-change completion, restart/env
reapplication, session transitions and recovery/reset policy still need definition.

## Dependencies

- Consumed by the [prototype plan](../planning/README.md#local-prototype-contract-triage)
  before a real administrative dashboard/action loop is exposed.
- Consumed by [LIVE](live-state.md) for dashboard/stream access and
  [OPS](operation-coordination.md) for action admission and job/output reads.
  Authentication does not establish project identity or permitted operation effects.
- Git/registry credentials and future CI-trigger authorization are distinct from
  app login; a browser identity is not a complete webhook/API auth design.
- Deployment/TLS/proxy trust, persisted secrets and recovery still need decisions.
  No external auth service or database is mandated here.

## Acceptance Cases

Owner-approved scope scenarios, not implemented/executed tests:

- AUTH-003: The initial user-facing login is a local username/password form with
  server-managed sessions, not HTTP Basic or a required OIDC redirect.
- AUTH-003: The local-login application can be configured without an external
  identity provider. Sensitive requests consume the shared authenticated-session
  boundary; exact route exceptions and denied-access behavior remain to refine.
- AUTH-004: One administrator signs in from two devices; sessions are distinct
  while referencing the same internal user identity, not a global login boolean.
- AUTH-004: Initial provisioning enforces the one-account scope at its owning
  boundary. User/session associations and shared access checks are not keyed to
  a hard-coded username or rewritten for every business handler.
- AUTH-004: Future multiple-account support can retain existing user/session
  references. This is a design/acceptance obligation, not a claim that multiple
  accounts or upgrade migrations are already implemented.
- AUTH-005: An initial administrator provisioned from deployment env signs in,
  enters the setup/password-change flow and cannot use normal administrative
  views or operations while the required change is incomplete.
- AUTH-005: Direct action, output and SSE requests cannot bypass that restriction
  merely because bootstrap credentials were accepted. No unauthenticated first-
  visitor claim or separate one-time setup token is the selected bootstrap flow.

Candidate scenarios, not approved detailed outcomes or run tests:

- Requests bypassing proxy/UI cannot bypass the app's selected access policy;
  forwarded identity/address headers are handled within explicit trust boundaries.
- Login failure, expired/logout sessions, direct htmx requests, action requests,
  output viewers and SSE connections have consistent denied-access behavior
  without credential or sensitive project-data leaks.
- Proxy HTTPS, cookie/request protection and canonical origin remain correct
  under the chosen deployment configuration.
- Later OIDC supplies authenticated identity through the same shared app access
  boundary without duplicating Compose operation policy.

## Decisions

- Owner expects reverse-proxy/WAF/local-network deployment and requested app
  login early, with a maintainable path to OIDC or other methods.
- 2026-11-19: Owner chose local form login with server-managed sessions first and
  deferred OIDC (AUTH-003). No auth implementation is authorized by this decision.
- 2026-11-19: Owner chose one initial admin with genuine multi-user-ready identity
  and session boundaries (AUTH-004). Prototype scope must not be used to justify
  singleton assumptions that require replacing those foundations later.
- 2026-11-19: Owner amended browser setup to use environment-provisioned initial
  credentials and a forced password change at first login (AUTH-005).
- Open: Detailed bootstrap validation, restart/reapplication and recovery,
  credential hashing/storage, throttling and session lifecycle.
- Open: Later OIDC scope, proxy/canonical URL/TLS rules, endpoint/public-health
  exceptions, authorization, CSRF/origin protection and stream expiry/revocation.
- No auth runtime work is authorized merely by this draft.

## Verification

Scaffold routes/configuration and security guidance inspected. No login, proxy
trust, credentials, sessions, CSRF/origin or OIDC behavior implemented or tested.
Document checks cannot establish administrative security. See the
[authentication evidence](../planning/sources.md#authentication-evidence).