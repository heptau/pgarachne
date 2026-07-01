---
title: "SSE Tester"
description: "PgArachne SSE Tester - Documentation"
---

<section id="sse-tester">
<h2>SSE Tester</h2>
<p>Le <strong>SSE Tester</strong> est un outil pour navigateur en fichier unique situé dans <code>tools/test-sse</code>. Il permet de s'abonner à un nombre quelconque de canaux PostgreSQL <code>NOTIFY</code> via une connexion <a href="../../real-time-notifications/">Server-Sent Events</a> en direct et d'observer les événements en temps réel.</p>

<h3>Fonctionnalités</h3>
<ul>
<li><strong>Abonnements multi-canaux</strong> — ajoutez ou retirez des canaux sous forme de tags avant de vous connecter. Tous les canaux sont transmis comme paramètre de requête <code>?channels=</code> séparé par des virgules.</li>
<li><strong>Toutes les méthodes d'authentification</strong> — l'onglet <em>Password</em> utilise HTTP Basic Auth (identifiants directs, sans JWT nécessaire) ; l'onglet <em>Bearer Token</em> accepte un JWT ou un token d'API.</li>
<li><strong>Flux en direct</strong> — utilise la Fetch Streams API avec un <code>AbortController</code> pour une déconnexion propre. Les lignes SSE sont analysées manuellement afin que les champs <code>event:</code>, <code>data:</code> et heartbeat (<code>:</code>) soient tous correctement gérés.</li>
<li><strong>Coloration syntaxique JSON</strong> — les payloads de données qui sont du JSON valide sont formatés avec coloration syntaxique.</li>
<li><strong>Paramètres persistants</strong> — URL de l'API, préfixe, base de données, identifiant et liste de canaux sont sauvegardés dans <code>localStorage</code>.</li>
</ul>

<h3>Authentification</h3>
<p>L'outil vérifie les identifiants avant d'afficher le panneau d'abonnement. Cliquer sur <strong>Verify &amp; Continue</strong> appelle <code>capabilities</code> avec l'en-tête choisi — si PgArachne renvoie une réponse réussie, le panneau d'abonnement se déverrouille.</p>

<ul>
<li><strong>Onglet Password</strong> — envoie <code>Authorization: Basic &lt;base64(utilisateur:mot_de_passe)&gt;</code>. L'utilisateur de base de données doit avoir <code>GRANT EXECUTE</code> sur les fonctions cibles ainsi que l'accès à l'endpoint SSE ; aucun <code>GRANT &lt;rôle&gt; TO pgarachne</code> n'est nécessaire.</li>
<li><strong>Onglet Bearer Token</strong> — envoie <code>Authorization: Bearer &lt;token&gt;</code>. Collez un JWT (obtenu via le <a href="../get-jwt/">JWT Getter</a>) ou un token d'API longue durée.</li>
</ul>

<div class="tip">
<strong>Activation :</strong> Définissez <code>STATIC_FILES_PATH</code> sur <code>tools/test-sse</code> et visitez <code>http://localhost:8080</code>. Vous pouvez aussi ouvrir <code>index.html</code> directement dans le navigateur — toutes les fonctionnalités marchent sans serveur local.
</div>
</section>
