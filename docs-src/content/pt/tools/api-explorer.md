---
title: "Explorador PgArachne"
description: "Explorador PgArachne - PgArachne"
---

<section id="explorer">
<h2>Explorador PgArachne</h2>
<p>O <strong>Explorer</strong> é uma interface web poderosa incluída no diretório <code>tools/pgarachne-explorer</code>. Não é apenas
		documentação; é uma <strong>aplicação de demonstração</strong> totalmente funcional construída usando
		HTML/JS que se comunica com o banco de dados exclusivamente via PgArachne. Você também pode usar a versão hospedada em 
<a href="https://explorer.pgarachne.com" target="_blank">explorer.pgarachne.com</a>.</p>

<p><strong>O que ele pode fazer?</strong></p>
<ul>
<li><strong>Inspecionar API:</strong> Ele lê a função <code>capabilities</code> para exibir todos os endpoints
			disponíveis e seus parâmetros.</li>
<li><strong>Testes ao Vivo:</strong> Você pode executar funções diretamente do navegador.</li>
<li><strong>Autodocumentação:</strong> Ele renderiza os comentários SQL (incluindo metadados <code>--- PARAMS
			---</code>) em documentação legível.</li>
<li><strong>Interface moderna:</strong> Oferece suporte para os modos Escuro/Claro e pode ser instalada como um <strong>PWA</strong> em dispositivos móveis.</li>
<li><strong>Autenticação:</strong> A aba <em>Password</em> usa HTTP Basic Auth para conectar diretamente como o usuário do banco de dados — sem <code>GRANT … TO pgarachne</code>. A aba <em>API Token</em> aceita um JWT ou token de API de longa duração como <code>Bearer</code>.</li>
</ul>

<p><strong>Parâmetros URL:</strong></p>
<p>Você pode pré-preencher as configurações de conexão usando o parâmetro <code>?url=</code>, por exemplo: 
<code>?url=http://localhost:8080/db/meu_banco_de_dados</code>. O Explorer dividirá automaticamente a URL nos campos Host da API, Prefixo e Banco de dados.</p>

<div class="tip">
<strong>Como habilitá-lo:</strong> Defina a variável de ambiente <code>STATIC_FILES_PATH</code> para apontar para a
		pasta <code>tools/pgarachne-explorer</code> no seu disco. Em seguida, visite <code>http://localhost:8080</code>.
</div>
</section>
