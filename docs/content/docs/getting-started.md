+++
title = 'Getting started'
url = '/getting-started/'
weight = 10
+++

## Run locally

<p>Install the Go version specified in <code>go.mod</code> (currently 1.27.4).
Set a temporary bootstrap account without placing its password in shell history:</p>
<pre><code>read -rp 'Admin username: ' APP_BOOTSTRAP_USERNAME
read -rsp 'Temporary admin password: ' APP_BOOTSTRAP_PASSWORD
printf '\n'
export APP_BOOTSTRAP_USERNAME APP_BOOTSTRAP_PASSWORD
make run</code></pre>
<p>Open <code>http://127.0.0.1:8080/</code>, sign in and complete the required
password change. Discovery is available on request; Docker inventory is disabled
by default and management actions are not connected. Stop the server with Ctrl+C.</p>

## Run in Docker

<p>Install Docker Engine and Docker Compose. The
<a href="../examples/compose.yaml">example Compose file</a>
uses the planned <code>latest</code> image tag, which is not yet published.
Until a normal release exists, use the Linux amd64 <code>dev</code> tag for
testing instead; it can change and is not a production release. Save the example
as <code>compose.yaml</code>, change its image tag to <code>dev</code>, and
create a private <code>.env</code> file containing
<code>APP_BOOTSTRAP_USERNAME</code> and a strong temporary
<code>APP_BOOTSTRAP_PASSWORD</code>. Do not commit the file or place the
password in shell history. For example, edit the file privately and replace
both placeholders:</p>
<pre><code>APP_BOOTSTRAP_USERNAME=replace-with-admin-name
APP_BOOTSTRAP_PASSWORD=replace-with-strong-temporary-password</code></pre>
<p>Then start the app:</p>
<pre><code>chmod 600 .env
docker compose pull
docker compose up -d
docker compose logs -f</code></pre>
<p>Open <code>http://127.0.0.1:8080/</code> and complete the forced password
change. Compose binds the host port to loopback and stores account/session state
in its persistent <code>app-data</code> volume. Remove the bootstrap values from
<code>.env</code> after setup and run <code>docker compose up -d --force-recreate</code>
to remove them from the container environment. Docker status is disabled
by default; management actions are not connected.</p>

## Discovery and Docker inventory

<p>Mount a trusted project root into the container and save its in-container
path as the discovery root. Requested scans examine direct-child directories.
Discovered projects begin inactive; activation grants permission only and does
not start or modify containers. Project paths shown in the app are container
paths. A read-only project mount prevents writes but is not a read-access
sandbox for Compose references outside the project directory.</p>
<p>Docker inventory requires both <code>APP_DOCKER_SOCKET</code> and a socket
mount. The app user must have permission to connect: inspect the host socket's
numeric group with <code>stat -Lc '%g' /var/run/docker.sock</code> and, if its
group has write permission, add that ID under <code>group_add</code> and recreate
the service. Inspect socket ownership and mode with
<code>stat -Lc 'uid=%u gid=%g mode=%a' /var/run/docker.sock</code>.
Docker API access is effectively host-administrative; a <code>:ro</code> socket
mount does not make API access read-only. Only grant it inside a trusted security
boundary. Snapshots list running and stopped containers but do not provide logs,
live metrics or lifecycle operations.</p>
<p>On the Projects and Containers pages, use <strong>Customize columns</strong>
to choose visible columns and restore the defaults. Preferences are saved
separately for each view in your browser; the identifying column always stays
visible. Without JavaScript, all columns remain visible.</p>

## Metrics and container observability

