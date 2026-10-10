---
title: "JWT Getter"
description: "PgArachne JWT Getter - Documentación"
---

<section id="get-jwt">
<h2>JWT Getter</h2>
<p>El <strong>JWT Getter</strong> es una herramienta minimalista para el navegador ubicada en <code>tools/jwt-getter</code>. Intercambia un nombre de usuario y contraseña de PostgreSQL por un JWT de corta duración llamando a <code>POST /{prefix}/{database}/token</code> con credenciales HTTP Basic, y muestra el payload decodificado y el tiempo de expiración.</p>
<p>Versión en línea: <a href="https://jwt-getter.pgarachne.com" target="_blank" rel="noopener">jwt-getter.pgarachne.com</a></p>

<figure class="tool-screenshot">
<img src="/assets/pgarachne-tool-jwt-getter.webp" alt="JWT Getter" loading="lazy">
</figure>


<h3>Cuándo usarlo</h3>
<p>Use el JWT Getter cuando necesite un token para alguno de estos fines:</p>
<ul>
<li>Para pegar un token en la pestaña Bearer Token del <a href="../api-explorer/">Explorer</a> o del <a href="../sse-tester/">SSE Tester</a>.</li>
<li>Para probar la expiración de un JWT o los claims <code>db_role</code>/<code>db_name</code> en su aplicación.</li>
<li>Para verificar que el usuario del sistema <code>pgarachne</code> puede cambiar a un rol concreto (<code>GRANT &lt;rol&gt; TO pgarachne</code> debe existir).</li>
</ul>

<div class="tip">
<strong>Nota:</strong> un JWT solo funciona si PgArachne puede cambiar al rol del usuario, lo que requiere <code>GRANT &lt;rol&gt; TO pgarachne</code> en la base de datos (PgArachne usa <code>SET LOCAL ROLE</code> en las peticiones autenticadas con token). Si desea evitar ese grant, use credenciales directas (HTTP Basic Auth) en el <a href="../api-explorer/">Explorer</a> o el <a href="../sse-tester/">SSE Tester</a> en su lugar.
</div>

<h3>Funciones</h3>
<ul>
<li>Rellene URL, prefijo, base de datos, usuario y contraseña — pulse <kbd>Enter</kbd> o haga clic en <strong>Get JWT</strong>.</li>
<li>El token generado se muestra y puede copiarse con un clic.</li>
<li>El payload del JWT se decodifica en Base64 y se muestra con resaltado de sintaxis JSON.</li>
<li>Un indicador muestra el tiempo de vida restante (o marca el token como expirado).</li>
<li>La configuración de conexión y el nombre de usuario se guardan en <code>localStorage</code>.</li>
</ul>

<h3>Cómo habilitarlo</h3>
<p>Establezca <code>STATIC_FILES_PATH</code> en <code>tools/jwt-getter</code> y visite <code>http://localhost:8080</code>, o abra <code>index.html</code> directamente en el navegador.</p>
</section>
