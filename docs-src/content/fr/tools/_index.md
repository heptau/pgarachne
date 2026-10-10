---
title: "Outils"
description: "Outils pour PgArachne - Documentation."
menu:
  main:
    name: "Outils"
    weight: 70
---

<section id="tools">
<h2>Outils</h2>
<p>PgArachne est accompagné d'un ensemble d'outils pour navigateur qui simplifient le développement, les tests et l'exploration de l'API. Chaque outil est un simple fichier HTML — sans étape de compilation ni dépendances.</p>

<div class="tools-grid">
<div class="card">
<h3>PgArachne Explorer</h3>
<p>Une interface web complète pour explorer votre API, tester des fonctions et consulter la documentation générée automatiquement. Supporte les identifiants directs (HTTP Basic Auth) et l'authentification par Bearer token.</p>
<p><a href="api-explorer/" class="btn stretched-link">En savoir plus sur l'Explorer</a></p>
</div>

<div class="card">
<h3>SSE Tester</h3>
<p>Abonnez-vous à un ou plusieurs canaux PostgreSQL NOTIFY via une connexion Server-Sent Events en direct. Compatible avec les trois méthodes d'authentification et affiche les événements JSON avec coloration syntaxique.</p>
<p><a href="sse-tester/" class="btn stretched-link">En savoir plus sur le SSE Tester</a></p>
</div>

<div class="card">
<h3>JWT Getter</h3>
<p>Échangez un nom d'utilisateur et un mot de passe PostgreSQL contre un JWT de courte durée via l'endpoint <code>/token</code> (identifiants HTTP Basic). Affiche le payload décodé et la date d'expiration.</p>
<p><a href="get-jwt/" class="btn stretched-link">En savoir plus sur le JWT Getter</a></p>
</div>

<div class="card">
<h3>JWT Signer</h3>
<p>Générez des JWT localement dans le navigateur en saisissant <code>JWT_SECRET</code> à la main — sans base de données ni serveur. Définissez rôle, base, durée de validité et claims supplémentaires, y compris un jeton déjà expiré pour les tests. Avec tous les avertissements de sécurité.</p>
<p><a href="jwt-signer/" class="btn stretched-link">En savoir plus sur JWT Signer</a></p>
</div>

<div class="card">
<h3>PgArachne Toolbar (macOS)</h3>
<p>Une application macOS native dans la barre des menus. Gérez plusieurs instances de PgArachne, consultez les journaux en direct et surveillez les métriques en un clic.</p>
<p><a href="macos-toolbar/" class="btn stretched-link">Explorer les fonctionnalités du Toolbar</a></p>
</div>
</div>
</section>