<p>After configuring Docker access, open
<strong>Settings &gt; Containers</strong> to find <strong>Observability settings</strong>.
Choose Low (activated stacks every 15 seconds), Medium (activated stacks every
5 seconds), High (all running containers every second), or Custom to choose the
scope and interval. Enter an explicit storage target, check
<strong>Enable background metrics collection</strong>, and save.
Selecting a profile alone does not enable collection. Storage units are decimal;
the target is not a hard disk quota. Suggested sizes have not been measured.</p>
<p>The <strong>Container stats</strong> card shows the selected preset's scope
and sampling interval. With JavaScript, the interval is read-only under a
preset; Custom unlocks those current values, not an older custom configuration.
Changing the storage target marks the profile Custom without resetting the
current scope or interval. Without JavaScript, save a preset before selecting
Custom to edit its values. Preferences can still be saved while background
collection is unchecked.</p>
<p>On <strong>Overview</strong>, switch between CPU, memory and network charts.
These totals cover collected containers, not the Docker host's total usage.
For individual history, open <strong>Containers &gt; Observability</strong>,
apply filters in the first sidebar, and select a name in the second sidebar.
The first filtered container is selected by default. The vertical timeline
and stacked charts share the selected time; selecting an observed event also
moves that selection. Missing observations remain gaps, not zero values.</p>
<p>The blue time label follows your selection and shows local time. Hover
<strong>Selected time</strong> for the date, time zone and any fractional seconds;
the slider's accessible value includes those details for screen readers.
Event rows display UTC explicitly. Use the arrow at the bottom of the timeline
to return all charts to the end of the displayed hour; this does not start
live following or refresh the page. Keyboard users can use the arrow keys,
Home and End on the slider.</p>
<p>The records area shows observed container events. The
<strong>Container logs</strong> tab currently reports that application logs are
not collected; enabling metrics does not enable log capture.</p>

### Storage pressure and empty charts

<p>Check <strong>Collection status</strong> in Settings first. The
storage-pressure warning is cumulative: its sample count and timestamp bounds
describe earlier removals, not necessarily the current collection cycle or a
continuous gap. Collection retains up to one hour, but storage pressure can
remove the oldest samples earlier. The collector pauses writes when maintenance
cannot verify usable space and retries maintenance every five seconds.</p>
<p>A healthy collection status does not guarantee retained samples for the
selected container. Check filters, collection scope and whether the container
was recently recreated. If storage warnings persist, review the configured
target and available disk space; do not delete the state volume to reset a
chart. If the layout itself is broken after an upgrade, check the
<a href="../deployment/">reverse proxy caching guidance</a> before changing
collection settings.</p>

## Configuration

<p>The native binary reads exported environment variables, not dotenv files.</p>
<p>In the Compose example, <code>APP_ADDR</code> and <code>APP_DATA_DIR</code>
are fixed to the container listener and volume path. Change the Compose file
itself if those container paths need to change; <code>APP_PORT</code> controls
only the host port.</p>
<table>
  <thead><tr><th>Variable</th><th>Default</th><th>Purpose</th></tr></thead>
  <tbody>
    <tr><td>APP_ADDR</td><td>127.0.0.1:8080</td><td>Listen host and numeric port (1–65535)</td></tr>
    <tr><td>APP_LOG_LEVEL</td><td>info</td><td>debug, info, warn, or error</td></tr>
    <tr><td>APP_DATA_DIR</td><td>./data</td><td>Persistent SQLite state directory</td></tr>
    <tr><td>APP_DOCKER_SOCKET</td><td>unset</td><td>Optional absolute Unix socket path for read-only inventory and opt-in metrics; socket access is effectively host-administrative</td></tr>
    <tr><td>APP_PUBLIC_URL</td><td>http://127.0.0.1:8080</td><td>Canonical app origin; remote origin requires HTTPS</td></tr>
    <tr><td>APP_TRUSTED_PROXIES</td><td>empty</td><td>Comma-separated immediate proxy CIDRs</td></tr>
    <tr><td>APP_BOOTSTRAP_USERNAME</td><td>unset</td><td>Required with password only for an uninitialized database</td></tr>
    <tr><td>APP_BOOTSTRAP_PASSWORD</td><td>unset</td><td>Temporary bootstrap password; remove from deployment env after setup</td></tr>
    <tr><td>APP_PORT</td><td>8080</td><td>Compose host port only</td></tr>
  </tbody>
</table>
<p>Bootstrap credentials are used only once; later environment changes do not
reset saved credentials. Docker administrators can inspect container environment,
so protect the temporary bootstrap values and remove them after first-run setup.
Compose persists app state in a dedicated <code>/data</code> volume.</p>
<p>The database is local persistent state, not a supported network-shared or
multi-instance database. Back it up consistently before upgrades or moving hosts.
If the volume is lost, the app cannot distinguish a genuinely new installation
from lost credentials; recovery/reset procedures are not implemented yet.</p>
