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
boundary. Inventory snapshots list running and stopped containers. Background
stats, state/event history and container logs are separate opt-in sources;
lifecycle management operations are not connected.</p>
<p>On the Projects and Containers pages, use <strong>Customize columns</strong>
to choose visible columns and restore the defaults. Preferences are saved
separately for each view in your browser; the identifying column always stays
visible. Without JavaScript, all columns remain visible.</p>
<p>Both inventories use the same table. Select a stack or container name to
open its details; stack rows no longer expand inline. The right-hand drawer
overlays the desktop inventory without resizing it, so you can select another
row directly. On narrow screens it fills the screen. Use the close control or
Escape to return to the list.</p>
<p><strong>Overview</strong> shows current discovery/runtime information and
associations. <strong>Configuration</strong> shows read-only identity/location
information and, for containers, image and reported ports. These are snapshots;
use <strong>Refresh details</strong> for a new read. Unavailable Docker status
does not imply an empty stack. A removed/recreated container may no longer be
available when selected. Environment values and arbitrary Docker labels are
not displayed. The existing permission-only <strong>Activate</strong> control
is in stack details; lifecycle actions, editing and tags/groups are not yet
available. Without JavaScript, name links open a page with both detail sections
and a normal close link.</p>

## Metrics and container observability

<p>After configuring Docker access, open
<strong>Settings &gt; Containers</strong> to find <strong>Observability settings</strong>.
All sources start <strong>Off</strong> on a new installation.
The overall Low, Medium and High choices configure all three sources, including
logs, when you <strong>Save observability preferences</strong>.
Review the source cards before saving. Individual changes mark the overall
choice Custom without changing the other sources.</p>
<ul>
<li><strong>Container stats:</strong> Low samples activated stacks every
15 seconds; Medium every 5 seconds; High samples all running containers every
second. Custom allows a different scope and positive interval.</li>
<li><strong>Container state &amp; events:</strong> choose Off, Activated stacks
or All containers. Enabled scopes include stopped-container context and record
changes rather than an unchanged state every second. Overall Low/Medium select
Activated stacks; High selects All containers.</li>
<li><strong>Container logs:</strong> Low reads activated-stack backlog every
60 seconds; Medium every 30 seconds; High follows all containers live.
Readable stopped-container backlog is included. Custom changes capture scope
while preserving the current preset's delivery; choose a preset first if no
log preferences exist. Custom transport/save-frequency tuning is not available.</li>
</ul>
<p>The <strong>Container stats</strong> card shows the selected preset's scope
and sampling interval. With JavaScript, the interval is read-only under a
preset; Custom unlocks those current values, not an older custom configuration.
Changing a storage target preserves the current capture values and does not
enable an Off source. Without JavaScript, save a preset or Custom selection
before editing its values.</p>
<p><strong>Off</strong> stops a source and hides its retained history; it is not
a clear-history action. Preferences remain saved. Overall Off followed directly
by Custom restores the preceding source selections and values. Re-enabling
history shows only data still eligible for retention, not an unlimited archive.
Upgrades preserve existing stats/state enablement and the metrics target;
logs remain Off until explicitly enabled and saved.</p>
<p>On <strong>Overview</strong>, switch between CPU, memory and network charts.
These totals cover collected containers, not the Docker host's total usage.
For individual history, open <strong>Containers &gt; Observability</strong>,
apply filters in the first sidebar, and select a name in the second sidebar.
The first filtered container is selected by default. The vertical timeline
and stacked charts share the selected time; selecting an observed event also
moves that selection. Missing observations remain gaps, not zero values.</p>
<p>The blue selection line stays in place while time ticks, event markers and
state bands move beneath it. Scroll over the timeline or drag its empty rail
to browse earlier observations; click or tap an empty point to select its time.
The minus, slider and plus controls change zoom without extending the retained
hour. Nearby records share a count badge to avoid overlapping labels; zoom in
or select the badge to separate them. Individual event markers and event rows
also select a time.</p>
<p>The blue badge shows local time, or <strong>Latest</strong> at the end of the
displayed snapshot. Hover <strong>Selected time</strong> for the date, time zone
and any fractional seconds; the time slider's accessible value includes those
details for screen readers. Event rows display UTC explicitly. Hover or focus
a marker or state band for its description and recorded time range; Escape
dismisses the detail. Use the round arrow at the bottom to return all charts to
the end of the displayed hour. This does not start live following or refresh
the page. Keyboard users can use arrow keys, Home, End, Page Up and Page Down
on the selected-time slider.</p>
<p>The records area contains state/event history and
<strong>Container logs</strong>. Log records show Docker time, stream and
message text. Stderr is a stream, not an inferred severity; application JSON
is not parsed. TTY output is merged and line-oriented, without dependable
stdout/stderr separation. Messages over 64 KiB are explicitly truncated.
The page shows at most 200 newest records within the 1 MiB query budget; an omission
notice indicates that the displayed result is not the entire matching history.</p>
<p>State health and lifecycle are separate. Striped red marks observed unhealthy
intervals; grey marks unavailable state observation and takes precedence over
red. A healthy observation after reconnect is labelled as observed on reconnect,
not assigned a fabricated recovery time during an unavailable interval.
One pre-window state baseline may be kept for context.</p>

