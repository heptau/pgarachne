---
title: "JWT Getter"
description: "PgArachne JWT Getter - Documentation"
---

<section id="get-jwt">
<h2>JWT Getter</h2>
<p>The <strong>JWT Getter</strong> is a minimal single-file browser tool located in <code>tools/get-jwt</code>. It exchanges a PostgreSQL username and password for a short-lived JWT by calling the <code>get_jwt</code> JSON-RPC method — and shows you the decoded payload and expiry time.</p>

<h3>When to use it</h3>
<p>Use the JWT Getter when you need a token for one of these purposes:</p>
<ul>
<li>Pasting a token into the <a href="../api-explorer/">Explorer</a> or <a href="../sse-tester/">SSE Tester</a> (Bearer Token tab).</li>
<li>Testing JWT expiry or <code>db_role</code> / <code>db_name</code> claims in your application.</li>
<li>Quickly verifying that the <code>pgarachne</code> system user can switch to a given role (<code>GRANT &lt;role&gt; TO pgarachne</code> must be in place).</li>
</ul>

<div class="tip">
<strong>Note:</strong> <code>get_jwt</code> requires <code>GRANT &lt;role&gt; TO pgarachne</code> in the database because PgArachne verifies the password internally via <code>SET LOCAL ROLE</code>. If you want to skip that grant, use direct credentials (HTTP Basic Auth) in the <a href="../api-explorer/">Explorer</a> or <a href="../sse-tester/">SSE Tester</a> instead.
</div>

<h3>Features</h3>
<ul>
<li>Enter API URL, prefix, database, username and password — press <kbd>Enter</kbd> or click <strong>Get JWT</strong>.</li>
<li>The raw token is displayed and can be copied with one click.</li>
<li>The JWT payload is Base64-decoded and displayed with JSON syntax highlighting.</li>
<li>An expiry badge shows the remaining lifetime (or marks the token as expired).</li>
<li>Connection settings and login are saved to <code>localStorage</code>.</li>
</ul>

<h3>How to enable it</h3>
<p>Set <code>STATIC_FILES_PATH</code> to <code>tools/get-jwt</code> and visit <code>http://localhost:8080</code>, or open <code>index.html</code> directly in a browser.</p>
</section>
