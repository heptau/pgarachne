---
title: "JWT Signer"
description: "PgArachne JWT Signer"
---

<section id="jwt-signer">
<h2>JWT Signer</h2>
<p><strong>JWT Signer</strong> è uno strumento browser a file singolo in <code>tools/jwt-signer</code>. A differenza di <a href="../get-jwt/">JWT Getter</a>, non contatta mai PgArachne né il database: inserisci <code>JWT_SECRET</code> a mano e il token viene firmato (HS256) direttamente nel browser.</p>
<p>Versione online: <a href="https://jwt-signer.pgarachne.com" target="_blank" rel="noopener">jwt-signer.pgarachne.com</a></p>

<figure class="tool-screenshot">
<img src="/assets/pgarachne-tool-jwt-signer.webp" alt="JWT Signer" loading="lazy">
</figure>

<div class="warning">
<strong>Maneggia con cura il tuo segreto JWT</strong>
<ul>
<li><code>JWT_SECRET</code> è la chiave principale della tua API: chiunque la conosca può firmare un token per <em>qualsiasi</em> ruolo del database, inclusi i superutenti.</li>
<li><strong>Non incollare mai un segreto di produzione in una pagina web che non controlli completamente.</strong> Usa la versione ospitata solo con segreti di sviluppo/test. In produzione salva <code>index.html</code> e aprilo in locale (funziona offline) oppure firma i token sul server.</li>
<li>Tutto viene eseguito nel browser. La pagina vieta qualsiasi accesso alla rete (Content Security Policy <code>connect-src 'none'</code>), non carica risorse esterne e <strong>non memorizza mai il segreto</strong> — né in <code>localStorage</code>, né nei cookie, né nell'URL. Vengono ricordati solo ruolo, database, durata, issuer e audience.</li>
<li>PgArachne accetta un token firmato qui esattamente come uno emesso da <code>/token</code>. Usa durate brevi e non condividere mai token firmati con un segreto reale.</li>
</ul>
</div>

<h3>Quando usarlo</h3>
<ul>
<li>Sviluppo e test quando non vuoi (o non puoi) accedere con la password del database.</li>
<li>Verificare come il client gestisce la scadenza: una durata negativa produce un token già scaduto.</li>
<li>Provare <code>iss</code> / <code>aud</code> e claim aggiuntivi su un server che li richiede.</li>
<li>Generare con un clic un segreto casuale a 256 bit per un nuovo <code>JWT_SECRET</code>.</li>
</ul>

<div class="tip">
<strong>Come attivarlo:</strong> Imposta <code>STATIC_FILES_PATH</code> su <code>tools/jwt-signer</code> e visita <code>http://localhost:8080</code>, oppure apri <code>index.html</code> direttamente nel browser: funziona senza server.
</div>
</section>
