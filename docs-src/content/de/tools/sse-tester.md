---
title: "SSE Tester"
description: "PgArachne SSE Tester - Dokumentation"
---

<section id="sse-tester">
<h2>SSE Tester</h2>
<p>Der <strong>SSE Tester</strong> ist ein einzelnes Browser-Werkzeug im Verzeichnis <code>tools/test-sse</code>. Er ermöglicht das Abonnieren beliebig vieler PostgreSQL <code>NOTIFY</code>-Kanäle über eine Live <a href="../../real-time-notifications/">Server-Sent Events</a>-Verbindung und die Beobachtung eingehender Ereignisse in Echtzeit.</p>

<h3>Funktionen</h3>
<ul>
<li><strong>Mehrere Kanäle</strong> — Kanäle werden als Tags hinzugefügt; alle werden als <code>?channels=</code>-Parameter übergeben.</li>
<li><strong>Alle Authentifizierungsmethoden</strong> — der Tab <em>Password</em> nutzt HTTP Basic Auth (direkte Anmeldedaten, kein JWT erforderlich); der Tab <em>Bearer Token</em> akzeptiert ein JWT oder API-Token.</li>
<li><strong>Live-Stream</strong> — verwendet die Fetch Streams API mit <code>AbortController</code> für sauberes Trennen.</li>
<li><strong>JSON-Syntaxhervorhebung</strong> — JSON-Payloads werden farbig formatiert dargestellt.</li>
<li><strong>Einstellungen speichern</strong> — URL, Präfix, Datenbank, Anmeldename und Kanalliste werden in <code>localStorage</code> gespeichert.</li>
</ul>

<h3>Authentifizierung</h3>
<ul>
<li><strong>Tab Password</strong> — sendet <code>Authorization: Basic &lt;base64(Benutzer:Passwort)&gt;</code>. Kein <code>GRANT &lt;Rolle&gt; TO pgarachne</code> erforderlich.</li>
<li><strong>Tab Bearer Token</strong> — sendet <code>Authorization: Bearer &lt;token&gt;</code>. JWT (über den <a href="../get-jwt/">JWT Getter</a>) oder langlebigen API-Token einfügen.</li>
</ul>

<div class="tip">
<strong>Aktivierung:</strong> Setzen Sie <code>STATIC_FILES_PATH</code> auf <code>tools/test-sse</code> und besuchen Sie <code>http://localhost:8080</code>. Alternativ öffnen Sie <code>index.html</code> direkt im Browser — alles funktioniert ohne lokalen Server.
</div>
</section>
