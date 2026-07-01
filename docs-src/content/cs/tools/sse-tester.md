---
title: "SSE Tester"
description: "PgArachne SSE Tester - Dokumentace"
---

<section id="sse-tester">
<h2>SSE Tester</h2>
<p><strong>SSE Tester</strong> je jednoduchý nástroj pro prohlížeč umístěný ve složce <code>tools/test-sse</code>. Umožňuje přihlásit se k odběru libovolného počtu PostgreSQL <code>NOTIFY</code> kanálů přes živé <a href="../../real-time-notifications/">Server-Sent Events</a> spojení a sledovat příchozí události v reálném čase.</p>

<h3>Funkce</h3>
<ul>
<li><strong>Více kanálů najednou</strong> — kanály přidáváte jako tagy před připojením. Všechny kanály jsou předány jako parametr <code>?channels=</code>.</li>
<li><strong>Všechny způsoby autentizace</strong> — záložka <em>Password</em> používá HTTP Basic Auth (přímé přihlášení, bez JWT); záložka <em>Bearer Token</em> přijímá JWT nebo API token.</li>
<li><strong>Živé přijímání dat</strong> — využívá Fetch Streams API s <code>AbortController</code> pro čisté odpojení. Řádky SSE jsou parsovány manuálně, takže pole <code>event:</code>, <code>data:</code> i heartbeat (<code>:</code>) jsou zpracovány správně.</li>
<li><strong>Zvýraznění JSON syntaxe</strong> — datové payload ve formátu JSON jsou zobrazeny s barevným zvýrazněním.</li>
<li><strong>Ukládání nastavení</strong> — URL, prefix, databáze, přihlašovací jméno a seznam kanálů se ukládají do <code>localStorage</code>.</li>
</ul>

<h3>Autentizace</h3>
<p>Nástroj ověří přihlašovací údaje ještě před zobrazením panelu pro odběr kanálů. Kliknutí na <strong>Verify &amp; Continue</strong> zavolá <code>capabilities</code> se zvolenou hlavičkou — pokud PgArachne vrátí úspěšnou odpověď, panel pro odběr se odemkne.</p>

<ul>
<li><strong>Záložka Password</strong> — odesílá <code>Authorization: Basic &lt;base64(uživatel:heslo)&gt;</code>. Databázový uživatel musí mít <code>GRANT EXECUTE</code> na cílové funkce a přístup k SSE endpointu; <code>GRANT &lt;role&gt; TO pgarachne</code> není potřeba.</li>
<li><strong>Záložka Bearer Token</strong> — odesílá <code>Authorization: Bearer &lt;token&gt;</code>. Vložte JWT (získaný přes <a href="../get-jwt/">JWT Getter</a>) nebo dlouhodobý API token.</li>
</ul>

<div class="tip">
<strong>Jak nástroj zprovoznit:</strong> Nastavte <code>STATIC_FILES_PATH</code> na <code>tools/test-sse</code> a navštivte <code>http://localhost:8080</code>. Nebo otevřete <code>index.html</code> přímo v prohlížeči — vše funguje bez lokálního serveru.
</div>
</section>
