---
title: "PgArachne Explorer"
description: "PgArachne Explorer - PgArachne documentation."
---

<section id="explorer">
<h2>PgArachne Explorer</h2>
<p>L’<strong>Explorer</strong> è una potente interfaccia web inclusa nella directory <code>tools/pgarachne-explorer</code>. Non è solo
		documentazione; è una <strong>applicazione demo</strong> completamente funzionale costruita usando HTML/JS
		che comunica con il database esclusivamente tramite PgArachne. Puoi anche usare la versione ospitata su 
<a href="https://explorer.pgarachne.com" target="_blank">explorer.pgarachne.com</a>.</p>

<p><strong>Cosa può fare?</strong></p>
<ul>
<li><strong>Ispezionare API:</strong> Legge la funzione <code>capabilities</code> per mostrare tutti gli endpoint
			disponibili e i loro parametri.</li>
<li><strong>Test dal Vivo:</strong> Puoi eseguire le funzioni direttamente dal browser.</li>
<li><strong>Auto-Documentazione:</strong> Renderizza i commenti SQL (inclusi i metadati <code>--- PARAMS ---</code>)
			in una documentazione leggibile.</li>
<li><strong>Interfaccia moderna:</strong> Offre il supporto per la modalità Scura/Chiara e può essere installata come <strong>PWA</strong> sui dispositivi mobili.</li>
</ul>

<p><strong>Parametri URL:</strong></p>
<p>Puoi pre-compilare le impostazioni di connessione usando il parametro <code>?url=</code>, per esempio: 
<code>?url=http://localhost:8080/db/mio_database</code>. L'Explorer dividerà automaticamente l'URL nei campi Host API, Prefisso e Database.</p>

<div class="tip">
<strong>Come abilitarlo:</strong> Imposta la variabile d’ambiente <code>STATIC_FILES_PATH</code> in modo che punti
		alla cartella <code>tools/pgarachne-explorer</code> sul tuo disco. Poi visita <code>http://localhost:8080</code>.
</div>
</section>
