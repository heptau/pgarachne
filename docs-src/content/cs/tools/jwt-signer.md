---
title: "JWT Signer"
description: "PgArachne JWT Signer"
---

<section id="jwt-signer">
<h2>JWT Signer</h2>
<p><strong>JWT Signer</strong> je jednosouborový nástroj pro prohlížeč umístěný ve složce <code>tools/jwt-signer</code>. Na rozdíl od <a href="../get-jwt/">JWT Getteru</a> nikdy nekontaktuje PgArachne ani databázi: <code>JWT_SECRET</code> zadáte ručně a token se podepíše (HS256) přímo ve vašem prohlížeči.</p>
<p>Online verze: <a href="https://jwt-signer.pgarachne.com" target="_blank" rel="noopener">jwt-signer.pgarachne.com</a></p>

<figure class="tool-screenshot">
<img src="/assets/pgarachne-tool-jwt-signer.webp" alt="JWT Signer" loading="lazy">
</figure>

<div class="warning">
<strong>S klíčem JWT zacházejte opatrně</strong>
<ul>
<li><code>JWT_SECRET</code> je hlavní klíč vašeho API: kdo ho zná, může podepsat token pro <em>jakoukoli</em> databázovou roli, včetně superuživatelů.</li>
<li><strong>Nikdy nevkládejte produkční klíč do webové stránky, kterou plně nekontrolujete.</strong> Hostovanou verzi používejte jen s vývojovými / testovacími klíči. Pro produkci si uložte <code>index.html</code> a otevřete ho lokálně (funguje offline), nebo podepisujte tokeny na serveru.</li>
<li>Vše běží ve vašem prohlížeči. Stránka zakazuje jakýkoli síťový přístup (Content Security Policy <code>connect-src 'none'</code>), nenačítá žádné externí zdroje a <strong>klíč nikdy neukládá</strong> — ani do <code>localStorage</code>, cookies nebo URL. Pamatuje se jen role, databáze, platnost, issuer a audience.</li>
<li>Token podepsaný zde PgArachne přijme stejně jako token vydaný přes <code>/token</code>. Volte krátkou platnost a nikdy nesdílejte tokeny podepsané skutečným klíčem.</li>
</ul>
</div>

<h3>Kdy jej použít</h3>
<ul>
<li>Vývoj a testování, když se nechcete (nebo nemůžete) přihlašovat heslem k databázi.</li>
<li>Ověření, jak váš klient zvládá expiraci: záporná platnost vytvoří už expirovaný token.</li>
<li>Vyzkoušení <code>iss</code> / <code>aud</code> a dalších claimů proti serveru, který je vyžaduje.</li>
<li>Vygenerování náhodného 256bitového klíče pro nový <code>JWT_SECRET</code> jedním kliknutím.</li>
</ul>

<div class="tip">
<strong>Zprovoznění:</strong> Nastavte <code>STATIC_FILES_PATH</code> na <code>tools/jwt-signer</code> a navštivte <code>http://localhost:8080</code>, nebo otevřete <code>index.html</code> přímo v prohlížeči — funguje bez jakéhokoli serveru.
</div>
</section>
