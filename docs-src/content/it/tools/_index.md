---
title: "Strumenti"
description: "Strumenti per PgArachne - Documentazione."
menu:
  main:
    name: "Strumenti"
    weight: 70
---

<section id="tools">
<h2>Strumenti</h2>
<p>PgArachne è corredato da un insieme di strumenti per browser che semplificano lo sviluppo, il collaudo e l'esplorazione dell'API. Ogni strumento è un singolo file HTML — nessun build, nessuna dipendenza.</p>

<div class="tools-grid">
<div class="card">
<h3>PgArachne Explorer</h3>
<p>Un'interfaccia web completa per esplorare l'API, testare le funzioni e visualizzare la documentazione generata automaticamente. Supporta le credenziali dirette (HTTP Basic Auth) e l'autenticazione Bearer token.</p>
<p><a href="api-explorer/" class="btn">Scopri di più sull'Explorer</a></p>
</div>

<div class="card">
<h3>SSE Tester</h3>
<p>Iscriviti a uno o più canali PostgreSQL NOTIFY tramite una connessione Server-Sent Events in tempo reale. Supporta tutti e tre i metodi di autenticazione e visualizza gli eventi JSON con evidenziazione della sintassi.</p>
<p><a href="sse-tester/" class="btn">Scopri di più sull'SSE Tester</a></p>
</div>

<div class="card">
<h3>JWT Getter</h3>
<p>Scambia nome utente e password PostgreSQL con un JWT di breve durata tramite il metodo <code>get_jwt</code>. Mostra il payload decodificato e la scadenza.</p>
<p><a href="get-jwt/" class="btn">Scopri di più sul JWT Getter</a></p>
</div>

<div class="card">
<h3>PgArachne Toolbar (macOS)</h3>
<p>Un'applicazione nativa per macOS nella barra dei menu. Gestisci più istanze di PgArachne, visualizza i log in tempo reale e monitora le metriche con un clic.</p>
<p><a href="macos-toolbar/" class="btn">Esplora le funzionalità del Toolbar</a></p>
</div>
</div>
</section>
