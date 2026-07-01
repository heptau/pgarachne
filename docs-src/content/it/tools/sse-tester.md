---
title: "SSE Tester"
description: "PgArachne SSE Tester - Documentazione"
---

<section id="sse-tester">
<h2>SSE Tester</h2>
<p>L'<strong>SSE Tester</strong> è uno strumento per browser a file singolo situato in <code>tools/test-sse</code>. Permette di iscriversi a un numero qualsiasi di canali PostgreSQL <code>NOTIFY</code> tramite una connessione <a href="../../real-time-notifications/">Server-Sent Events</a> in tempo reale e di osservare gli eventi in arrivo.</p>

<h3>Funzionalità</h3>
<ul>
<li><strong>Più canali</strong> — aggiungi o rimuovi canali come tag prima di connetterti. Tutti i canali vengono passati come parametro <code>?channels=</code> separato da virgole.</li>
<li><strong>Tutti i metodi di autenticazione</strong> — la scheda <em>Password</em> usa HTTP Basic Auth (credenziali dirette, senza bisogno di JWT); la scheda <em>Bearer Token</em> accetta un JWT o un token API.</li>
<li><strong>Stream in tempo reale</strong> — utilizza la Fetch Streams API con <code>AbortController</code> per una disconnessione pulita. Le righe SSE vengono analizzate manualmente, in modo che i campi <code>event:</code>, <code>data:</code> e heartbeat (<code>:</code>) siano gestiti tutti correttamente.</li>
<li><strong>Evidenziazione sintassi JSON</strong> — i payload JSON vengono visualizzati con colori.</li>
<li><strong>Impostazioni persistenti</strong> — URL, prefisso, database, nome utente e lista canali vengono salvati in <code>localStorage</code>.</li>
</ul>

<h3>Autenticazione</h3>
<p>Lo strumento verifica le credenziali prima di mostrare il pannello di iscrizione. Facendo clic su <strong>Verify &amp; Continue</strong> si chiama <code>capabilities</code> con l'header scelto — se PgArachne restituisce una risposta positiva, il pannello di iscrizione si sblocca.</p>

<ul>
<li><strong>Scheda Password</strong> — invia <code>Authorization: Basic &lt;base64(utente:password)&gt;</code>. L'utente del database deve avere <code>GRANT EXECUTE</code> sulle funzioni target e accesso all'endpoint SSE; non è richiesto alcun <code>GRANT &lt;ruolo&gt; TO pgarachne</code>.</li>
<li><strong>Scheda Bearer Token</strong> — invia <code>Authorization: Bearer &lt;token&gt;</code>. Incolla un JWT (ottenuto tramite il <a href="../get-jwt/">JWT Getter</a>) o un token API longevo.</li>
</ul>

<div class="tip">
<strong>Come abilitarlo:</strong> Imposta <code>STATIC_FILES_PATH</code> su <code>tools/test-sse</code> e visita <code>http://localhost:8080</code>. In alternativa, apri <code>index.html</code> direttamente nel browser.
</div>
</section>
