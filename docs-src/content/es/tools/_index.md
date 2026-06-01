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
<p><a href="api-explorer/" class="btn">Más información sobre el Explorer</a></p>
</div>

<div class="card">
<h3>SSE Tester</h3>
<p>Suscríbase a uno o más canales PostgreSQL NOTIFY a través de una conexión Server-Sent Events en vivo. Compatible con los tres métodos de autenticación y muestra eventos JSON con resaltado de sintaxis.</p>
<p><a href="sse-tester/" class="btn">Más información sobre el SSE Tester</a></p>
</div>

<div class="card">
<h3>JWT Getter</h3>
<p>Intercambie un nombre de usuario y contraseña de PostgreSQL por un JWT de corta duración mediante el método <code>get_jwt</code>. Muestra el payload decodificado y el tiempo de expiración.</p>
<p><a href="get-jwt/" class="btn">Más información sobre el JWT Getter</a></p>
</div>

<div class="card">
<h3>PgArachne Toolbar (macOS)</h3>
<p>Una aplicación nativa de macOS en la barra de menús. Gestione múltiples instancias de PgArachne, consulte registros en vivo y supervise métricas con un solo clic.</p>
<p><a href="macos-toolbar/" class="btn">Explorar funciones del Toolbar</a></p>
</div>
</div>
</section>
