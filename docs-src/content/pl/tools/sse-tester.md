---
title: "SSE Tester"
description: "PgArachne SSE Tester - Documentation"
---

<section id="sse-tester">
<h2>SSE Tester</h2>
<p><strong>SSE Tester</strong> to narzędzie przeglądarkowe w postaci jednego pliku, znajdujące się w <code>tools/test-sse</code>. Pozwala zapisać się do dowolnej liczby kanałów PostgreSQL <code>NOTIFY</code> przez działające na żywo połączenie <a href="../../real-time-notifications/">Server-Sent Events</a> i obserwować przychodzące zdarzenia w czasie rzeczywistym.</p>

<h3>Funkcje</h3>
<ul>
<li><strong>Subskrypcje wielu kanałów</strong> — dodawaj lub usuwaj kanały jako tagi (pill tags) przed połączeniem. Wszystkie kanały są przekazywane jako parametr zapytania <code>?channels=</code> oddzielony przecinkami.</li>
<li><strong>Wszystkie metody uwierzytelniania</strong> — zakładka <em>Password</em> używa HTTP Basic Auth (bezpośrednie dane logowania, bez potrzeby JWT); zakładka <em>Bearer Token</em> przyjmuje JWT lub token API.</li>
<li><strong>Strumień na żywo</strong> — wykorzystuje Fetch Streams API z <code>AbortController</code> dla czystego rozłączenia. Linie SSE są parsowane ręcznie, dzięki czemu pola <code>event:</code>, <code>data:</code> oraz heartbeat (<code>:</code>) są obsługiwane prawidłowo.</li>
<li><strong>Podświetlanie składni JSON</strong> — payloady danych będące poprawnym JSON-em są formatowane z kolorowym podświetleniem.</li>
<li><strong>Trwałe ustawienia</strong> — URL API, prefiks, baza danych, dane logowania oraz lista kanałów są zapisywane w <code>localStorage</code>.</li>
</ul>

<h3>Uwierzytelnianie</h3>
<p>Narzędzie weryfikuje dane logowania przed odblokowaniem panelu subskrypcji. Kliknięcie <strong>Verify &amp; Continue</strong> wywołuje <code>capabilities</code> z wybranym nagłówkiem — jeśli PgArachne zwróci poprawną odpowiedź, panel subskrypcji zostaje odblokowany.</p>

<ul>
<li><strong>Zakładka Password</strong> — wysyła <code>Authorization: Basic &lt;base64(user:pass)&gt;</code>. Użytkownik bazy danych musi mieć <code>GRANT EXECUTE</code> na docelowych funkcjach oraz dostęp do endpointu SSE; <code>GRANT &lt;role&gt; TO pgarachne</code> nie jest wymagany.</li>
<li><strong>Zakładka Bearer Token</strong> — wysyła <code>Authorization: Bearer &lt;token&gt;</code>. Wklej JWT (uzyskany za pomocą <a href="../get-jwt/">JWT Getter</a>) lub długotrwały token API.</li>
</ul>

<div class="tip">
<strong>Jak go włączyć:</strong> Ustaw <code>STATIC_FILES_PATH</code> na <code>tools/test-sse</code> i odwiedź <code>http://localhost:8080</code>. Możesz też otworzyć <code>index.html</code> bezpośrednio w przeglądarce — cała funkcjonalność działa bez lokalnego serwera.
</div>
</section>
