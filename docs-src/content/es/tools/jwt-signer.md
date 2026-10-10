---
title: "JWT Signer"
description: "PgArachne JWT Signer"
---

<section id="jwt-signer">
<h2>JWT Signer</h2>
<p><strong>JWT Signer</strong> es una herramienta de navegador de un solo archivo en <code>tools/jwt-signer</code>. A diferencia de <a href="../get-jwt/">JWT Getter</a>, nunca contacta con PgArachne ni con la base de datos: usted introduce <code>JWT_SECRET</code> a mano y el token se firma (HS256) directamente en su navegador.</p>
<p>Versión en línea: <a href="https://jwt-signer.pgarachne.com" target="_blank" rel="noopener">jwt-signer.pgarachne.com</a></p>

<figure class="tool-screenshot">
<img src="/assets/pgarachne-tool-jwt-signer.webp" alt="JWT Signer" loading="lazy">
</figure>

<div class="warning">
<strong>Maneje su secreto JWT con cuidado</strong>
<ul>
<li><code>JWT_SECRET</code> es la clave maestra de su API: quien la conozca puede firmar un token para <em>cualquier</em> rol de base de datos, incluidos los superusuarios.</li>
<li><strong>Nunca pegue un secreto de producción en una página web que no controle por completo.</strong> Use la versión alojada solo con secretos de desarrollo/prueba. En producción, guarde <code>index.html</code> y ábralo localmente (funciona sin conexión) o firme los tokens en el servidor.</li>
<li>Todo se ejecuta en su navegador. La página prohíbe cualquier acceso a la red (Content Security Policy <code>connect-src 'none'</code>), no carga recursos externos y <strong>nunca guarda el secreto</strong>: ni en <code>localStorage</code>, ni en cookies, ni en la URL. Solo se recuerdan rol, base de datos, vigencia, issuer y audience.</li>
<li>PgArachne acepta un token firmado aquí igual que uno emitido por <code>/token</code>. Use vigencias cortas y nunca comparta tokens firmados con un secreto real.</li>
</ul>
</div>

<h3>Cuándo usarla</h3>
<ul>
<li>Desarrollo y pruebas cuando no quiere (o no puede) iniciar sesión con la contraseña de la base de datos.</li>
<li>Comprobar cómo gestiona su cliente la caducidad: una vigencia negativa genera un token ya caducado.</li>
<li>Probar <code>iss</code> / <code>aud</code> y claims adicionales contra un servidor que los exija.</li>
<li>Generar con un clic un secreto aleatorio de 256 bits para un nuevo <code>JWT_SECRET</code>.</li>
</ul>

<div class="tip">
<strong>Cómo activarla:</strong> Establezca <code>STATIC_FILES_PATH</code> en <code>tools/jwt-signer</code> y visite <code>http://localhost:8080</code>, o abra <code>index.html</code> directamente en un navegador: funciona sin servidor.
</div>
</section>
