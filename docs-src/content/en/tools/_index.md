---
title: "Tools"
description: "Tools for PgArachne - Documentation."
menu:
  main:
    name: "Tools"
    weight: 80
---

<section id="tools">
<h2>Tools</h2>
<p>PgArachne is supported by a set of browser-based tools that simplify development, testing, and API exploration. All tools are single HTML files — no build step, no dependencies.</p>

<div class="tools-grid">
<div class="card">
<h3>PgArachne Explorer</h3>
<p>A full-featured web UI to browse your API, test functions, and view auto-generated documentation. Supports both direct credentials (HTTP Basic Auth) and Bearer token authentication.</p>
<p><a href="api-explorer/" class="btn stretched-link">Learn more about Explorer</a></p>
</div>

<div class="card">
<h3>SSE Tester</h3>
<p>Subscribe to one or more PostgreSQL NOTIFY channels over a live Server-Sent Events connection. Supports all three authentication methods and displays JSON events with syntax highlighting.</p>
<p><a href="sse-tester/" class="btn stretched-link">Learn more about SSE Tester</a></p>
</div>

<div class="card">
<h3>JWT Getter</h3>
<p>Exchange a PostgreSQL username and password for a short-lived JWT via the <code>/token</code> endpoint (HTTP Basic credentials). Shows the decoded payload and expiry time — useful for debugging token-based flows.</p>
<p><a href="get-jwt/" class="btn stretched-link">Learn more about JWT Getter</a></p>
</div>

<div class="card">
<h3>JWT Signer</h3>
<p>Generate JWTs locally in the browser by entering <code>JWT_SECRET</code> by hand — no database or server needed. Set the role, database, lifetime and extra claims, even an already expired token for testing. Includes full security warnings.</p>
<p><a href="jwt-signer/" class="btn stretched-link">Learn more about JWT Signer</a></p>
</div>

<div class="card">
<h3>PgArachne Toolbar (macOS)</h3>
<p>A native macOS application that lives in your menu bar. Manage multiple PgArachne instances, view live logs, and monitor metrics with a single click.</p>
<p><a href="macos-toolbar/" class="btn stretched-link">Explore Toolbar Features</a></p>
</div>
</div>
</section>
