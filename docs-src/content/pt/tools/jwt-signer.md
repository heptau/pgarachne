---
title: "JWT Signer"
description: "PgArachne JWT Signer"
---

<section id="jwt-signer">
<h2>JWT Signer</h2>
<p>O <strong>JWT Signer</strong> é uma ferramenta de navegador num único ficheiro, em <code>tools/jwt-signer</code>. Ao contrário do <a href="../get-jwt/">JWT Getter</a>, nunca contacta o PgArachne nem a base de dados: introduz o <code>JWT_SECRET</code> manualmente e o token é assinado (HS256) diretamente no navegador.</p>
<p>Versão online: <a href="https://jwt-signer.pgarachne.com" target="_blank" rel="noopener">jwt-signer.pgarachne.com</a></p>

<figure class="tool-screenshot">
<img src="/assets/pgarachne-tool-jwt-signer.webp" alt="JWT Signer" loading="lazy">
</figure>

<div class="warning">
<strong>Trate o seu segredo JWT com cuidado</strong>
<ul>
<li>O <code>JWT_SECRET</code> é a chave mestra da sua API: quem o conhecer pode assinar um token para <em>qualquer</em> função da base de dados, incluindo superutilizadores.</li>
<li><strong>Nunca cole um segredo de produção numa página web que não controle totalmente.</strong> Use a versão alojada apenas com segredos de desenvolvimento/teste. Em produção, guarde o <code>index.html</code> e abra-o localmente (funciona offline) ou assine os tokens no servidor.</li>
<li>Tudo corre no seu navegador. A página proíbe qualquer acesso à rede (Content Security Policy <code>connect-src 'none'</code>), não carrega recursos externos e <strong>nunca guarda o segredo</strong> — nem em <code>localStorage</code>, nem em cookies, nem no URL. Só são lembrados função, base de dados, validade, issuer e audience.</li>
<li>O PgArachne aceita um token assinado aqui exatamente como um emitido por <code>/token</code>. Use validades curtas e nunca partilhe tokens assinados com um segredo real.</li>
</ul>
</div>

<h3>Quando usar</h3>
<ul>
<li>Desenvolvimento e testes quando não quer (ou não pode) autenticar-se com a palavra-passe da base de dados.</li>
<li>Verificar como o seu cliente lida com a expiração: uma validade negativa produz um token já expirado.</li>
<li>Experimentar <code>iss</code> / <code>aud</code> e claims adicionais num servidor que os exija.</li>
<li>Gerar com um clique um segredo aleatório de 256 bits para um novo <code>JWT_SECRET</code>.</li>
</ul>

<div class="tip">
<strong>Como ativar:</strong> Defina <code>STATIC_FILES_PATH</code> como <code>tools/jwt-signer</code> e visite <code>http://localhost:8080</code>, ou abra o <code>index.html</code> diretamente no navegador — funciona sem servidor.
</div>
</section>
