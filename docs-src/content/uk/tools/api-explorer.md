---
title: "PgArachne Explorer"
description: "PgArachne Explorer - PgArachne"
---

<section id="explorer">
<h2>PgArachne Explorer</h2>
<p><strong>Explorer</strong> — це потужний веб-інтерфейс, що входить до каталогу <code>tools/pgarachne-explorer</code>. Це не
		просто інструмент документації — це повністю функціональний <strong>демонстраційний застосунок</strong>, побудований на HTML/JS,
		який спілкується з базою даних виключно через PgArachne. Хостована версія також доступна на
<a href="https://explorer.pgarachne.com" target="_blank">explorer.pgarachne.com</a>.</p>

<p><strong>Що він уміє?</strong></p>
<ul>
<li><strong>Огляд API:</strong> Він читає функцію <code>capabilities</code>, щоб відобразити всі доступні
			ендпоінти та їхні параметри.</li>
<li><strong>Живе тестування:</strong> Ви можете виконувати функції безпосередньо з браузера.</li>
<li><strong>Автоматична документація:</strong> Він перетворює SQL-коментарі (включно з метаданими <code>--- PARAMS ---</code>)
			на читабельну документацію.</li>
<li><strong>Сучасний інтерфейс:</strong> Підтримує темний/світлий режим і може бути встановлений як <strong>PWA</strong> на мобільних пристроях.</li>
<li><strong>Автентифікація:</strong> Вкладка <em>Password</em> використовує HTTP Basic Auth для прямого підключення як користувач бази даних — <code>GRANT &hellip; TO pgarachne</code> не потрібен. Вкладка <em>API Token</em> приймає JWT або довгостроковий API-токен, надісланий як <code>Bearer</code>.</li>
</ul>

<p><strong>Параметри URL:</strong></p>
<p>Ви можете попередньо заповнити налаштування з'єднання за допомогою параметра <code>?url=</code>, наприклад: 
<code>?url=http://localhost:8080/db/my_database</code>. Це автоматично розділить URL на поля API Host, Prefix і Database.</p>

<div class="tip">
<strong>Як увімкнути:</strong> Встановіть змінну середовища <code>STATIC_FILES_PATH</code>, щоб вона вказувала на
каталог <code>tools/pgarachne-explorer</code> на вашому диску. Потім відвідайте <code>http://localhost:8080</code>.
</div>
</section>
