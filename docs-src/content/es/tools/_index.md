---
title: "Herramientas"
description: "Herramientas para PgArachne - Documentación."
menu:
  main:
    name: "Herramientas"
    weight: 70
---

<section id="tools">
<h2>Herramientas</h2>
<p>PgArachne incluye un conjunto de herramientas para el navegador que simplifican el desarrollo, las pruebas y la exploración de la API. Cada herramienta es un único archivo HTML — sin pasos de compilación ni dependencias.</p>

<div class="tools-grid">
<div class="card">
<h3>PgArachne Explorer</h3>
<p>Una interfaz web completa para explorar su API, probar funciones y ver la documentación generada automáticamente. Compatible con credenciales directas (HTTP Basic Auth) y autenticación mediante Bearer token.</p>
<p><a href="api-explorer/" class="btn stretched-link">Más información sobre el Explorer</a></p>
</div>

<div class="card">
<h3>SSE Tester</h3>
<p>Suscríbase a uno o más canales PostgreSQL NOTIFY a través de una conexión Server-Sent Events en vivo. Compatible con los tres métodos de autenticación y muestra eventos JSON con resaltado de sintaxis.</p>
<p><a href="sse-tester/" class="btn stretched-link">Más información sobre el SSE Tester</a></p>
</div>

<div class="card">
<h3>JWT Getter</h3>
<p>Intercambie un nombre de usuario y contraseña de PostgreSQL por un JWT de corta duración mediante el endpoint <code>/token</code> (credenciales HTTP Basic). Muestra el payload decodificado y el tiempo de expiración.</p>
<p><a href="get-jwt/" class="btn stretched-link">Más información sobre el JWT Getter</a></p>
</div>

<div class="card">
<h3>JWT Signer</h3>
<p>Genere JWT localmente en el navegador introduciendo <code>JWT_SECRET</code> a mano — sin base de datos ni servidor. Defina rol, base de datos, vigencia y claims adicionales, incluso un token ya caducado para pruebas. Incluye todas las advertencias de seguridad.</p>
<p><a href="jwt-signer/" class="btn stretched-link">Más sobre JWT Signer</a></p>
</div>

<div class="card">
<h3>PgArachne Toolbar (macOS)</h3>
<p>Una aplicación nativa de macOS en la barra de menús. Gestione múltiples instancias de PgArachne, consulte registros en vivo y supervise métricas con un solo clic.</p>
<p><a href="macos-toolbar/" class="btn stretched-link">Explorar funciones del Toolbar</a></p>
</div>
</div>
</section>
