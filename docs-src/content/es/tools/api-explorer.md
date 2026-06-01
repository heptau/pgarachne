---
title: "Explorador de PgArachne"
description: "Explorador de PgArachne - PgArachne"
---

<section id="explorer">
<h2>Explorador de PgArachne</h2>
<p>El <strong>Explorer</strong> es una potente interfaz web incluida en el directorio <code>tools/pgarachne-explorer</code>. No es
		solo documentación; es una <strong>aplicación de demostración</strong> completamente funcional construida usando
		HTML/JS
		que se comunica con la base de datos exclusivamente a través de PgArachne. También puedes usar la versión alojada en 
<a href="https://explorer.pgarachne.com" target="_blank">explorer.pgarachne.com</a>.</p>

<p><strong>¿Qué puede hacer?</strong></p>
<ul>
<li><strong>Inspeccionar API:</strong> Lee la función <code>capabilities</code> para mostrar todos los
			endpoints disponibles y sus parámetros.</li>
<li><strong>Pruebas en Vivo:</strong> Puedes ejecutar funciones directamente desde el navegador.</li>
<li><strong>Autodocumentación:</strong> Renderiza los comentarios SQL (incluyendo metadatos <code>--- PARAMS ---</code>)
			en documentación legible.</li>
<li><strong>Interfaz moderna:</strong> Incluye soporte para modo oscuro/claro y se puede instalar como una <strong>PWA</strong> en dispositivos móviles.</li>
<li><strong>Autenticación:</strong> La pestaña <em>Password</em> usa HTTP Basic Auth para conectarse directamente como usuario de la base de datos — sin necesidad de <code>GRANT … TO pgarachne</code>. La pestaña <em>API Token</em> acepta un JWT o un token de API duradero como <code>Bearer</code>.</li>
</ul>

<p><strong>Parámetros de URL:</strong></p>
<p>Puedes pre-completar los ajustes de conexión usando el parámetro <code>?url=</code>, por ejemplo: 
<code>?url=http://localhost:8080/db/mi_base_de_datos</code>. El Explorador dividirá automáticamente la URL en los campos de Host de API, Prefijo y Base de datos.</p>

<div class="tip">
<strong>Cómo habilitarlo:</strong> Establece la variable de entorno <code>STATIC_FILES_PATH</code> para que apunte a la
		carpeta <code>tools/pgarachne-explorer</code> en tu disco. Luego visita <code>http://localhost:8080</code>.
</div>
</section>