### Storage pressure and empty charts

<p>Check <strong>Collection status</strong> in Settings first. The
storage-pressure warning is cumulative: its sample count and timestamp bounds
describe earlier removals, not necessarily the current collection cycle or a
continuous gap. Collection retains up to one hour, but storage pressure can
remove the oldest samples or logs earlier within that source's own database.
New-install targets are <strong>128 MB</strong> for metrics/state history and
<strong>256 MB</strong> for logs; existing targets remain unchanged. Units are
decimal B/KB/MB/GB, not reserved disk space or hard disk quotas. Deleted rows can
leave reusable pages without shrinking the file. The collector pauses writes when maintenance
cannot verify usable space and retries maintenance every five seconds.</p>
<p>A healthy collection status does not guarantee retained samples for the
selected container. Check filters, collection scope and whether the container
was recently recreated. If storage warnings persist, review the configured
target and available disk space; do not delete the state volume to reset a
chart. If the layout itself is broken after an upgrade, check the
<a href="../deployment/">reverse proxy caching guidance</a> before changing
collection settings.</p>
<p>Observability allows five seconds for its stored-history reads after obtaining
the Docker snapshot. If retained container identities cannot be read, it shows
an explicit warning and keeps available snapshot names; charts, logs or state
history may also be unavailable. Refresh after checking collection status.
Previously recorded duplicate state rows are not deleted by an upgrade; normal
retention removes old in-window rows.</p>

## Troubleshooting logs

<p>Set <code>APP_LOG_LEVEL=debug</code> and restart the manager for additional
diagnostics. Debug output includes startup/database timing, loaded collection
profiles and scopes, source-session starts/stops, metrics-cycle counts/timing,
live-log collection status, page/history-read timing and matched HTTP routes.
Normal Info logging remains quieter. Health checks and static-file requests
are excluded from request diagnostics.</p>
<p>Application diagnostics, standard-library server messages and Compose SDK
warnings use the same newline-delimited JSON format. Compose messages have
<code>source: "compose"</code>, preserve warning/error severity and may identify
the affected Compose file. This reformats the warning; it does not fix an obsolete
Compose attribute. Debug diagnostics do not copy request query strings, bodies,
cookies, authorization headers or captured container message text. SDK
debug/trace configuration dumps remain disabled. Per-message log-ingestion
diagnostics are deliberately omitted so collecting the manager's own container
does not create a logging feedback loop.</p>
<p>When sharing troubleshooting output, include the relevant timestamps, source
settings and errors, and review it for sensitive application details first.
These manager diagnostics are separate from the collected container output
shown in Observability.</p>

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
    <tr><td>APP_DOCKER_SOCKET</td><td>unset</td><td>Optional absolute Unix socket path for inventory and opt-in stats/state/logs; socket access is effectively host-administrative</td></tr>
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
