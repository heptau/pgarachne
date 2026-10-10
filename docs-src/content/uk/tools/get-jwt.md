---
title: "JWT Getter"
description: "PgArachne JWT Getter - Documentation"
---

<section id="get-jwt">
<h2>JWT Getter</h2>
<p><strong>JWT Getter</strong> — це мінімалістичний однофайловий браузерний інструмент, розташований у <code>tools/jwt-getter</code>. Він обмінює ім'я користувача та пароль PostgreSQL на короткостроковий JWT, викликаючи <code>POST /{prefix}/{database}/token</code> з обліковими даними HTTP Basic — і показує вам декодований payload і час завершення дії.</p>
<p>Онлайн-версія: <a href="https://jwt-getter.pgarachne.com" target="_blank" rel="noopener">jwt-getter.pgarachne.com</a></p>

<figure class="tool-screenshot">
<img src="/assets/pgarachne-tool-jwt-getter.webp" alt="JWT Getter" loading="lazy">
</figure>


<h3>Коли використовувати</h3>
<p>Використовуйте JWT Getter, коли вам потрібен токен для однієї з таких цілей:</p>
<ul>
<li>Вставлення токена в <a href="../api-explorer/">Explorer</a> або <a href="../sse-tester/">SSE Tester</a> (вкладка Bearer Token).</li>
<li>Перевірка терміну дії JWT або claim-ів <code>db_role</code> / <code>db_name</code> у вашому застосунку.</li>
<li>Швидка перевірка того, що системний користувач <code>pgarachne</code> може перемкнутися на певну роль (<code>GRANT &lt;role&gt; TO pgarachne</code> має бути налаштований).</li>
</ul>

<div class="tip">
<strong>Примітка:</strong> JWT працює лише тоді, коли PgArachne може перемкнутися на роль користувача, що вимагає <code>GRANT &lt;role&gt; TO pgarachne</code> у базі даних (PgArachne використовує <code>SET LOCAL ROLE</code> для запитів, автентифікованих токеном). Якщо ви хочете уникнути цього grant-а, використовуйте прямі облікові дані (HTTP Basic Auth) у <a href="../api-explorer/">Explorer</a> або <a href="../sse-tester/">SSE Tester</a>.
</div>

<h3>Можливості</h3>
<ul>
<li>Введіть URL API, префікс, базу даних, ім'я користувача та пароль — натисніть <kbd>Enter</kbd> або клацніть <strong>Get JWT</strong>.</li>
<li>Відображається необроблений токен, який можна скопіювати одним клацанням.</li>
<li>Payload JWT декодується з Base64 і відображається з підсвічуванням синтаксису JSON.</li>
<li>Значок терміну дії показує залишковий час життя (або позначає токен як застарілий).</li>
<li>Налаштування з'єднання та вхід зберігаються в <code>localStorage</code>.</li>
</ul>

<h3>Як увімкнути</h3>
<p>Встановіть <code>STATIC_FILES_PATH</code> на <code>tools/jwt-getter</code> і відвідайте <code>http://localhost:8080</code>, або відкрийте <code>index.html</code> безпосередньо в браузері.</p>
</section>
