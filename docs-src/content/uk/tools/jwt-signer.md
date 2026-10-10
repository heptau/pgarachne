---
title: "JWT Signer"
description: "PgArachne JWT Signer"
---

<section id="jwt-signer">
<h2>JWT Signer</h2>
<p><strong>JWT Signer</strong> — це однофайловий інструмент для браузера в <code>tools/jwt-signer</code>. На відміну від <a href="../get-jwt/">JWT Getter</a>, він ніколи не звертається до PgArachne чи бази даних: ви вводите <code>JWT_SECRET</code> вручну, а токен підписується (HS256) безпосередньо у вашому браузері.</p>
<p>Онлайн-версія: <a href="https://jwt-signer.pgarachne.com" target="_blank" rel="noopener">jwt-signer.pgarachne.com</a></p>

<figure class="tool-screenshot">
<img src="/assets/pgarachne-tool-jwt-signer.webp" alt="JWT Signer" loading="lazy">
</figure>

<div class="warning">
<strong>Поводьтеся з секретом JWT обережно</strong>
<ul>
<li><code>JWT_SECRET</code> — головний ключ вашого API: хто його знає, може підписати токен для <em>будь-якої</em> ролі бази даних, зокрема суперкористувачів.</li>
<li><strong>Ніколи не вставляйте виробничий секрет на веб-сторінку, яку ви повністю не контролюєте.</strong> Хостовану версію використовуйте лише з секретами для розробки/тестів. Для продакшну збережіть <code>index.html</code> і відкрийте локально (працює офлайн) або підписуйте токени на сервері.</li>
<li>Усе працює у вашому браузері. Сторінка забороняє будь-який мережевий доступ (Content Security Policy <code>connect-src 'none'</code>), не завантажує зовнішніх ресурсів і <strong>ніколи не зберігає секрет</strong> — ні в <code>localStorage</code>, ні в cookies, ні в URL. Запам’ятовуються лише роль, база, строк дії, issuer і audience.</li>
<li>Токен, підписаний тут, PgArachne приймає так само, як виданий через <code>/token</code>. Обирайте короткий строк дії і ніколи не діліться токенами, підписаними справжнім секретом.</li>
</ul>
</div>

<h3>Коли його використовувати</h3>
<ul>
<li>Розробка й тестування, коли ви не хочете (або не можете) входити з паролем до бази даних.</li>
<li>Перевірка, як ваш клієнт обробляє завершення строку дії: від’ємний строк створює вже прострочений токен.</li>
<li>Випробування <code>iss</code> / <code>aud</code> і додаткових claims на сервері, що їх вимагає.</li>
<li>Генерація випадкового 256-бітного секрету для нового <code>JWT_SECRET</code> одним кліком.</li>
</ul>

<div class="tip">
<strong>Як увімкнути:</strong> Встановіть <code>STATIC_FILES_PATH</code> на <code>tools/jwt-signer</code> і відвідайте <code>http://localhost:8080</code>, або відкрийте <code>index.html</code> безпосередньо в браузері — працює без сервера.
</div>
</section>
