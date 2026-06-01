---
title: "JWT Getter"
description: "PgArachne JWT Getter - Dokumentace"
---

<section id="get-jwt">
<h2>JWT Getter</h2>
<p><strong>JWT Getter</strong> je minimalistický nástroj pro prohlížeč umístěný ve složce <code>tools/get-jwt</code>. Vyměňuje PostgreSQL uživatelské jméno a heslo za krátkodobý JWT voláním JSON-RPC metody <code>get_jwt</code> — a zobrazuje dekódovaný payload i čas expirace.</p>

<h3>Kdy jej použít</h3>
<ul>
<li>Chcete token vložit do záložky Bearer Token v <a href="../api-explorer/">Exploreru</a> nebo <a href="../sse-tester/">SSE Testeru</a>.</li>
<li>Testujete expiraci JWT nebo claimy <code>db_role</code> / <code>db_name</code> ve své aplikaci.</li>
<li>Ověřujete, zda systémový uživatel <code>pgarachne</code> může přepnout na danou roli (musí existovat <code>GRANT &lt;role&gt; TO pgarachne</code>).</li>
</ul>

<div class="tip">
<strong>Poznámka:</strong> <code>get_jwt</code> vyžaduje <code>GRANT &lt;role&gt; TO pgarachne</code>. Pokud chcete tento grant vynechat, použijte přímé přihlášení heslem (HTTP Basic Auth) v <a href="../api-explorer/">Exploreru</a> nebo <a href="../sse-tester/">SSE Testeru</a>.
</div>

<h3>Funkce</h3>
<ul>
<li>Vyplňte URL, prefix, databázi, uživatelské jméno a heslo — stiskněte <kbd>Enter</kbd> nebo tlačítko <strong>Get JWT</strong>.</li>
<li>Vygenerovaný token se zobrazí a lze jej jedním kliknutím zkopírovat.</li>
<li>Payload JWT je dekódován a zobrazen se zvýrazněním syntaxe JSON.</li>
<li>Badge zobrazuje zbývající životnost tokenu (nebo ho označí jako expirovaný).</li>
<li>Nastavení připojení a přihlašovací jméno se ukládají do <code>localStorage</code>.</li>
</ul>

<h3>Zprovoznění</h3>
<p>Nastavte <code>STATIC_FILES_PATH</code> na <code>tools/get-jwt</code> a navštivte <code>http://localhost:8080</code>, nebo otevřete <code>index.html</code> přímo v prohlížeči.</p>
</section>
