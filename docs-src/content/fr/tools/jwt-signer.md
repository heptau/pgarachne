---
title: "JWT Signer"
description: "PgArachne JWT Signer"
---

<section id="jwt-signer">
<h2>JWT Signer</h2>
<p><strong>JWT Signer</strong> est un outil de navigateur en un seul fichier, situé dans <code>tools/jwt-signer</code>. Contrairement au <a href="../get-jwt/">JWT Getter</a>, il ne contacte jamais PgArachne ni la base de données : vous saisissez <code>JWT_SECRET</code> à la main et le jeton est signé (HS256) directement dans votre navigateur.</p>
<p>Version en ligne: <a href="https://jwt-signer.pgarachne.com" target="_blank" rel="noopener">jwt-signer.pgarachne.com</a></p>

<figure class="tool-screenshot">
<img src="/assets/pgarachne-tool-jwt-signer.webp" alt="JWT Signer" loading="lazy">
</figure>

<div class="warning">
<strong>Manipulez votre secret JWT avec précaution</strong>
<ul>
<li><code>JWT_SECRET</code> est la clé maîtresse de votre API : quiconque la connaît peut signer un jeton pour <em>n'importe quel</em> rôle de base de données, y compris les superutilisateurs.</li>
<li><strong>Ne collez jamais un secret de production dans une page web que vous ne contrôlez pas entièrement.</strong> N'utilisez la version hébergée qu'avec des secrets de développement/test. En production, enregistrez <code>index.html</code> et ouvrez-le localement (il fonctionne hors ligne), ou signez les jetons sur le serveur.</li>
<li>Tout s'exécute dans votre navigateur. La page interdit tout accès réseau (Content Security Policy <code>connect-src 'none'</code>), ne charge aucune ressource externe et <strong>ne stocke jamais le secret</strong> — ni dans <code>localStorage</code>, ni dans les cookies, ni dans l'URL. Seuls le rôle, la base, la durée, l'issuer et l'audience sont mémorisés.</li>
<li>PgArachne accepte un jeton signé ici exactement comme un jeton émis par <code>/token</code>. Choisissez des durées courtes et ne partagez jamais de jetons signés avec un vrai secret.</li>
</ul>
</div>

<h3>Quand l'utiliser</h3>
<ul>
<li>Développement et tests quand vous ne voulez pas (ou ne pouvez pas) vous connecter avec un mot de passe de base de données.</li>
<li>Vérifier comment votre client gère l'expiration : une durée négative produit un jeton déjà expiré.</li>
<li>Essayer <code>iss</code> / <code>aud</code> et des claims supplémentaires face à un serveur qui les exige.</li>
<li>Générer d'un clic un secret aléatoire de 256 bits pour un nouveau <code>JWT_SECRET</code>.</li>
</ul>

<div class="tip">
<strong>Comment l'activer:</strong> Définissez <code>STATIC_FILES_PATH</code> sur <code>tools/jwt-signer</code> et visitez <code>http://localhost:8080</code>, ou ouvrez <code>index.html</code> directement dans un navigateur — cela fonctionne sans serveur.
</div>
</section>
