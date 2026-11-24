+++
title = 'Deployment and publishing'
url = '/deployment/'
weight = 40
+++

## Docker Compose

<pre><code>cp .env.example .env
docker compose up --build -d
docker compose logs -f
curl http://localhost:8080/healthz
docker compose down</code></pre>
<p>Before first start, set temporary <code>APP_BOOTSTRAP_USERNAME</code> and
<code>APP_BOOTSTRAP_PASSWORD</code> values in the ignored local <code>.env</code>
file. Sign in through the browser and complete the forced first-login password
change, then remove those bootstrap values and restart. Reapplying or changing
bootstrap env after provisioning does not reset saved credentials.
Compose reads <code>.env</code> for the host port, log level, public origin,
trusted proxy CIDRs and bootstrap values; native Go execution reads exported
variables, not dotenv files. The app persists SQLite state in the named
<code>app-data</code> volume mounted at <code>/data</code>. Back up this local state
consistently. Network-shared databases and concurrent multi-instance operation
are unsupported. Loss of the state volume has no automatic password-reset
recovery flow. On Windows, Unix permission bits do not enforce database
privacy; restrict access to the data directory using Windows access controls.</p>
<p>The data directory contains <code>app.db</code> for accounts, settings and
discovery, <code>metrics.db</code> for stats/state/event history, and
<code>logs.db</code> for collected container output. History files are opened
only when their sources need them. Back up the directory consistently,
including SQLite sidecars while applicable; copying only an active main database
file can miss committed WAL data. Protect log history as potentially sensitive
application output. Legacy log rows in <code>metrics.db</code> are not silently
moved or deleted by this prototype; automatic cross-store migration is not
implemented.</p>
<p>The app's built-in default listener is loopback-only for local use. For reverse
proxy access, set <code>APP_PUBLIC_URL</code> to the external HTTPS origin and
<code>APP_TRUSTED_PROXIES</code> to only the immediate proxy CIDRs. Prevent direct
untrusted access to the backend; forwarded identity headers are not accepted as
login credentials. Compose's example <code>.env</code> binds the host port to
loopback. The multistage image contains a static binary and CA certificates, runs as
UID/GID 65532, and provides a built-in health check. Compose uses a read-only
filesystem, drops capabilities, and prevents privilege escalation. No secrets,
source files, shell, or docs are included in the runtime image.</p>
<p>Do not cache application pages or <code>/static/</code> responses at the
reverse proxy; preserve the app's <code>Cache-Control: no-store</code> header.
After an upgrade, a cached stylesheet or script can leave new page content
misaligned even after a browser refresh. If that happens, disable the proxy's
cache for the app and purge its existing cached responses, then reload.
A private browser window does not bypass a reverse proxy cache.</p>

## Container registry (off by default)

<p>After explicit setup, relevant passing protected-main CI triggers dev images.
Only successful main push runs from this repository's CI are accepted, never
PR runs or arbitrary artifacts. A read-only job builds/scans/smoke-tests; a
separate job loads and pushes the image without executing it; a separate OIDC
job attests its digest. Initial images are Linux x64. No Docker socket is
exposed to the scanner; its image is digest-pinned.</p>
<p>Registry names are lowercase. Dev uses <code>dev</code> and
<code>sha-FULL_COMMIT_SHA</code>; beta/RC use their exact version tags plus
<code>beta</code>/<code>rc</code>; only normal moves <code>latest</code>.
Do not deliberately overwrite version/SHA tags. Roll back by an exact version
or digest only when config/data remain compatible. GHCR package visibility is
separate from repository visibility; check package access explicitly.</p>

## GitHub setup for each repository

<p>Template-created repos do not inherit live settings. Authenticate GitHub CLI
outside chat, review this public-repo setup, then explicitly apply it:</p>
<pre><code>node scripts/setup-github.mjs --repo OWNER/PROJECT
node scripts/setup-github.mjs --repo OWNER/PROJECT --apply
# Only when publication is authorized:
node scripts/setup-github.mjs --repo OWNER/PROJECT --apply --enable-publishing</code></pre>
<p>The script never changes visibility or buys plan features. It requires admin
access, a public repo and main as the default branch. It enables Issues, private
security reporting, secret scanning/push protection, Dependabot alerts/fixes,
squash-only PRs, the required <code>CI gate</code>, immutable version tags,
read-only Actions defaults and SHA-pinned allowlisted Actions. It disables Wiki,
Discussions, dependency auto-merge and routine ruleset bypass.</p>
<p>Setup preserves the repository's existing template status; it does not mark
new projects as templates. Set that GitHub setting separately if you intend to
create another reusable template.</p>
<p>The release environment requires the applying maintainer's approval and
protected-branch deployment. Self-review remains allowed for a solo maintainer;
this is an intentional approval pause, not two-person security. GitHub combines
bot-PR creation and approval in one Actions settings switch; the script enables
it so Release Please can create PRs, not to authorize automatic approvals.</p>
<p>Without <code>--enable-publishing</code>, release/container variables are false.
With it, <code>RELEASES_ENABLED</code>, <code>PUBLISH_CONTAINER</code>, and the
verified-setup acknowledgement <code>RELEASE_APPROVAL_CONFIGURED</code> are true.
Keep the environment protected; that acknowledgement is not itself enforcement.
Docs publication remains separately off. Apply local changes through a feature
branch/PR; do not bypass the new main rule to bootstrap them.</p>

