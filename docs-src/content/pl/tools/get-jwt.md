---
title: "JWT Getter"
description: "PgArachne JWT Getter - Documentation"
---

<section id="get-jwt">
<h2>JWT Getter</h2>
<p><strong>JWT Getter</strong> to minimalistyczne narzędzie przeglądarkowe w postaci jednego pliku, znajdujące się w <code>tools/get-jwt</code>. Wymienia nazwę użytkownika i hasło PostgreSQL na krótkotrwały JWT, wywołując metodę JSON-RPC <code>get_jwt</code> — i pokazuje zdekodowany payload oraz czas wygaśnięcia.</p>

<h3>Kiedy go używać</h3>
<p>Skorzystaj z JWT Getter, gdy potrzebujesz tokenu do jednego z tych celów:</p>
<ul>
<li>Wklejenia tokenu do <a href="../api-explorer/">Explorera</a> lub <a href="../sse-tester/">SSE Testera</a> (zakładka Bearer Token).</li>
<li>Testowania wygaśnięcia JWT lub claimów <code>db_role</code> / <code>db_name</code> w swojej aplikacji.</li>
<li>Szybkiego sprawdzenia, czy systemowy użytkownik <code>pgarachne</code> może przełączyć się na daną rolę (musi być ustawiony <code>GRANT &lt;role&gt; TO pgarachne</code>).</li>
</ul>

<div class="tip">
<strong>Uwaga:</strong> <code>get_jwt</code> wymaga <code>GRANT &lt;role&gt; TO pgarachne</code> w bazie danych, ponieważ PgArachne weryfikuje hasło wewnętrznie za pomocą <code>SET LOCAL ROLE</code>. Jeśli chcesz obejść ten grant, użyj bezpośrednich danych logowania (HTTP Basic Auth) w <a href="../api-explorer/">Explorerze</a> lub <a href="../sse-tester/">SSE Testerze</a>.
</div>

<h3>Funkcje</h3>
<ul>
<li>Wprowadź URL API, prefiks, bazę danych, nazwę użytkownika i hasło — naciśnij <kbd>Enter</kbd> lub kliknij <strong>Get JWT</strong>.</li>
<li>Surowy token jest wyświetlany i można go skopiować jednym kliknięciem.</li>
<li>Payload JWT jest dekodowany z Base64 i wyświetlany z podświetlaniem składni JSON.</li>
<li>Odznaka wygaśnięcia pokazuje pozostały czas życia tokenu (lub oznacza token jako wygasły).</li>
<li>Ustawienia połączenia i dane logowania są zapisywane w <code>localStorage</code>.</li>
</ul>

<h3>Jak go włączyć</h3>
<p>Ustaw <code>STATIC_FILES_PATH</code> na <code>tools/get-jwt</code> i odwiedź <code>http://localhost:8080</code>, albo otwórz <code>index.html</code> bezpośrednio w przeglądarce.</p>
</section>
