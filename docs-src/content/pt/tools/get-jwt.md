---
title: "JWT Getter"
description: "PgArachne JWT Getter - Documentação"
---

<section id="get-jwt">
<h2>JWT Getter</h2>
<p>O <strong>JWT Getter</strong> é uma ferramenta minimalista para navegador localizada em <code>tools/get-jwt</code>. Troca um nome de usuário e senha do PostgreSQL por um JWT de curta duração chamando o método JSON-RPC <code>get_jwt</code> — e exibe o payload decodificado e o tempo de expiração.</p>

<h3>Quando usar</h3>
<ul>
<li>Para colar um token na aba Bearer Token do <a href="../api-explorer/">Explorer</a> ou do <a href="../sse-tester/">SSE Tester</a>.</li>
<li>Para testar a expiração de um JWT ou os claims <code>db_role</code>/<code>db_name</code> na sua aplicação.</li>
<li>Para verificar que o usuário do sistema <code>pgarachne</code> consegue trocar para um determinado papel (<code>GRANT &lt;papel&gt; TO pgarachne</code> deve existir).</li>
</ul>

<div class="tip">
<strong>Nota:</strong> <code>get_jwt</code> requer <code>GRANT &lt;papel&gt; TO pgarachne</code>. Para evitar esse grant, use credenciais diretas (HTTP Basic Auth) no <a href="../api-explorer/">Explorer</a> ou no <a href="../sse-tester/">SSE Tester</a>.
</div>

<h3>Funcionalidades</h3>
<ul>
<li>Preencha URL, prefixo, banco de dados, nome de usuário e senha — pressione <kbd>Enter</kbd> ou clique em <strong>Get JWT</strong>.</li>
<li>O token gerado é exibido e pode ser copiado com um clique.</li>
<li>O payload do JWT é decodificado em Base64 e exibido com realce de sintaxe JSON.</li>
<li>Um badge mostra o tempo de vida restante (ou marca o token como expirado).</li>
<li>As configurações de conexão e o nome de usuário são salvos no <code>localStorage</code>.</li>
</ul>

<h3>Como habilitar</h3>
<p>Defina <code>STATIC_FILES_PATH</code> como <code>tools/get-jwt</code> e visite <code>http://localhost:8080</code>, ou abra <code>index.html</code> diretamente no navegador.</p>
</section>
