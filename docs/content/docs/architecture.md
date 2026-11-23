+++
title = 'Architecture'
url = '/architecture/'
weight = 20
+++

## Small packages with clear responsibilities

<pre><code>cmd/server/       executable, auth wiring, signals, health probe
internal/config/  environment parsing and validation
internal/auth/    SQLite bootstrap/account state, Argon2id, server sessions
internal/web/     protected login/setup/dashboard routes and local assets
tests/            unit/API/browser checks
docs/contracts/   canonical behavioral contracts, unpublished by default
docs/content/     user guides and opt-in contributor documentation
docs/assets/      site-specific styling and assets
docs/i18n/        site translation overrides
docs/go.mod       pinned Hextra Hugo theme
docs/static/      downloadable examples
.github/          CI, opt-in publishing, dependency updates</code></pre>
<p>Keep wiring in <code>cmd/server</code> and behavior in focused internal packages.
Add domain packages when actual requirements arrive; do not start with generic
repository, service, or dependency-injection abstractions.</p>

## Runtime contract

<ul>
  <li><code>GET /</code> requires a valid application session and serves the embedded dashboard shell; no working-directory assumptions.</li>
  <li>Login and setup are form/session-based; a bootstrap session can only complete the required password change and sign out.</li>
  <li><code>GET /healthz</code> returns <code>{"status":"ok"}</code>.
    It is process liveness, not a dependency readiness check.</li>
  <li>Unsupported paths return 404; unsupported methods return 405.</li>
  <li>JSON lifecycle logs go to stdout. Configuration errors exit nonzero.</li>
  <li>HTTP timeouts bound slow clients; SIGINT/SIGTERM allow up to ten seconds for shutdown.</li>
  <li>The <code>-healthcheck</code> flag probes the configured address with a three-second timeout.</li>
</ul>

## Authentication and persistence

<p>A local administrator is provisioned once from deployment environment values,
stored as an Argon2id password hash and required to change the bootstrap password
on first login. SQLite stores the account, auth version, CSRF signing key and
server-managed sessions. Bootstrap environment values do not reset existing state.
HTTP Basic, OIDC, multiple-account UI, password recovery and Docker operations are
not part of this first auth slice. Review proxy trust, origin/CSRF rules, password
hash work limits and persistent-volume access before remote administration.</p>

## Server-driven UI

<p>Go owns business rules and renders embedded html/template layouts and partials.
htmx is pinned locally for future server-driven updates. Small JS modules handle
browser-only behavior. No client framework, bundler, or Node server is required
at runtime; login and first-run setup work as ordinary forms without JavaScript.</p>
<p>Templates parse when the handler is constructed; invalid templates fail startup.
Rendering is buffered before headers are committed. Untrusted values stay escaped.
Full-page and fragment responses vary by htmx headers to avoid cache confusion.
htmx eval, script injection, and external requests are disabled. The CSP allows
only local scripts/styles/connections. Authentication, CSRF protection,
cross-origin protection and canonical-host validation are separate layers.
Login and first-run setup work without JavaScript.</p>

## Reference projects

<p>The structure follows patterns found in
<a href="https://github.com/mstrhakr/network-list-sync">network-list-sync</a>,
<a href="https://github.com/mstrhakr/audplexus">audplexus</a>, and
<a href="https://github.com/Printmaster-Org/printmaster">PrintMaster</a>:
single-binary services, focused packages, explicit lifecycle management, and
multistage builds. No application code or domain dependencies were copied.</p>
