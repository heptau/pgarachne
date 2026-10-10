---
title: "Інструменти"
description: "Інструменти для PgArachne - Документація."
menu:
  main:
    name: "Інструменти"
    weight: 80
---

<section id="tools">
<h2>Інструменти</h2>
<p>PgArachne підтримується набором браузерних інструментів, які спрощують розробку, тестування та дослідження API. Усі інструменти — це окремі HTML-файли — без кроку збірки, без залежностей.</p>

<div class="tools-grid">
<div class="card">
<h3>PgArachne Explorer</h3>
<p>Повнофункціональний веб-інтерфейс для перегляду вашого API, тестування функцій та перегляду автоматично згенерованої документації. Підтримує як прямі облікові дані (HTTP Basic Auth), так і автентифікацію через Bearer-токен.</p>
<p><a href="api-explorer/" class="btn stretched-link">Дізнатися більше про Explorer</a></p>
</div>

<div class="card">
<h3>SSE Tester</h3>
<p>Підпишіться на один або декілька каналів PostgreSQL NOTIFY через живе з'єднання Server-Sent Events. Підтримує всі три методи автентифікації та відображає JSON-події з підсвічуванням синтаксису.</p>
<p><a href="sse-tester/" class="btn stretched-link">Дізнатися більше про SSE Tester</a></p>
</div>

<div class="card">
<h3>JWT Getter</h3>
<p>Обміняйте ім'я користувача та пароль PostgreSQL на короткостроковий JWT за допомогою endpoint-а <code>/token</code> (облікові дані HTTP Basic). Показує декодований payload і час завершення дії — корисно для налагодження потоків на основі токенів.</p>
<p><a href="get-jwt/" class="btn stretched-link">Дізнатися більше про JWT Getter</a></p>
</div>

<div class="card">
<h3>JWT Signer</h3>
<p>Генеруйте JWT локально в браузері, вводячи <code>JWT_SECRET</code> вручну — без бази даних і сервера. Задайте роль, базу, строк дії та додаткові claims, навіть уже прострочений токен для тестів. Містить усі попередження щодо безпеки.</p>
<p><a href="jwt-signer/" class="btn stretched-link">Докладніше про JWT Signer</a></p>
</div>

<div class="card">
<h3>PgArachne Toolbar (macOS)</h3>
<p>Рідний застосунок для macOS, що живе у вашій панелі меню. Керуйте кількома екземплярами PgArachne, переглядайте живі логи та метрики одним клацанням.</p>
<p><a href="macos-toolbar/" class="btn stretched-link">Дослідити функції Toolbar</a></p>
</div>
</div>
</section>
