---
title: "PgArachne Explorer"
description: "PgArachne Explorer - PgArachne"
---

<section id="explorer">
<h2>PgArachne Explorer</h2>
<p>The <strong>Explorer</strong> is a powerful web GUI included in the <code>tools/pgarachne-explorer</code> directory. It is not
		just documentation; it is a fully functional <strong>demo application</strong> built using HTML/JS
		that communicates with the database exclusively via PgArachne. You can also use the hosted version at 
<a href="https://explorer.pgarachne.com" target="_blank">explorer.pgarachne.com</a>.</p>

<p><strong>What can it do?</strong></p>
<ul>
<li><strong>Inspect API:</strong> It reads the <code>capabilities</code> function to display all available
			endpoints and their parameters.</li>
<li><strong>Live Testing:</strong> You can execute functions directly from the browser.</li>
<li><strong>Auto-Documentation:</strong> It renders the SQL comments (including <code>--- PARAMS ---</code>
			metadata) into readable documentation.</li>
<li><strong>Modern UI:</strong> Features Dark/Light mode support and can be installed as a <strong>PWA</strong> on mobile devices.</li>
</ul>

<p><strong>URL Parameters:</strong></p>
<p>You can pre-fill the connection settings using the <code>?url=</code> parameter, for example: 
<code>?url=http://localhost:8080/db/my_database</code>. This will automatically split the URL into API Host, Prefix, and Database fields.</p>

<div class="tip">
<strong>How to enable it:</strong> Set the <code>STATIC_FILES_PATH</code> environment variable to point to the
<code>tools/pgarachne-explorer</code> folder on your disk. Then visit <code>http://localhost:8080</code>.
</div>
</section>
