---
title: "JWT Getter"
description: "PgArachne JWT Getter - Documentazione"
---

<section id="get-jwt">
<h2>JWT Getter</h2>
<p>Il <strong>JWT Getter</strong> è uno strumento minimalista per browser situato in <code>tools/get-jwt</code>. Scambia nome utente e password PostgreSQL con un JWT di breve durata chiamando il metodo JSON-RPC <code>get_jwt</code> — e mostra il payload decodificato e la scadenza.</p>

<h3>Quando usarlo</h3>
<ul>
<li>Per incollare un token nella scheda Bearer Token dell'<a href="../api-explorer/">Explorer</a> o dell'<a href="../sse-tester/">SSE Tester</a>.</li>
<li>Per testare la scadenza di un JWT o i claim <code>db_role</code>/<code>db_name</code> nella propria applicazione.</li>
<li>Per verificare che l'utente di sistema <code>pgarachne</code> possa cambiare a un determinato ruolo (<code>GRANT &lt;ruolo&gt; TO pgarachne</code> deve essere presente).</li>
</ul>

<div class="tip">
<strong>Nota:</strong> <code>get_jwt</code> richiede <code>GRANT &lt;ruolo&gt; TO pgarachne</code>. Per evitare quel grant, usa le credenziali dirette (HTTP Basic Auth) nell'<a href="../api-explorer/">Explorer</a> o nell'<a href="../sse-tester/">SSE Tester</a>.
</div>

<h3>Funzionalità</h3>
<ul>
<li>Inserisci URL, prefisso, database, nome utente e password — premi <kbd>Invio</kbd> o clicca su <strong>Get JWT</strong>.</li>
<li>Il token generato viene visualizzato e può essere copiato con un clic.</li>
<li>Il payload del JWT viene decodificato in Base64 e visualizzato con evidenziazione della sintassi JSON.</li>
<li>Un badge mostra la durata rimanente (o segna il token come scaduto).</li>
<li>Le impostazioni di connessione e il nome utente vengono salvati in <code>localStorage</code>.</li>
</ul>

<h3>Come abilitarlo</h3>
<p>Imposta <code>STATIC_FILES_PATH</code> su <code>tools/get-jwt</code> e visita <code>http://localhost:8080</code>, oppure apri <code>index.html</code> direttamente nel browser.</p>
</section>
