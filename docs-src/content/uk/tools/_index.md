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
<p><a href="api-explorer/" class="btn">Дізнатися більше про Explorer</a></p>
</div>

<div class="card">
<h3>SSE Tester</h3>
<p>Підпишіться на один або декілька каналів PostgreSQL NOTIFY через живе з'єднання Server-Sent Events. Підтримує всі три методи автентифікації та відображає JSON-події з підсвічуванням синтаксису.</p>
<p><a href="sse-tester/" class="btn">Дізнатися більше про SSE Tester</a></p>
</div>

<div class="card">
<h3>JWT Getter</h3>
<p>Обміняйте ім'я користувача та пароль PostgreSQL на короткостроковий JWT за допомогою методу <code>get_jwt</code>. Показує декодований payload і час завершення дії — корисно для налагодження потоків на основі токенів.</p>
<p><a href="get-jwt/" class="btn">Дізнатися більше про JWT Getter</a></p>
</div>

<div class="card">
<h3>PgArachne Toolbar (macOS)</h3>
<p>Рідний застосунок для macOS, що живе у вашій панелі меню. Керуйте кількома екземплярами PgArachne, переглядайте живі логи та метрики одним клацанням.</p>
<p><a href="macos-toolbar/" class="btn">Дослідити функції Toolbar</a></p>
</div>
</div>
</section>
