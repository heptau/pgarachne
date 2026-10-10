---
title: "JWT Signer"
description: "PgArachne JWT Signer"
---

<section id="jwt-signer">
<h2>JWT Signer</h2>
<p>Der <strong>JWT Signer</strong> ist ein Ein-Datei-Browser-Tool in <code>tools/jwt-signer</code>. Anders als der <a href="../get-jwt/">JWT Getter</a> kontaktiert er weder PgArachne noch die Datenbank: Sie geben <code>JWT_SECRET</code> von Hand ein, und das Token wird direkt in Ihrem Browser signiert (HS256).</p>
<p>Online-Version: <a href="https://jwt-signer.pgarachne.com" target="_blank" rel="noopener">jwt-signer.pgarachne.com</a></p>

<figure class="tool-screenshot">
<img src="/assets/pgarachne-tool-jwt-signer.webp" alt="JWT Signer" loading="lazy">
</figure>

<div class="warning">
<strong>Gehen Sie sorgfältig mit Ihrem JWT-Secret um</strong>
<ul>
<li><code>JWT_SECRET</code> ist der Hauptschlüssel Ihrer API: Wer ihn kennt, kann ein Token für <em>jede</em> Datenbankrolle signieren, auch für Superuser.</li>
<li><strong>Fügen Sie niemals ein Produktions-Secret in eine Webseite ein, die Sie nicht vollständig kontrollieren.</strong> Nutzen Sie die gehostete Version nur mit Entwicklungs-/Test-Secrets. Für die Produktion speichern Sie <code>index.html</code> und öffnen es lokal (funktioniert offline) oder signieren Tokens auf dem Server.</li>
<li>Alles läuft in Ihrem Browser. Die Seite verbietet jeden Netzwerkzugriff (Content Security Policy <code>connect-src 'none'</code>), lädt keine externen Ressourcen und <strong>speichert das Secret nie</strong> — weder in <code>localStorage</code>, Cookies noch in der URL. Gemerkt werden nur Rolle, Datenbank, Laufzeit, Issuer und Audience.</li>
<li>Ein hier signiertes Token akzeptiert PgArachne genauso wie eines von <code>/token</code>. Wählen Sie kurze Laufzeiten und geben Sie mit einem echten Secret signierte Tokens niemals weiter.</li>
</ul>
</div>

<h3>Wann Sie es einsetzen</h3>
<ul>
<li>Entwicklung und Tests, wenn Sie sich nicht mit einem Datenbankpasswort anmelden wollen oder können.</li>
<li>Prüfen, wie Ihr Client mit Ablauf umgeht: eine negative Laufzeit erzeugt ein bereits abgelaufenes Token.</li>
<li><code>iss</code> / <code>aud</code> und zusätzliche Claims gegen einen Server testen, der sie verlangt.</li>
<li>Ein zufälliges 256-Bit-Secret für ein neues <code>JWT_SECRET</code> per Klick erzeugen.</li>
</ul>

<div class="tip">
<strong>So aktivieren Sie es:</strong> Setzen Sie <code>STATIC_FILES_PATH</code> auf <code>tools/jwt-signer</code> und rufen Sie <code>http://localhost:8080</code> auf, oder öffnen Sie <code>index.html</code> direkt im Browser — es funktioniert ohne Server.
</div>
</section>
