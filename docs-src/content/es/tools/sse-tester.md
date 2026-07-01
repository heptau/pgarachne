---
title: "SSE Tester"
description: "PgArachne SSE Tester - Documentación"
---

<section id="sse-tester">
<h2>SSE Tester</h2>
<p>El <strong>SSE Tester</strong> es una herramienta de un único archivo para el navegador, ubicada en <code>tools/test-sse</code>. Permite suscribirse a cualquier número de canales PostgreSQL <code>NOTIFY</code> a través de una conexión <a href="../../real-time-notifications/">Server-Sent Events</a> en vivo y observar los eventos en tiempo real.</p>

<h3>Funciones</h3>
<ul>
<li><strong>Múltiples canales</strong> — añada o elimine canales como etiquetas antes de conectar. Todos los canales se pasan como un parámetro <code>?channels=</code> separado por comas.</li>
<li><strong>Todos los métodos de autenticación</strong> — la pestaña <em>Password</em> usa HTTP Basic Auth (credenciales directas, sin JWT necesario); la pestaña <em>Bearer Token</em> acepta un JWT o token de API.</li>
<li><strong>Stream en vivo</strong> — utiliza la Fetch Streams API con un <code>AbortController</code> para una desconexión limpia. Las líneas SSE se parsean manualmente, de modo que los campos <code>event:</code>, <code>data:</code> y heartbeat (<code>:</code>) se gestionan correctamente.</li>
<li><strong>Resaltado de sintaxis JSON</strong> — los payloads JSON se muestran con colores.</li>
<li><strong>Configuración persistente</strong> — URL, prefijo, base de datos, nombre de usuario y lista de canales se guardan en <code>localStorage</code>.</li>
</ul>

<h3>Autenticación</h3>
<p>La herramienta verifica las credenciales antes de mostrar el panel de suscripción. Al hacer clic en <strong>Verify &amp; Continue</strong> se llama a <code>capabilities</code> con el encabezado elegido — si PgArachne devuelve una respuesta correcta, el panel de suscripción se desbloquea.</p>

<ul>
<li><strong>Pestaña Password</strong> — envía <code>Authorization: Basic &lt;base64(usuario:contraseña)&gt;</code>. El usuario de base de datos debe tener <code>GRANT EXECUTE</code> sobre las funciones objetivo y acceso al endpoint SSE; no requiere <code>GRANT &lt;rol&gt; TO pgarachne</code>.</li>
<li><strong>Pestaña Bearer Token</strong> — envía <code>Authorization: Bearer &lt;token&gt;</code>. Pegue un JWT (obtenido con el <a href="../get-jwt/">JWT Getter</a>) o un token de API duradero.</li>
</ul>

<div class="tip">
<strong>Cómo habilitarlo:</strong> Establezca <code>STATIC_FILES_PATH</code> en <code>tools/test-sse</code> y visite <code>http://localhost:8080</code>. También puede abrir <code>index.html</code> directamente en el navegador.
</div>
</section>
