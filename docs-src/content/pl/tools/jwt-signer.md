---
title: "JWT Signer"
description: "PgArachne JWT Signer"
---

<section id="jwt-signer">
<h2>JWT Signer</h2>
<p><strong>JWT Signer</strong> to jednoplikowe narzędzie przeglądarkowe w <code>tools/jwt-signer</code>. W odróżnieniu od <a href="../get-jwt/">JWT Getter</a> nigdy nie łączy się z PgArachne ani z bazą danych: wpisujesz <code>JWT_SECRET</code> ręcznie, a token jest podpisywany (HS256) bezpośrednio w przeglądarce.</p>
<p>Wersja online: <a href="https://jwt-signer.pgarachne.com" target="_blank" rel="noopener">jwt-signer.pgarachne.com</a></p>

<figure class="tool-screenshot">
<img src="/assets/pgarachne-tool-jwt-signer.webp" alt="JWT Signer" loading="lazy">
</figure>

<div class="warning">
<strong>Z sekretem JWT obchodź się ostrożnie</strong>
<ul>
<li><code>JWT_SECRET</code> to klucz główny Twojego API: kto go zna, może podpisać token dla <em>dowolnej</em> roli bazy danych, także superużytkowników.</li>
<li><strong>Nigdy nie wklejaj sekretu produkcyjnego na stronę, której w pełni nie kontrolujesz.</strong> Wersji hostowanej używaj tylko z sekretami deweloperskimi/testowymi. W produkcji zapisz <code>index.html</code> i otwórz go lokalnie (działa offline) albo podpisuj tokeny na serwerze.</li>
<li>Wszystko działa w Twojej przeglądarce. Strona blokuje cały dostęp do sieci (Content Security Policy <code>connect-src 'none'</code>), nie ładuje zasobów zewnętrznych i <strong>nigdy nie zapisuje sekretu</strong> — ani w <code>localStorage</code>, ani w cookies, ani w URL. Zapamiętywane są tylko rola, baza, ważność, issuer i audience.</li>
<li>Token podpisany tutaj PgArachne przyjmuje tak samo jak wydany przez <code>/token</code>. Wybieraj krótkie okresy ważności i nigdy nie udostępniaj tokenów podpisanych prawdziwym sekretem.</li>
</ul>
</div>

<h3>Kiedy go używać</h3>
<ul>
<li>Rozwój i testy, gdy nie chcesz (lub nie możesz) logować się hasłem do bazy danych.</li>
<li>Sprawdzenie, jak klient radzi sobie z wygaśnięciem: ujemna ważność tworzy token już wygasły.</li>
<li>Wypróbowanie <code>iss</code> / <code>aud</code> i dodatkowych claimów na serwerze, który ich wymaga.</li>
<li>Wygenerowanie jednym kliknięciem losowego 256-bitowego sekretu dla nowego <code>JWT_SECRET</code>.</li>
</ul>

<div class="tip">
<strong>Jak go włączyć:</strong> Ustaw <code>STATIC_FILES_PATH</code> na <code>tools/jwt-signer</code> i odwiedź <code>http://localhost:8080</code> albo otwórz <code>index.html</code> bezpośrednio w przeglądarce — działa bez serwera.
</div>
</section>
