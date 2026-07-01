---
title: "PgArachne Explorer"
description: "PgArachne Explorer - PgArachne"
---

<section id="explorer">
<h2>PgArachne Explorer</h2>
<p><strong>Explorer</strong> to zaawansowany interfejs webowy GUI zawarty w katalogu <code>tools/pgarachne-explorer</code>. Nie jest to
		wyłącznie narzędzie dokumentacyjne — to w pełni funkcjonalna <strong>aplikacja demonstracyjna</strong> zbudowana w HTML/JS,
		która komunikuje się z bazą danych wyłącznie za pośrednictwem PgArachne. Hostowana wersja jest również dostępna na
<a href="https://explorer.pgarachne.com" target="_blank">explorer.pgarachne.com</a>.</p>

<p><strong>Co potrafi?</strong></p>
<ul>
<li><strong>Inspekcja API:</strong> Odczytuje funkcję <code>capabilities</code>, aby wyświetlić wszystkie dostępne
			endpointy i ich parametry.</li>
<li><strong>Testowanie na żywo:</strong> Możesz wykonywać funkcje bezpośrednio z przeglądarki.</li>
<li><strong>Automatyczna dokumentacja:</strong> Renderuje komentarze SQL (w tym metadane
			<code>--- PARAMS ---</code>) do postaci czytelnej dokumentacji.</li>
<li><strong>Nowoczesny interfejs:</strong> Wspiera tryb ciemny i jasny oraz może być zainstalowany jako <strong>PWA</strong> na urządzeniach mobilnych.</li>
<li><strong>Uwierzytelnianie:</strong> Zakładka <em>Password</em> używa HTTP Basic Auth do połączenia bezpośrednio jako użytkownik bazy danych — bez potrzeby <code>GRANT &hellip; TO pgarachne</code>. Zakładka <em>API Token</em> przyjmuje JWT lub długotrwały token API wysyłany jako <code>Bearer</code>.</li>
</ul>

<p><strong>Parametry URL:</strong></p>
<p>Ustawienia połączenia możesz wstępnie wypełnić za pomocą parametru <code>?url=</code>, na przykład: 
<code>?url=http://localhost:8080/db/my_database</code>. Spowoduje to automatyczny podział adresu URL na pola API Host, Prefix i Database.</p>

<div class="tip">
<strong>Jak go włączyć:</strong> Ustaw zmienną środowiskową <code>STATIC_FILES_PATH</code>, tak aby wskazywała na
katalog <code>tools/pgarachne-explorer</code> na dysku. Następnie odwiedź <code>http://localhost:8080</code>.
</div>
</section>
