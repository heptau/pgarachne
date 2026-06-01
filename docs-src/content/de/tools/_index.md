---
title: "Werkzeuge"
description: "Werkzeuge für PgArachne - Dokumentation."
menu:
  main:
    name: "Werkzeuge"
    weight: 70
---

<section id="tools">
<h2>Werkzeuge</h2>
<p>PgArachne wird durch eine Reihe von Browser-Werkzeugen ergänzt, die Entwicklung, Tests und API-Erkundung vereinfachen. Jedes Werkzeug ist eine einzelne HTML-Datei — kein Build-Schritt, keine Abhängigkeiten.</p>

<div class="tools-grid">
<div class="card">
<h3>PgArachne Explorer</h3>
<p>Ein vollwertiges Web-Interface zur API-Erkundung, zum Testen von Funktionen und zur Anzeige automatisch generierter Dokumentation. Unterstützt direkte Anmeldedaten (HTTP Basic Auth) und Bearer-Token-Authentifizierung.</p>
<p><a href="api-explorer/" class="btn">Erfahren Sie mehr über den Explorer</a></p>
</div>

<div class="card">
<h3>SSE Tester</h3>
<p>Abonnieren Sie einen oder mehrere PostgreSQL NOTIFY-Kanäle über eine Live Server-Sent Events-Verbindung. Unterstützt alle drei Authentifizierungsmethoden und zeigt JSON-Ereignisse mit Syntaxhervorhebung.</p>
<p><a href="sse-tester/" class="btn">Mehr über den SSE Tester</a></p>
</div>

<div class="card">
<h3>JWT Getter</h3>
<p>Tauschen Sie PostgreSQL-Benutzername und Passwort gegen ein kurzlebiges JWT über die Methode <code>get_jwt</code>. Zeigt den dekodierten Payload und die Ablaufzeit — nützlich zur Fehlersuche in Token-basierten Abläufen.</p>
<p><a href="get-jwt/" class="btn">Mehr über den JWT Getter</a></p>
</div>

<div class="card">
<h3>PgArachne Toolbar (macOS)</h3>
<p>Eine native macOS-Anwendung für die Menüleiste. Verwalten Sie mehrere PgArachne-Instanzen, sehen Sie Live-Logs ein und überwachen Sie Metriken mit einem Klick.</p>
<p><a href="macos-toolbar/" class="btn">Toolbar-Funktionen entdecken</a></p>
</div>
</div>
</section>
