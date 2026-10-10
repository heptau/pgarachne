---
title: "JWT Getter"
description: "PgArachne JWT Getter - Dokumentation"
---

<section id="get-jwt">
<h2>JWT Getter</h2>
<p>Der <strong>JWT Getter</strong> ist ein minimalistisches Browser-Werkzeug im Verzeichnis <code>tools/jwt-getter</code>. Er tauscht PostgreSQL-Benutzername und Passwort gegen ein kurzlebiges JWT durch Aufruf von <code>POST /{prefix}/{database}/token</code> mit HTTP-Basic-Zugangsdaten — und zeigt den dekodierten Payload sowie die Ablaufzeit an.</p>
<p>Online-Version: <a href="https://jwt-getter.pgarachne.com" target="_blank" rel="noopener">jwt-getter.pgarachne.com</a></p>

<figure class="tool-screenshot">
<img src="/assets/pgarachne-tool-jwt-getter.webp" alt="JWT Getter" loading="lazy">
</figure>


<h3>Wann verwenden</h3>
<p>Verwenden Sie den JWT Getter, wenn Sie ein Token für einen dieser Zwecke benötigen:</p>
<ul>
<li>Ein Token in den Tab Bearer Token im <a href="../api-explorer/">Explorer</a> oder <a href="../sse-tester/">SSE Tester</a> einfügen.</li>
<li>JWT-Ablauf oder <code>db_role</code>/<code>db_name</code>-Claims in der eigenen Anwendung testen.</li>
<li>Überprüfen, ob der Systembenutzer <code>pgarachne</code> auf eine bestimmte Rolle wechseln kann (<code>GRANT &lt;Rolle&gt; TO pgarachne</code> muss vorhanden sein).</li>
</ul>

<div class="tip">
<strong>Hinweis:</strong> Ein JWT funktioniert nur, wenn PgArachne zur Rolle des Benutzers wechseln darf. Das erfordert <code>GRANT &lt;Rolle&gt; TO pgarachne</code> in der Datenbank (PgArachne verwendet <code>SET LOCAL ROLE</code> für Token-authentifizierte Anfragen). Um dieses Grant zu vermeiden, verwenden Sie stattdessen direkte Anmeldedaten (HTTP Basic Auth) im <a href="../api-explorer/">Explorer</a> oder <a href="../sse-tester/">SSE Tester</a>.
</div>

<h3>Funktionen</h3>
<ul>
<li>URL, Präfix, Datenbank, Benutzername und Passwort eingeben — <kbd>Enter</kbd> drücken oder <strong>Get JWT</strong> klicken.</li>
<li>Das generierte Token wird angezeigt und kann per Klick kopiert werden.</li>
<li>Der JWT-Payload wird Base64-dekodiert und mit JSON-Syntaxhervorhebung dargestellt.</li>
<li>Ein Badge zeigt die verbleibende Laufzeit an (oder markiert das Token als abgelaufen).</li>
<li>Verbindungseinstellungen und Anmeldename werden in <code>localStorage</code> gespeichert.</li>
</ul>

<h3>Aktivierung</h3>
<p>Setzen Sie <code>STATIC_FILES_PATH</code> auf <code>tools/jwt-getter</code> und besuchen Sie <code>http://localhost:8080</code>, oder öffnen Sie <code>index.html</code> direkt im Browser.</p>
</section>