## Versions and release channels

<p>Initial normal version: v0.1.0. Before v1, feat/fix increment patch;
breaking changes increment minor. Docs/test/chore-only work does not ordinarily
release the app. Entering v1 requires an explicit policy/config change.</p>
<p>Release Please opens/updates version/changelog PRs. Its built-in token does
not automatically trigger PR workflows, so a separate scoped dispatch job starts
CI on the generated branch. Merging the release PR creates a draft release;
full validation, packaging, checksums, attestations and release-environment
approval precede publication. A failed stage leaves a visible failed run/draft,
not an implied successful release. Do not manually publish that draft to skip gates.</p>
<p>For beta/RC, run <strong>Releases (opt-in)</strong> manually on main, selecting
channel, exact tag (for example v0.2.0-beta.1 or v0.2.0-rc.1), and full 40-character
main-history commit SHA. Branch names are rejected. RC means intended final scope;
new features return to beta. These prereleases are optional for small fixes.
Final release source/build changes require validation, not blind promotion of a
moving candidate alias. Release numbering is independent from Go toolchain versions.</p>
<table>
  <thead><tr><th>Stage</th><th>Validation</th></tr></thead>
  <tbody>
    <tr><td>PR</td><td>Relevant unit/API/browser/security/docs/contracts checks</td></tr>
    <tr><td>Dev</td><td>Passing main CI, exact-source image build, scan and smoke test</td></tr>
    <tr><td>Beta/RC</td><td>Full current readiness suite and packaging; RC intended final behavior</td></tr>
    <tr><td>Normal</td><td>Same full readiness, final artifact checks, explicit approval and publication</td></tr>
  </tbody>
</table>
<p>The starter suite is small, so beta/RC currently share the full readiness
suite. Add slower integration/upgrade/rollback cases as those features exist;
never invent empty stage gates. Security boundaries apply at every stage.
Native CI covers declared OS/architecture targets; local cross-builds do not
establish native Windows/macOS support. No desktop framework is included.</p>

## Verify downloads and images

<p>GoReleaser produces Linux/Windows x64 and macOS x64/ARM64 downloads, checksums
and embedded version/revision metadata. Use <code>server -version</code> locally.
Verify checksums and provenance with the repository/workflow you expect:</p>
<pre><code>sha256sum --check checksums.txt
gh attestation verify FILE --repo OWNER/PROJECT --signer-workflow OWNER/PROJECT/.github/workflows/publish.yml
gh attestation verify oci://ghcr.io/owner/project@sha256:DIGEST --repo OWNER/PROJECT --signer-workflow OWNER/PROJECT/.github/workflows/image.yml</code></pre>
<p>For a manually selected source revision, the triggering workflow revision may
differ from packaged source. Verify the additional signed source-revision receipt
and inspect its repository/revision/tag/run against your expectations:</p>
<pre><code>gh attestation verify FILE --repo OWNER/PROJECT --predicate-type https://github.com/mstrhakr/go-template/source-revision/v1 --format json</code></pre>
<p>The predicate URI identifies this schema, not a required upstream signer.
Verify your own repository/workflow identity. Attestations are origin evidence,
not proof that upstream code or an authorized workflow is harmless. Publication
and consumer-side verification must be exercised before claiming end-to-end
verification; local snapshot builds alone do not provide that evidence.</p>

## Support, retention and recovery

<p>Initially support the latest normal release; dev/beta/RC are test channels.
Retain normal and prerelease versions. Target about 30 days of dev registry
history with cautious manual cleanup; do not delete referenced release digests.
CI reports/build handoff artifacts expire after seven days. No deletion bot is
given broad package write access. For incidents, stop publishing, revoke affected
access, review trusted source/workflows, rebuild, and restore deliberately.</p>

## GitHub Pages (off by default)

<p><strong>A private repository does not guarantee a private Pages site.</strong>
Pages availability and private-site access controls depend on your GitHub plan.
Treat publishing as public disclosure unless you have explicitly verified access controls.
No repository visibility or Pages setting is changed automatically.</p>
<ol>
  <li>Review the generated content and remove anything that must remain private.</li>
  <li>When ready, select GitHub Actions as the Pages source in repository settings.</li>
  <li>Set the Actions variable <code>PUBLISH_DOCS</code> to <code>true</code>.</li>
  <li>After setup, successful CI runs on default-branch pushes publish the site
    only when documentation changes. A manual default-branch run remains
    available and requires explicit publication confirmation. Optionally require
    approval on the <code>github-pages</code> environment.</li>
</ol>
<p>The workflow derives the base URL from GitHub Pages, so project paths and custom
domains do not require hardcoded repository names. Preview locally with
<code>make docs-serve</code> while publishing remains disabled. Changing either
opt-in variable back to false prevents future runs, but does not delete already
published sites, packages, or artifacts.</p>
<p>Contracts stay excluded unless <code>PUBLISH_CONTRACTS=true</code>. Contributor
pages stay excluded unless <code>PUBLISH_DEVELOPMENT_DOCS=true</code>. Preview
them locally with <code>make docs-contracts</code> and
<code>make docs-development</code> before enabling either variable. Review both
sets separately; enabling one does not enable the other.</p>
