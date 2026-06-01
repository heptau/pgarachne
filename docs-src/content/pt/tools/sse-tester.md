---
title: "SSE Tester"
description: "PgArachne SSE Tester - Documentação"
---

<section id="sse-tester">
<h2>SSE Tester</h2>
<p>O <strong>SSE Tester</strong> é uma ferramenta de arquivo único para navegador localizada em <code>tools/test-sse</code>. Permite subscrever qualquer número de canais PostgreSQL <code>NOTIFY</code> via uma conexão <a href="../../real-time-notifications/">Server-Sent Events</a> ao vivo e observar os eventos em tempo real.</p>

<h3>Funcionalidades</h3>
<ul>
<li><strong>Múltiplos canais</strong> — os canais são adicionados como tags e passados como parâmetro <code>?channels=</code>.</li>
<li><strong>Todos os métodos de autenticação</strong> — a aba <em>Password</em> usa HTTP Basic Auth (sem JWT); a aba <em>Bearer Token</em> aceita um JWT ou token de API.</li>
<li><strong>Stream ao vivo</strong> — utiliza a Fetch Streams API com <code>AbortController</code> para desconexão limpa.</li>
<li><strong>Realce de sintaxe JSON</strong> — os payloads JSON são exibidos com cores.</li>
<li><strong>Configurações persistentes</strong> — URL, prefixo, banco de dados, nome de usuário e lista de canais são salvos no <code>localStorage</code>.</li>
</ul>

<h3>Autenticação</h3>
<ul>
<li><strong>Aba Password</strong> — envia <code>Authorization: Basic &lt;base64(usuário:senha)&gt;</code>. Não requer <code>GRANT &lt;papel&gt; TO pgarachne</code>.</li>
<li><strong>Aba Bearer Token</strong> — envia <code>Authorization: Bearer &lt;token&gt;</code>. Cole um JWT (obtido pelo <a href="../get-jwt/">JWT Getter</a>) ou um token de API de longa duração.</li>
</ul>

<div class="tip">
<strong>Como habilitar:</strong> Defina <code>STATIC_FILES_PATH</code> como <code>tools/test-sse</code> e visite <code>http://localhost:8080</code>. Ou abra <code>index.html</code> diretamente no navegador.
</div>
</section>
