# OZmap User Manager

Aplicação web em Go para importar usuários via XLSX e criá-los individualmente na API do OZmap, com associação de roles amigáveis, histórico, autenticação administrativa e controles de segurança.

O histórico de importações é paginado e pode ser filtrado por data.

A prévia da planilha também é paginada, exibindo 20 usuários por página; os totais de encontrados, válidos e com erro permanecem visíveis acima da tabela.

Durante a criação, a interface acompanha o progresso em segundo plano, mostrando usuários processados, criados e com erro. A lista de resultados final é paginada em grupos de 20.

## Integração OZmap

A implementação segue os exemplos concretos fornecidos na especificação: `POST /api/v2/auth/token`, body com `clientId`/`clientSecret`, resposta `accessToken`, e `POST /api/v2/users` com `resources`, `username`, `password`, `email`, `role` e demais campos opcionais. O token é mantido somente em memória e enviado como `Authorization: Bearer <token>` conforme o contrato indicado na especificação.

O padrão é enviar a senha original (`OZMAP_PASSWORD_MODE=plain`), removendo apenas quebras de linha e espaços nas extremidades que podem ter sido copiados do Excel. Configure `OZMAP_PASSWORD_MODE=sha256` somente se sua instalação OZmap exigir explicitamente esse formato.

Durante a criação de usuários, o aplicativo lista os projetos disponíveis no OZmap (com paginação) e envia a associação de cada usuário a todos eles, usando em cada projeto a role selecionada na planilha. Opcionalmente, a coluna `projectGroups` permite informar nomes cadastrados na página **Grupos de projetos**; o usuário é associado aos IDs correspondentes e a listagem de projetos não é feita para esses usuários. Linhas sem `projectGroups` mantêm o comportamento original.

## Requisitos

- Docker + Docker Compose
- ou Go 1.27.1 para execução local

## Primeiro uso com Docker

1. Gere um hash Argon2id para a senha administrativa. Uma forma simples é criar um pequeno utilitário usando `internal/security.HashAdmin`, ou usar o comando auxiliar documentado abaixo.
2. Copie `.env.example` para `.env`.
3. Defina `POSTGRES_PASSWORD`, `DATABASE_URL`, `OZMAP_BASE_URL`, `OZMAP_CLIENT_ID`, `OZMAP_CLIENT_SECRET`, `ADMIN_USERNAME`, `ADMIN_PASSWORD_HASH` e `SESSION_SECRET`.
4. Execute:

```bash
docker compose up -d --build
```

5. Acesse `http://localhost:8080`.

### Gerar hash administrativo

Crie temporariamente um programa Go que chame `security.HashAdmin("sua-senha")`, ou use uma ferramenta externa compatível com Argon2id. O valor final deve ser colocado em `ADMIN_PASSWORD_HASH`; a senha em texto puro nunca deve entrar no `.env` versionado.

## Estrutura

```text
cmd/server
internal/auth
internal/config
internal/excel
internal/handlers
internal/middleware
internal/models
internal/ozmap
internal/repositories
internal/security
internal/services
migrations
web/templates
web/static
```

## Excel

O endpoint `GET /download/template` gera `modelo_usuarios_ozmap.xlsx` com abas `USUARIOS` e `INSTRUCOES`. Colunas: `email`, `password`, `role`, `resources` e `projectGroups` (opcional). Resources e nomes de grupos aceitam itens separados por vírgula. Cadastre primeiro os nomes e IDs em **Grupos de projetos**. Se `projectGroups` for informado, usa esses grupos em vez da associação automática a todos os projetos. Planilhas existentes com as quatro primeiras colunas continuam funcionando como antes.

O username enviado ao OZmap é o e-mail completo informado na planilha.

As senhas são mantidas apenas em memória durante o fluxo de pré-visualização/criação e não entram no banco, histórico, logs ou respostas do backend.
Ao importar, quebras de linha são removidas de toda a senha, e espaços nas extremidades são removidos para evitar caracteres acidentais copiados do Excel; espaços internos e os demais caracteres especiais são preservados.

## Segurança implementada

- autenticação administrativa por sessão server-side;
- Argon2id para senha administrativa;
- cookie HttpOnly/SameSite;
- CSRF em operações mutáveis;
- CSP, nosniff, frame-ancestors/X-Frame-Options, Referrer-Policy e no-store;
- rate limiting básico para endpoints sensíveis;
- limite de tamanho e quantidade de linhas do XLSX;
- parsing real do XLSX com `excelize`;
- queries parametrizadas PostgreSQL;
- timeouts HTTP e `context.Context`;
- cliente OZmap isolado;
- tratamento público de erros sem credenciais/stack traces;
- container non-root, read-only e sem capabilities;
- token OZmap somente em memória;
- sem persistência de senhas de usuários.

## Desenvolvimento

```bash
cp .env.example .env
go mod tidy
go test ./...
go vet ./...
go run ./cmd/server
```

`staticcheck ./...` pode ser usado quando instalado.

## HTTPS

O aplicativo entrega HTTP e deve ficar atrás de um reverse proxy/TLS em produção. Configure o proxy para HTTPS, HSTS e limites adicionais de rede conforme sua infraestrutura.
