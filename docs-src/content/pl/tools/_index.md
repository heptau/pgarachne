---
title: "Narzędzia"
description: "Narzędzia dla PgArachne - Dokumentacja."
menu:
  main:
    name: "Narzędzia"
    weight: 80
---

<section id="tools">
<h2>Narzędzia</h2>
<p>PgArachne jest wspierany przez zestaw narzędzi działających w przeglądarce, które ułatwiają rozwój, testowanie i eksplorację API. Wszystkie narzędzia są pojedynczymi plikami HTML — bez procesu budowania, bez zależności.</p>

<div class="tools-grid">
<div class="card">
<h3>PgArachne Explorer</h3>
<p>W pełni funkcjonalny interfejs webowy do przeglądania API, testowania funkcji i wyświetlania automatycznie generowanej dokumentacji. Wspiera zarówno uwierzytelnianie bezpośrednimi danymi logowania (HTTP Basic Auth), jak i tokenem Bearer.</p>
<p><a href="api-explorer/" class="btn">Więcej o Explorerze</a></p>
</div>

<div class="card">
<h3>SSE Tester</h3>
<p>Zapisz się do jednego lub wielu kanałów PostgreSQL NOTIFY przez działające na żywo połączenie Server-Sent Events. Wspiera wszystkie trzy metody uwierzytelniania i wyświetla zdarzenia JSON z podświetlaniem składni.</p>
<p><a href="sse-tester/" class="btn">Więcej o SSE Testerze</a></p>
</div>

<div class="card">
<h3>JWT Getter</h3>
<p>Wymień nazwę użytkownika i hasło PostgreSQL na krótkotrwały JWT za pomocą metody <code>get_jwt</code>. Pokazuje zdekodowany payload oraz czas wygaśnięcia — przydatne przy debugowaniu przepływów opartych na tokenach.</p>
<p><a href="get-jwt/" class="btn">Więcej o JWT Getterze</a></p>
</div>

<div class="card">
<h3>PgArachne Toolbar (macOS)</h3>
<p>Natywna aplikacja macOS działająca w pasku menu. Zarządzaj wieloma instancjami PgArachne, przeglądaj logi na żywo i monitoruj metryki jednym kliknięciem.</p>
<p><a href="macos-toolbar/" class="btn">Poznaj funkcje Toolbar</a></p>
</div>
</div>
</section>
