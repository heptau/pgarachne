---
title: "JWT Signer"
description: "PgArachne JWT Signer"
---

<section id="jwt-signer">
<h2>JWT Signer</h2>
<p>The <strong>JWT Signer</strong> is a single-file browser tool located in <code>tools/jwt-signer</code>. Unlike the <a href="../get-jwt/">JWT Getter</a>, it never contacts PgArachne or the database: you enter the <code>JWT_SECRET</code> by hand and the token is signed (HS256) directly in your browser.</p>
<p>Online version: <a href="https://jwt-signer.pgarachne.com" target="_blank" rel="noopener">jwt-signer.pgarachne.com</a></p>

<figure class="tool-screenshot">
<img src="/assets/pgarachne-tool-jwt-signer.webp" alt="JWT Signer" loading="lazy">
</figure>

<div class="warning">
<strong>Handle your JWT secret with care</strong>
<ul>
<li><code>JWT_SECRET</code> is the master key of your API: anyone who knows it can sign a token for <em>any</em> database role, including superusers.</li>
<li><strong>Never paste a production secret into a web page you do not fully control.</strong> Use the hosted version with development / test secrets only. For production, save <code>index.html</code> and open it locally (it works offline), or sign tokens on the server.</li>
<li>Everything runs in your browser. The page forbids all network access (Content Security Policy <code>connect-src 'none'</code>), loads no external resources and <strong>never stores the secret</strong> — not in <code>localStorage</code>, cookies or the URL. Only the role, database, lifetime, issuer and audience are remembered.</li>
<li>A token signed here is accepted by PgArachne exactly like one issued by <code>/token</code>. Keep lifetimes short and never share tokens signed with a real secret.</li>
</ul>
</div>

<h3>When to use it</h3>
<ul>
<li>Testing and development when you do not want (or cannot) log in with a database password.</li>
<li>Verifying how your client handles expiry: a negative lifetime produces an already expired token.</li>
<li>Trying out <code>iss</code> / <code>aud</code> and extra claims against a server configured to require them.</li>
<li>Generating a random 256-bit secret for a new <code>JWT_SECRET</code> with one click.</li>
</ul>

<div class="tip">
<strong>How to enable it:</strong> Set <code>STATIC_FILES_PATH</code> to <code>tools/jwt-signer</code> and visit <code>http://localhost:8080</code>, or open <code>index.html</code> directly in a browser — it works without any server.
</div>
</section>
