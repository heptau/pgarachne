---
title: "SSE Tester"
description: "PgArachne SSE Tester - Documentation"
---

<section id="sse-tester">
<h2>SSE Tester</h2>
<p>Le <strong>SSE Tester</strong> est un outil pour navigateur en fichier unique situé dans <code>tools/test-sse</code>. Il permet de s'abonner à un nombre quelconque de canaux PostgreSQL <code>NOTIFY</code> via une connexion <a href="../../real-time-notifications/">Server-Sent Events</a> en direct et d'observer les événements en temps réel.</p>

<h3>Fonctionnalités</h3>
<ul>
<li><strong>Plusieurs canaux</strong> — les canaux s'ajoutent sous forme de tags et sont passés en paramètre <code>?channels=</code>.</li>
<li><strong>Toutes les méthodes d'authentification</strong> — l'onglet <em>Password</em> utilise HTTP Basic Auth (sans JWT) ; l'onglet <em>Bearer Token</em> accepte un JWT ou un token d'API.</li>
<li><strong>Flux en direct</strong> — utilise la Fetch Streams API avec <code>AbortController</code> pour une déconnexion propre.</li>
<li><strong>Coloration syntaxique JSON</strong> — les payloads JSON sont affichés avec coloration.</li>
<li><strong>Paramètres persistants</strong> — URL, préfixe, base de données, identifiant et liste de canaux sont sauvegardés dans <code>localStorage</code>.</li>
</ul>

<h3>Authentification</h3>
<ul>
<li><strong>Onglet Password</strong> — envoie <code>Authorization: Basic &lt;base64(utilisateur:mot_de_passe)&gt;</code>. Aucun <code>GRANT &lt;rôle&gt; TO pgarachne</code> requis.</li>
<li><strong>Onglet Bearer Token</strong> — envoie <code>Authorization: Bearer &lt;token&gt;</code>. Collez un JWT (obtenu via le <a href="../get-jwt/">JWT Getter</a>) ou un token d'API longue durée.</li>
</ul>

<div class="tip">
<strong>Activation :</strong> Définissez <code>STATIC_FILES_PATH</code> sur <code>tools/test-sse</code> et visitez <code>http://localhost:8080</code>. Vous pouvez aussi ouvrir <code>index.html</code> directement dans le navigateur.
</div>
</section>
