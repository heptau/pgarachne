---
title: "Ferramentas"
description: "Ferramentas para PgArachne - Documentação."
menu:
  main:
    name: "Ferramentas"
    weight: 70
---

<section id="tools">
<h2>Ferramentas</h2>
<p>O PgArachne é acompanhado de um conjunto de ferramentas para navegador que simplificam o desenvolvimento, os testes e a exploração da API. Cada ferramenta é um único arquivo HTML — sem etapas de compilação nem dependências.</p>

<div class="tools-grid">
<div class="card">
<h3>PgArachne Explorer</h3>
<p>Uma interface web completa para explorar sua API, testar funções e visualizar a documentação gerada automaticamente. Suporta credenciais diretas (HTTP Basic Auth) e autenticação por Bearer token.</p>
<p><a href="api-explorer/" class="btn">Saiba mais sobre o Explorer</a></p>
</div>

<div class="card">
<h3>SSE Tester</h3>
<p>Assine um ou mais canais PostgreSQL NOTIFY via uma conexão Server-Sent Events ao vivo. Compatível com os três métodos de autenticação e exibe eventos JSON com realce de sintaxe.</p>
<p><a href="sse-tester/" class="btn">Saiba mais sobre o SSE Tester</a></p>
</div>

<div class="card">
<h3>JWT Getter</h3>
<p>Troque um nome de usuário e senha do PostgreSQL por um JWT de curta duração através do método <code>get_jwt</code>. Exibe o payload decodificado e o tempo de expiração.</p>
<p><a href="get-jwt/" class="btn">Saiba mais sobre o JWT Getter</a></p>
</div>

<div class="card">
<h3>PgArachne Toolbar (macOS)</h3>
<p>Um aplicativo nativo para macOS na barra de menus. Gerencie múltiplas instâncias do PgArachne, visualize logs em tempo real e monitore métricas com um único clique.</p>
<p><a href="macos-toolbar/" class="btn">Explorar recursos do Toolbar</a></p>
</div>
</div>
</section>
