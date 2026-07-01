---
title: "JWT Getter"
description: "PgArachne JWT Getter - Documentation"
---

<section id="get-jwt">
<h2>JWT Getter</h2>
<p>Le <strong>JWT Getter</strong> est un outil minimaliste pour navigateur situé dans <code>tools/get-jwt</code>. Il échange un nom d'utilisateur et un mot de passe PostgreSQL contre un JWT de courte durée en appelant la méthode JSON-RPC <code>get_jwt</code> — et affiche le payload décodé ainsi que la date d'expiration.</p>

<h3>Quand l'utiliser</h3>
<p>Utilisez le JWT Getter lorsque vous avez besoin d'un token pour l'une de ces raisons :</p>
<ul>
<li>Pour coller un token dans l'onglet Bearer Token de l'<a href="../api-explorer/">Explorer</a> ou du <a href="../sse-tester/">SSE Tester</a>.</li>
<li>Pour tester l'expiration d'un JWT ou les claims <code>db_role</code>/<code>db_name</code> dans votre application.</li>
<li>Pour vérifier que l'utilisateur système <code>pgarachne</code> peut se substituer à un rôle donné (<code>GRANT &lt;rôle&gt; TO pgarachne</code> doit exister).</li>
</ul>

<div class="tip">
<strong>Note :</strong> <code>get_jwt</code> nécessite <code>GRANT &lt;rôle&gt; TO pgarachne</code> dans la base de données, car PgArachne vérifie le mot de passe en interne via <code>SET LOCAL ROLE</code>. Pour éviter ce grant, utilisez les identifiants directs (HTTP Basic Auth) dans l'<a href="../api-explorer/">Explorer</a> ou le <a href="../sse-tester/">SSE Tester</a>.
</div>

<h3>Fonctionnalités</h3>
<ul>
<li>Remplissez l'URL, le préfixe, la base de données, le nom d'utilisateur et le mot de passe — appuyez sur <kbd>Entrée</kbd> ou cliquez sur <strong>Get JWT</strong>.</li>
<li>Le token généré s'affiche et peut être copié en un clic.</li>
<li>Le payload du JWT est décodé en Base64 et affiché avec coloration syntaxique JSON.</li>
<li>Un badge indique la durée de vie restante (ou marque le token comme expiré).</li>
<li>Les paramètres de connexion et l'identifiant sont sauvegardés dans <code>localStorage</code>.</li>
</ul>

<h3>Activation</h3>
<p>Définissez <code>STATIC_FILES_PATH</code> sur <code>tools/get-jwt</code> et visitez <code>http://localhost:8080</code>, ou ouvrez <code>index.html</code> directement dans le navigateur.</p>
</section>
