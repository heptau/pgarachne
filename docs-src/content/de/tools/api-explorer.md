---
title: "PgArachne Explorer"
description: "PgArachne Explorer - PgArachne documentation."
---

<section id="explorer">
<h2>PgArachne Explorer</h2>
<p>Der <strong>Explorer</strong> ist eine leistungsstarke Web-GUI, die im Verzeichnis <code>tools/pgarachne-explorer</code> enthalten ist. Es ist nicht
		nur Dokumentation; es ist eine voll funktionsfähige <strong>Demo-Anwendung</strong>, die mit HTML/JS erstellt wurde
		und ausschließlich über PgArachne mit der Datenbank kommuniziert. Sie können die gehostete Version auch unter 
<a href="https://explorer.pgarachne.com" target="_blank">explorer.pgarachne.com</a> ausprobieren.</p>

<p><strong>Was kann er tun?</strong></p>
<ul>
<li><strong>API inspizieren:</strong> Er liest die Funktion <code>capabilities</code>, um alle verfügbaren
			Endpunkte und deren Parameter anzuzeigen.</li>
<li><strong>Live-Tests:</strong> Sie können Funktionen direkt im Browser ausführen.</li>
<li><strong>Automatische Dokumentation:</strong> Er rendert die SQL-Kommentare (einschließlich <code>--- PARAMS ---</code>
			Metadaten) in eine lesbare Dokumentation.</li>
<li><strong>Modernes UI:</strong> Bietet Unterstützung für den Dunkel-/Hell-Modus und kann als <strong>PWA</strong> auf mobilen Geräten installiert werden.</li>
</ul>

<p><strong>URL-Parameter:</strong></p>
<p>Die Verbindungseinstellungen können Sie mit dem Parameter <code>?url=</code> vorab ausfüllen, zum Beispiel: 
<code>?url=http://localhost:8080/db/meine_datenbank</code>. Der Explorer teilt die URL automatisch in API-Host, Präfix und Datenbankname auf.</p>

<div class="tip">
<strong>So aktivieren Sie ihn:</strong> Setzen Sie die Umgebungsvariable <code>STATIC_FILES_PATH</code> so, dass sie auf den
		Ordner <code>tools/pgarachne-explorer</code> auf Ihrer Festplatte zeigt. Besuchen Sie dann <code>http://localhost:8080</code>.
</div>
</section>
