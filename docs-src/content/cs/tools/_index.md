---
title: "Nástroje"
description: "Nástroje pro PgArachne - Dokumentace."
menu:
  main:
    name: "Nástroje"
    weight: 80
---

<section id="tools">
<h2>Nástroje</h2>
<p>PgArachne je doplněn sadou nástrojů pro prohlížeč, které usnadňují vývoj, testování a průzkum API. Každý nástroj je jediný HTML soubor — bez buildu, bez závislostí.</p>

<div class="tools-grid">
<div class="card">
<h3>PgArachne Explorer</h3>
<p>Plnohodnotné webové rozhraní pro procházení API, testování funkcí a zobrazení automaticky generované dokumentace. Podporuje přímé přihlášení heslem (HTTP Basic Auth) i autentizaci přes Bearer token.</p>
<p><a href="api-explorer/" class="btn stretched-link">Více o Exploreru</a></p>
</div>

<div class="card">
<h3>SSE Tester</h3>
<p>Přihlašte se k odběru jednoho či více PostgreSQL NOTIFY kanálů přes živé Server-Sent Events spojení. Podporuje všechny tři způsoby autentizace a zobrazuje JSON události se zvýrazněním syntaxe.</p>
<p><a href="sse-tester/" class="btn stretched-link">Více o SSE Testeru</a></p>
</div>

<div class="card">
<h3>JWT Getter</h3>
<p>Vyměňte PostgreSQL uživatelské jméno a heslo za krátkodobý JWT přes endpoint <code>/token</code> (přihlašovací údaje HTTP Basic). Zobrazuje dekódovaný payload i čas expirace — užitečné pro ladění tokenových toků.</p>
<p><a href="get-jwt/" class="btn stretched-link">Více o JWT Getteru</a></p>
</div>

<div class="card">
<h3>JWT Signer</h3>
<p>Generujte JWT lokálně v prohlížeči po ručním zadání <code>JWT_SECRET</code> — bez databáze i serveru. Nastavte roli, databázi, platnost i další claimy, klidně i už expirovaný token pro testování. Obsahuje všechna bezpečnostní upozornění.</p>
<p><a href="jwt-signer/" class="btn stretched-link">Více o JWT Signeru</a></p>
</div>

<div class="card">
<h3>PgArachne Toolbar (macOS)</h3>
<p>Nativní macOS aplikace pro horní lištu. Spravujte více instancí PgArachne, sledujte živé logy a metriky jediným kliknutím.</p>
<p><a href="macos-toolbar/" class="btn stretched-link">Funkce aplikace Toolbar</a></p>
</div>
</div>
</section>
