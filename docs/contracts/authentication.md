+++
title = 'Application Authentication'
weight = 9
+++
# AUTH: Application Authentication

**Status:** Draft / early app-auth scope and extensible ownership direction
approved; initial mechanism and security policies awaiting review

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
2026-11-19. "Basic auth" does not yet select HTTP Basic versus local form/session
login. Bind addresses, TLS/proxy trust, bootstrap, accounts and exact route
protections remain unresolved.

**Source provenance:** Direct standalone owner decision. Plugin host authorization
is context, not a standalone login design.

**Enforcement:** Not implemented. The starter is not a secured Docker manager.

### AUTH-002: Authentication Methods Share One Application Boundary

Keep authentication-method details separate from shared access checks and app
policy, allowing future identity methods such as OIDC without duplicated
enforcement throughout business handlers. Start with the concrete mechanisms
selected for the prototype, not a speculative plugin framework or identity platform.

**Approval:** Owner requested foundations for additional auth types, including
OIDC, on 2026-11-19. Immediate OIDC delivery was tentative and remains unapproved;
no provider, library, session format or fallback mode chosen.

**Source provenance:** Direct owner direction, consistent with the approved
maintainability/clear-ownership planning direction.

**Enforcement:** Not implemented. OIDC account mapping, issuer/audience checks,
redirect trust, recovery and local/OIDC coexistence need review before support.

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
- Open: HTTP Basic versus form/session login; account model, bootstrap/recovery,
  credential hashing/storage, throttling and session lifecycle.
- Open: OIDC timing, proxy/canonical URL/TLS rules, endpoint/public-health
  exceptions, authorization, CSRF/origin protection and stream expiry/revocation.
- No auth runtime work is authorized merely by this draft.

## Verification

Scaffold routes/configuration and security guidance inspected. No login, proxy
trust, credentials, sessions, CSRF/origin or OIDC behavior implemented or tested.
Document checks cannot establish administrative security. See the
[authentication evidence](../planning/sources.md#authentication-evidence).