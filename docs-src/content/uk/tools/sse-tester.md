---
title: "SSE Tester"
description: "PgArachne SSE Tester - Documentation"
---

<section id="sse-tester">
<h2>SSE Tester</h2>
<p><strong>SSE Tester</strong> — це однофайловий браузерний інструмент, розташований у <code>tools/sse-tester</code>. Він дозволяє підписатися на будь-яку кількість каналів PostgreSQL <code>NOTIFY</code> через живе з'єднання <a href="../../real-time-notifications/">Server-Sent Events</a> і спостерігати за вхідними подіями в реальному часі.</p>
<p>Онлайн-версія: <a href="https://sse-tester.pgarachne.com" target="_blank" rel="noopener">sse-tester.pgarachne.com</a></p>

<figure class="tool-screenshot">
<img src="/assets/pgarachne-tool-sse-tester.webp" alt="SSE Tester" loading="lazy">
</figure>


<h3>Можливості</h3>
<ul>
<li><strong>Підписки на кілька каналів</strong> — додавайте або видаляйте канали у вигляді тегів-«пігулок» перед підключенням. Усі канали передаються як параметр запиту <code>?channels=</code>, розділені комами.</li>
<li><strong>Усі методи автентифікації</strong> — вкладка <em>Password</em> використовує HTTP Basic Auth (прямі облікові дані, JWT не потрібен); вкладка <em>Bearer Token</em> приймає JWT або API-токен.</li>
<li><strong>Живий потік</strong> — використовує Fetch Streams API з <code>AbortController</code> для коректного відключення. Рядки SSE розбираються вручну, тож поля <code>event:</code>, <code>data:</code> та heartbeat (<code>:</code>) обробляються правильно.</li>
<li><strong>Підсвічування синтаксису JSON</strong> — вміст даних, що є валідним JSON, форматується з кольоровим виділенням.</li>
<li><strong>Збереження налаштувань</strong> — URL API, префікс, база даних, дані входу та список каналів зберігаються в <code>localStorage</code>.</li>
</ul>

<h3>Автентифікація</h3>
<p>Інструмент перевіряє облікові дані перед тим, як показати панель підписки. Натискання <strong>Verify &amp; Continue</strong> викликає <code>capabilities</code> з обраним заголовком — якщо PgArachne повертає успішну відповідь, панель підписки розблоковується.</p>

<ul>
<li><strong>Вкладка Password</strong> — надсилає <code>Authorization: Basic &lt;base64(user:pass)&gt;</code>. Користувач бази даних повинен мати <code>GRANT EXECUTE</code> на цільові функції та доступ до ендпоінту SSE; <code>GRANT &lt;role&gt; TO pgarachne</code> не потрібен.</li>
<li><strong>Вкладка Bearer Token</strong> — надсилає <code>Authorization: Bearer &lt;token&gt;</code>. Вставте JWT (отриманий через <a href="../get-jwt/">JWT Getter</a>) або довгостроковий API-токен.</li>
</ul>

<div class="tip">
<strong>Як увімкнути:</strong> Встановіть <code>STATIC_FILES_PATH</code> на <code>tools/sse-tester</code> і відвідайте <code>http://localhost:8080</code>. Або відкрийте <code>index.html</code> безпосередньо в браузері — усі функції працюють без локального сервера.
</div>
</section>
