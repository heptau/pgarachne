---
title: "SSE Tester"
description: "PgArachne SSE Tester - Documentation"
---

<section id="sse-tester">
<h2>SSE Tester</h2>
<p>The <strong>SSE Tester</strong> is a single-file browser tool located in <code>tools/test-sse</code>. It lets you subscribe to any number of PostgreSQL <code>NOTIFY</code> channels over a live <a href="../../real-time-notifications/">Server-Sent Events</a> connection and watch the incoming events in real time.</p>

<h3>Features</h3>
<ul>
<li><strong>Multi-channel subscriptions</strong> — add or remove channels as pill tags before connecting. All channels are passed as a comma-separated <code>?channels=</code> query parameter.</li>
<li><strong>All authentication methods</strong> — the <em>Password</em> tab uses HTTP Basic Auth (direct credentials, no JWT needed); the <em>Bearer Token</em> tab accepts a JWT or API token.</li>
<li><strong>Live stream</strong> — uses the Fetch Streams API with an <code>AbortController</code> for clean disconnect. SSE lines are parsed manually so <code>event:</code>, <code>data:</code>, and heartbeat (<code>:</code>) fields are all handled correctly.</li>
<li><strong>JSON syntax highlighting</strong> — data payloads that are valid JSON are pretty-printed with colour coding.</li>
<li><strong>Persistent settings</strong> — API URL, prefix, database, login, and channel list are saved to <code>localStorage</code>.</li>
</ul>

<h3>Authentication</h3>
<p>The tool verifies credentials before revealing the subscription panel. Clicking <strong>Verify &amp; Continue</strong> calls <code>capabilities</code> with the chosen header — if PgArachne returns a successful response, the subscription panel unlocks.</p>

<ul>
<li><strong>Password tab</strong> — sends <code>Authorization: Basic &lt;base64(user:pass)&gt;</code>. The database user must have <code>GRANT EXECUTE</code> on the target functions and access to the SSE endpoint; no <code>GRANT &lt;role&gt; TO pgarachne</code> is needed.</li>
<li><strong>Bearer Token tab</strong> — sends <code>Authorization: Bearer &lt;token&gt;</code>. Paste a JWT (obtained via the <a href="../get-jwt/">JWT Getter</a>) or a long-lived API token.</li>
</ul>

<div class="tip">
<strong>How to enable it:</strong> Set <code>STATIC_FILES_PATH</code> to <code>tools/test-sse</code> and visit <code>http://localhost:8080</code>. Alternatively, open <code>index.html</code> directly in a browser — all functionality works without a local server.
</div>
</section>
