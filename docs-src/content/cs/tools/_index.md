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
<p><a href="api-explorer/" class="btn">Více o Exploreru</a></p>
</div>

<div class="card">
<h3>SSE Tester</h3>
<p>Přihlašte se k odběru jednoho či více PostgreSQL NOTIFY kanálů přes živé Server-Sent Events spojení. Podporuje všechny tři způsoby autentizace a zobrazuje JSON události se zvýrazněním syntaxe.</p>
<p><a href="sse-tester/" class="btn">Více o SSE Testeru</a></p>
</div>

<div class="card">
<h3>JWT Getter</h3>
<p>Vyměňte PostgreSQL uživatelské jméno a heslo za krátkodobý JWT přes metodu <code>get_jwt</code>. Zobrazuje dekódovaný payload i čas expirace — užitečné pro ladění tokenových toků.</p>
<p><a href="get-jwt/" class="btn">Více o JWT Getteru</a></p>
</div>

<div class="card">
<h3>PgArachne Toolbar (macOS)</h3>
<p>Nativní macOS aplikace pro horní lištu. Spravujte více instancí PgArachne, sledujte živé logy a metriky jediným kliknutím.</p>
<p><a href="macos-toolbar/" class="btn">Funkce aplikace Toolbar</a></p>
</div>
</div>
</section>
