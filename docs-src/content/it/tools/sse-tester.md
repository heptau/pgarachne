---
title: "SSE Tester"
description: "PgArachne SSE Tester - Documentazione"
---

<section id="sse-tester">
<h2>SSE Tester</h2>
<p>L'<strong>SSE Tester</strong> è uno strumento per browser a file singolo situato in <code>tools/test-sse</code>. Permette di iscriversi a un numero qualsiasi di canali PostgreSQL <code>NOTIFY</code> tramite una connessione <a href="../../real-time-notifications/">Server-Sent Events</a> in tempo reale e di osservare gli eventi in arrivo.</p>

<h3>Funzionalità</h3>
<ul>
<li><strong>Più canali</strong> — i canali si aggiungono come tag e vengono passati come parametro <code>?channels=</code>.</li>
<li><strong>Tutti i metodi di autenticazione</strong> — la scheda <em>Password</em> usa HTTP Basic Auth (senza JWT); la scheda <em>Bearer Token</em> accetta un JWT o un token API.</li>
<li><strong>Stream in tempo reale</strong> — utilizza la Fetch Streams API con <code>AbortController</code> per una disconnessione pulita.</li>
<li><strong>Evidenziazione sintassi JSON</strong> — i payload JSON vengono visualizzati con colori.</li>
<li><strong>Impostazioni persistenti</strong> — URL, prefisso, database, nome utente e lista canali vengono salvati in <code>localStorage</code>.</li>
</ul>

<h3>Autenticazione</h3>
<ul>
<li><strong>Scheda Password</strong> — invia <code>Authorization: Basic &lt;base64(utente:password)&gt;</code>. Non richiede <code>GRANT &lt;ruolo&gt; TO pgarachne</code>.</li>
<li><strong>Scheda Bearer Token</strong> — invia <code>Authorization: Bearer &lt;token&gt;</code>. Incolla un JWT (ottenuto tramite il <a href="../get-jwt/">JWT Getter</a>) o un token API longevo.</li>
</ul>

<div class="tip">
<strong>Come abilitarlo:</strong> Imposta <code>STATIC_FILES_PATH</code> su <code>tools/test-sse</code> e visita <code>http://localhost:8080</code>. In alternativa, apri <code>index.html</code> direttamente nel browser.
</div>
</section>
