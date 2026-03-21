---
title: "PgArachne Explorer"
description: "PgArachne Explorer - PgArachne"
---

<section id="explorer">
<h2>PgArachne Explorer</h2>
<p><strong>Explorer</strong> je webové rozhraní přiložené ve složce <code>tools/pgarachne-explorer</code>. Nejedná se
		jen o dokumentaci; je to plně funkční <strong>demo aplikace</strong> postavená na HTML/JS,
		která komunikuje s databází výhradně prostřednictvím PgArachne. Online verzi můžete vyzkoušet na adrese 
<a href="https://explorer.pgarachne.com" target="_blank">explorer.pgarachne.com</a>.</p>

<p><strong>Co umí?</strong></p>
<ul>
<li><strong>Inspekce API:</strong> Čte funkci <code>capabilities</code> a zobrazuje všechny dostupné
			endpointy a jejich parametry.</li>
<li><strong>Živé testování:</strong> Funkce můžete spouštět přímo z prohlížeče.</li>
<li><strong>Automatická dokumentace:</strong> Vykresluje SQL komentáře (včetně metadat <code>--- PARAMS ---</code>)
			do čitelné dokumentace.</li>
<li><strong>Moderní vzhled:</strong> Podporuje tmavý i světlý režim a lze jej nainstalovat jako <strong>PWA aplikaci</strong> na mobilní zařízení.</li>
</ul>

<p><strong>Parametry v URL:</strong></p>
<p>Údaje pro připojení můžete předvyplnit pomocí parametru <code>?url=</code>, například: 
<code>?url=http://localhost:8080/db/moje_databaze</code>. Explorer automaticky rozdělí URL na API Host, Prefix a jméno databáze.</p>

<div class="tip">
<strong>Jak jej povolit:</strong> Nastavte proměnnou prostředí <code>STATIC_FILES_PATH</code> tak, aby směřovala na
		složku <code>tools/pgarachne-explorer</code> na vašem disku. Poté navštivte <code>http://localhost:8080</code>.
</div>
</section>
