---
title: "Explorateur PgArachne"
description: "Explorateur PgArachne - PgArachne"
---

<section id="explorer">
<h2>Explorateur PgArachne</h2>
<p>L’<strong>Explorer</strong> est une interface web puissante incluse dans le répertoire <code>tools/pgarachne-explorer</code>. Ce n’est
		pas seulement de la documentation ; c’est une <strong>application de démonstration</strong> entièrement
		fonctionnelle construite en HTML/JS qui communique avec la base de données exclusivement via PgArachne. Vous pouvez également utiliser la version hébergée sur 
<a href="https://explorer.pgarachne.com" target="_blank">explorer.pgarachne.com</a>.</p>

<p><strong>Que peut-il faire ?</strong></p>
<ul>
<li><strong>Inspecter l’API :</strong> Il lit la fonction <code>capabilities</code> pour afficher tous les endpoints
			disponibles et leurs paramètres.</li>
<li><strong>Tests en direct :</strong> Vous pouvez exécuter des fonctions directement depuis
			le navigateur.</li>
<li><strong>Auto-Documentation :</strong> Il rend les commentaires SQL (y compris les métadonnées <code>---
			PARAMS ---</code>) en documentation lisible.</li>
<li><strong>Interface moderne :</strong> Propose un support pour le mode sombre/clair et peut être installée comme une <strong>PWA</strong> sur les appareils mobiles.</li>
<li><strong>Authentification :</strong> L’onglet <em>Password</em> utilise HTTP Basic Auth pour se connecter directement en tant qu’utilisateur de la base de données — sans <code>GRANT … TO pgarachne</code>. L’onglet <em>API Token</em> accepte un JWT ou un token d’API longue durée en tant que <code>Bearer</code>.</li>
</ul>

<p><strong>Paramètres URL :</strong></p>
<p>Vous pouvez pré-remplir les paramètres de connexion en utilisant le paramètre <code>?url=</code>, par exemple : 
<code>?url=http://localhost:8080/db/ma_base_de_donnees</code>. L’Explorateur divisera automatiquement l’URL en champs Hôte API, Préfixe et Base de données.</p>

<div class="tip">
<strong>Comment l’activer :</strong> Définissez la variable d’environnement <code>STATIC_FILES_PATH</code>
		pour pointer vers le dossier <code>tools/pgarachne-explorer</code> sur votre disque. Puis visitez <code>http://localhost:8080</code>.
</div>
</section>
