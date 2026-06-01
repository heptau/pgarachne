---
title: "SSE Tester"
description: "PgArachne SSE Tester - Documentación"
---

<section id="sse-tester">
<h2>SSE Tester</h2>
<p>El <strong>SSE Tester</strong> es una herramienta de un único archivo para el navegador, ubicada en <code>tools/test-sse</code>. Permite suscribirse a cualquier número de canales PostgreSQL <code>NOTIFY</code> a través de una conexión <a href="../../real-time-notifications/">Server-Sent Events</a> en vivo y observar los eventos en tiempo real.</p>

<h3>Funciones</h3>
<ul>
<li><strong>Múltiples canales</strong> — los canales se añaden como etiquetas y se pasan como parámetro <code>?channels=</code>.</li>
<li><strong>Todos los métodos de autenticación</strong> — la pestaña <em>Password</em> usa HTTP Basic Auth (sin JWT); la pestaña <em>Bearer Token</em> acepta un JWT o token de API.</li>
<li><strong>Stream en vivo</strong> — utiliza la Fetch Streams API con <code>AbortController</code> para una desconexión limpia.</li>
<li><strong>Resaltado de sintaxis JSON</strong> — los payloads JSON se muestran con colores.</li>
<li><strong>Configuración persistente</strong> — URL, prefijo, base de datos, nombre de usuario y lista de canales se guardan en <code>localStorage</code>.</li>
</ul>

<h3>Autenticación</h3>
<ul>
<li><strong>Pestaña Password</strong> — envía <code>Authorization: Basic &lt;base64(usuario:contraseña)&gt;</code>. No requiere <code>GRANT &lt;rol&gt; TO pgarachne</code>.</li>
<li><strong>Pestaña Bearer Token</strong> — envía <code>Authorization: Bearer &lt;token&gt;</code>. Pegue un JWT (obtenido con el <a href="../get-jwt/">JWT Getter</a>) o un token de API duradero.</li>
</ul>

<div class="tip">
<strong>Cómo habilitarlo:</strong> Establezca <code>STATIC_FILES_PATH</code> en <code>tools/test-sse</code> y visite <code>http://localhost:8080</code>. También puede abrir <code>index.html</code> directamente en el navegador.
</div>
</section>
