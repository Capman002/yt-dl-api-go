# TODO: yt-dl-api-go

> API Go open source para download de vídeos via yt-dlp.  
> Projetada para ser **fácil de clonar, configurar e usar** em qualquer site.

---

## 🎯 Objetivo do Projeto

Uma API REST pronta para produção que qualquer desenvolvedor pode:

1. **Clonar** → `git clone https://github.com/seu-user/yt-dl-api-go`
2. **Configurar** → Editar `.env` com suas credenciais
3. **Rodar** → `docker compose up` ou `go run ./cmd/api`
4. **Integrar** → Chamar a API do seu frontend

### Princípios

- **Open Source Safe**: Segurança não depende de código secreto
- **Zero Config Deploy**: Docker Compose funciona out-of-the-box
- **Custo Baixíssimo**: Cloudflare R2 (egress grátis) + VPS barato
- **Documentação Clara**: README, exemplos e API docs inclusos

---

## 🏗️ Fase 1: Fundação

### Estrutura do Projeto

```text
yt-dl-api-go/
├── cmd/api/main.go
├── internal/
│   ├── config/config.go
│   ├── domain/entities.go
│   ├── service/
│   │   ├── downloader/ytdlp.go
│   │   └── queue/dispatcher.go
│   ├── infra/
│   │   ├── sqlite/repository.go
│   │   ├── r2/client.go
│   │   └── fs/cleanup.go
│   └── transport/
│       ├── http/handlers.go
│       ├── http/router.go
│       └── http/middleware/
│           ├── validator.go
│           ├── turnstile.go
│           └── ratelimit.go
├── pkg/safeclient/client.go
│
├── examples/                    # 🆕 Exemplos de integração
│   ├── html-vanilla/            # HTML + JS puro
│   ├── react/                   # React/Next.js
│   └── astro/                   # Astro
│
├── docs/                        # 🆕 Documentação
│   ├── API.md                   # Documentação da API
│   ├── SECURITY.md              # Explicação das proteções
│   └── DEPLOY.md                # Guias de deploy por plataforma
│
├── go.mod
├── go.sum
├── Dockerfile
├── docker-compose.yml           # 🆕 Deploy com 1 comando
├── Makefile                     # 🆕 Comandos úteis
├── .env.example                 # Template de configuração
├── .gitignore
├── LICENSE                      # MIT
└── README.md                    # 🆕 Documentação principal
```

### Tarefas

- [ ] Criar estrutura de diretórios
- [ ] Inicializar `go.mod` com dependências:
  - `github.com/go-chi/chi/v5`
  - `nhooyr.io/websocket`
  - `modernc.org/sqlite`
  - `golang.org/x/time/rate`
  - `github.com/joho/godotenv`
  - `github.com/aws/aws-sdk-go-v2` (para R2)
- [ ] Criar `config.go` para carregar variáveis de ambiente
- [ ] Criar `.env.example` com template das variáveis

---

## 🛡️ Fase 2: Segurança

### 2.1 Validação de URL

**Arquivo:** `internal/transport/http/middleware/validator.go`

- [ ] Implementar allowlist de domínios:
  ```go
  youtube.com, youtu.be, twitter.com, x.com,
  tiktok.com, instagram.com, facebook.com, vimeo.com
  ```
- [ ] Validar scheme (apenas HTTPS)
- [ ] Rejeitar URLs com userinfo (`user:pass@host`)
- [ ] Criar função `ValidateURL(rawURL string) error`

### 2.2 Prevenção SSRF

**Arquivo:** `pkg/safeclient/client.go`

- [ ] Implementar `isForbiddenIP(ip net.IP) bool`
  - Bloquear: loopback, private, link-local, multicast
- [ ] Implementar `net.Dialer` com `Control` function
  - Valida IP **no momento da conexão** (anti DNS rebinding)
- [ ] Exportar `NewSafeHTTPClient() *http.Client`

### 2.3 Wrapper yt-dlp

**Arquivo:** `internal/service/downloader/ytdlp.go`

- [ ] Implementar struct `Downloader`
- [ ] Implementar `Download(ctx, url, outputDir) (VideoInfo, error)`
- [ ] Flags de segurança obrigatórias:
  ```go
  "--no-playlist"
  "--max-filesize", "500M"
  "--match-filter", "duration<1800"
  "--newline"
  "--print-json" // para metadados
  ```
- [ ] Usar `exec.CommandContext` com argumentos como slice
- [ ] Parsear stdout para capturar progresso
- [ ] Implementar timeout de 10 minutos por download

---

## ⚖️ Fase 3: Rate Limiting

### 3.1 Middleware Turnstile

**Arquivo:** `internal/transport/http/middleware/turnstile.go`

- [ ] Implementar `VerifyTurnstile(token, secret, remoteIP) (bool, error)`
- [ ] POST para `https://challenges.cloudflare.com/turnstile/v0/siteverify`
- [ ] Implementar middleware `TurnstileMiddleware(next http.Handler)`
- [ ] Extrair token do header `X-Turnstile-Token` ou body

### 3.2 Rate Limiter por IP

**Arquivo:** `internal/transport/http/middleware/ratelimit.go`

- [ ] Implementar mapa de visitors com `sync.Mutex`
- [ ] Usar `golang.org/x/time/rate.Limiter`
- [ ] Configuração: 5 requests/minuto, burst 2
- [ ] Usar header `CF-Connecting-IP` para IP real
- [ ] Implementar goroutine de cleanup (limpar IPs antigos a cada 10 min)
- [ ] Implementar middleware `RateLimitMiddleware(next http.Handler)`

### 3.3 Worker Pool

**Arquivo:** `internal/service/queue/dispatcher.go`

- [ ] Implementar `JobChannel` com buffer de 10
- [ ] Implementar `StartWorkers(ctx, n int)` - inicia N workers
- [ ] Implementar `Enqueue(job Job) error` - retorna erro se fila cheia
- [ ] Workers consomem do canal e chamam `Downloader.Download`
- [ ] Implementar callback para notificar progresso via WebSocket

---

## 💰 Fase 4: Cloudflare R2

### 4.1 Cliente R2

**Arquivo:** `internal/infra/r2/client.go`

- [ ] Configurar cliente AWS SDK v2 com endpoint customizado do R2
- [ ] Implementar `Upload(ctx, filePath, key) error`
- [ ] Implementar `GeneratePresignedURL(key, expiry) (string, error)`
  - Expiração padrão: 15 minutos
- [ ] Implementar `Delete(ctx, key) error`
- [ ] Implementar `ListOlderThan(ctx, age time.Duration) ([]string, error)`

### 4.2 Limpeza Automática

**Arquivo:** `internal/infra/fs/cleanup.go`

- [ ] Implementar `StartLocalCleanup(ctx, dir, maxAge, interval)`
  - Limpa arquivos locais em `/tmp` a cada 5 min
- [ ] Implementar `StartR2Cleanup(ctx, r2Client, maxAge, interval)`
  - Limpa arquivos no R2 a cada 30 min
  - Deleta arquivos com mais de 1 hora

---

## 🔌 Fase 5: API HTTP

### 5.1 Entidades

**Arquivo:** `internal/domain/entities.go`

- [ ] Struct `Job`:
  ```go
  ID, URL, Title, Status, FilePath,
  DownloadURL, Progress, CreatedAt, CompletedAt
  ```
- [ ] Enum `JobStatus`: pending, processing, done, error
- [ ] Struct `VideoInfo`: Title, Duration, Thumbnail, Filesize

### 5.2 Handlers

**Arquivo:** `internal/transport/http/handlers.go`

- [ ] `POST /api/download`
  - Body: `{ "url": "...", "turnstile": "..." }`
  - Response: `{ "job_id": "uuid" }`
  - Valida URL, verifica Turnstile, enfileira job
- [ ] `GET /api/status/:job_id`
  - Response: `{ "status": "...", "progress": 45, "download_url": "..." }`
- [ ] `GET /api/health`
  - Response: `{ "status": "ok", "queue_size": 3 }`

### 5.3 Router

**Arquivo:** `internal/transport/http/router.go`

- [ ] Configurar chi router
- [ ] Aplicar middlewares na ordem:
  1. Logger (slog)
  2. Recoverer
  3. CORS
  4. RateLimit
  5. Turnstile (apenas em `/api/download`)
- [ ] Configurar CORS para `ALLOWED_ORIGINS`

---

## 📦 Fase 6: Persistência

### 6.1 SQLite

**Arquivo:** `internal/infra/sqlite/repository.go`

- [ ] Criar schema:
  ```sql
  CREATE TABLE jobs (
      id TEXT PRIMARY KEY,
      url TEXT NOT NULL,
      title TEXT,
      status TEXT DEFAULT 'pending',
      file_key TEXT,
      download_url TEXT,
      progress INTEGER DEFAULT 0,
      error TEXT,
      created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
      completed_at DATETIME
  );
  CREATE INDEX idx_jobs_status ON jobs(status);
  CREATE INDEX idx_jobs_created ON jobs(created_at);
  ```
- [ ] Implementar `Create(job) error`
- [ ] Implementar `GetByID(id) (Job, error)`
- [ ] Implementar `Update(job) error`
- [ ] Implementar `ListPending() ([]Job, error)`

---

## ⚡ Fase 7: Performance

### 7.1 Response Imediata (Non-Blocking)

**Já implementado na arquitetura**, mas garantir:

- [ ] `POST /api/download` retorna em < 100ms
- [ ] Apenas valida e enfileira, nunca bloqueia no download
- [ ] Job ID retornado imediatamente para polling/WebSocket

### 7.2 HTTP Server Otimizado

**Arquivo:** `cmd/api/main.go`

- [ ] Configurar timeouts do servidor:
  ```go
  server := &http.Server{
      Addr:         ":8080",
      Handler:      router,
      ReadTimeout:  5 * time.Second,
      WriteTimeout: 10 * time.Second,
      IdleTimeout:  120 * time.Second,
  }
  ```
- [ ] Habilitar HTTP/2 (automático com TLS)
- [ ] Graceful shutdown com `signal.NotifyContext`

### 7.3 Compressão Gzip

**Arquivo:** `internal/transport/http/middleware/compress.go`

- [ ] Implementar middleware de compressão gzip
- [ ] Usar `github.com/go-chi/chi/v5/middleware` (já tem Compress)
- [ ] Comprimir respostas JSON > 1KB

### 7.4 Connection Pooling

**Arquivo:** `pkg/safeclient/client.go`

- [ ] Configurar `http.Transport` otimizado:
  ```go
  transport := &http.Transport{
      MaxIdleConns:        100,
      MaxIdleConnsPerHost: 10,
      IdleConnTimeout:     90 * time.Second,
  }
  ```
- [ ] Reutilizar cliente HTTP (não criar novo a cada request)

### 7.5 Sync.Pool para Buffers

**Arquivo:** `internal/service/downloader/ytdlp.go`

- [ ] Usar `sync.Pool` para reutilizar buffers de leitura do stdout:
  ```go
  var bufPool = sync.Pool{
      New: func() interface{} {
          return make([]byte, 32*1024) // 32KB buffer
      },
  }
  ```
- [ ] Reduz alocações e pressão no GC

### 7.6 Cache de Metadados (Opcional)

**Arquivo:** `internal/infra/cache/cache.go`

- [ ] Cache in-memory para metadados de vídeo (título, duração)
- [ ] Evita re-executar `yt-dlp --print-json` para mesma URL
- [ ] TTL de 1 hora
- [ ] Usar `sync.Map` ou biblioteca como `github.com/patrickmn/go-cache`

### 7.7 SQLite Otimizado

**Arquivo:** `internal/infra/sqlite/repository.go`

- [ ] Habilitar WAL mode (Write-Ahead Logging):
  ```go
  db.Exec("PRAGMA journal_mode=WAL")
  db.Exec("PRAGMA synchronous=NORMAL")
  db.Exec("PRAGMA cache_size=10000")
  ```
- [ ] Connection pool com `SetMaxOpenConns(1)` (SQLite single-writer)
- [ ] Prepared statements para queries frequentes

### 7.8 Logs Estruturados (slog)

**Arquivo:** `pkg/logger/logger.go`

- [ ] Usar `log/slog` nativo do Go (zero allocation em hot path)
- [ ] Nível INFO em produção, DEBUG em dev
- [ ] Output JSON para facilitar parsing
- [ ] Não logar dados sensíveis (URLs completas, IPs em produção)

---

## 🐳 Fase 8: Docker e Containerização

### 8.1 Dockerfile

- [ ] Multi-stage build:
  - Stage 1: Go builder (compilar binário estático com CGO_ENABLED=0)
  - Stage 2: Alpine com yt-dlp e ffmpeg
- [ ] Instalar yt-dlp no container
- [ ] Copiar binário Go
- [ ] Expor porta 8080
- [ ] Criar `.dockerignore`
- [ ] Health check: `HEALTHCHECK CMD wget -qO- http://localhost:8080/api/health`

### 8.2 Docker Compose

**Arquivo:** `docker-compose.yml`

- [ ] Serviço `api` com todas as variáveis de ambiente
- [ ] Volume para SQLite persistente
- [ ] Volume para arquivos temporários
- [ ] Restart policy: `unless-stopped`
- [ ] Limites de recursos (opcional)

```yaml
# docker-compose.yml
version: "3.8"
services:
  api:
    build: .
    ports:
      - "8080:8080"
    env_file:
      - .env
    volumes:
      - ./data:/app/data # SQLite
      - ./tmp:/app/tmp # Arquivos temporários
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "wget", "-qO-", "http://localhost:8080/api/health"]
      interval: 30s
      timeout: 10s
      retries: 3
```

---

## 📚 Fase 9: Developer Experience (Open Source)

### 9.1 README.md Principal

- [ ] Badges (Go version, License, Docker)
- [ ] Descrição clara do projeto
- [ ] Quick Start em 3 passos
- [ ] Tabela de endpoints da API
- [ ] Requisitos (Go, yt-dlp, ou Docker)
- [ ] Links para docs detalhadas

```markdown
# yt-dl-api-go

🚀 API REST para download de vídeos via yt-dlp

## Quick Start

1. Clone: `git clone https://github.com/seu-user/yt-dl-api-go`
2. Configure: `cp .env.example .env && vim .env`
3. Rode: `docker compose up -d`

## API Endpoints

| Método | Endpoint        | Descrição       |
| ------ | --------------- | --------------- |
| POST   | /api/download   | Inicia download |
| GET    | /api/status/:id | Status do job   |
| GET    | /api/health     | Health check    |
```

### 9.2 Documentação da API

**Arquivo:** `docs/API.md`

- [ ] Descrição detalhada de cada endpoint
- [ ] Exemplos de request/response
- [ ] Códigos de erro
- [ ] Rate limits explicados
- [ ] Exemplos com cURL

### 9.3 Documentação de Segurança

**Arquivo:** `docs/SECURITY.md`

- [ ] Explicar todas as camadas de proteção
- [ ] Como configurar Turnstile
- [ ] Como o Rate Limiting funciona
- [ ] Boas práticas para produção

### 9.4 Guias de Deploy

**Arquivo:** `docs/DEPLOY.md`

- [ ] Deploy no Fly.io
- [ ] Deploy no Railway
- [ ] Deploy no Hetzner/VPS
- [ ] Deploy no Oracle Cloud Free Tier
- [ ] Configuração do Cloudflare (DNS, Proxy)

### 9.5 Makefile

**Arquivo:** `Makefile`

- [ ] `make dev` - Rodar em modo desenvolvimento
- [ ] `make build` - Compilar binário
- [ ] `make test` - Rodar testes
- [ ] `make lint` - Rodar golangci-lint
- [ ] `make docker` - Build da imagem Docker
- [ ] `make up` - docker compose up
- [ ] `make down` - docker compose down

```makefile
.PHONY: dev build test lint docker up down

dev:
	go run ./cmd/api

build:
	CGO_ENABLED=0 go build -o bin/api ./cmd/api

test:
	go test -v ./...

lint:
	golangci-lint run

docker:
	docker build -t yt-dl-api-go .

up:
	docker compose up -d

down:
	docker compose down
```

### 9.6 Exemplos de Integração

**Pasta:** `examples/`

#### HTML + JavaScript Vanilla

- [ ] `examples/html-vanilla/index.html`
- [ ] Formulário simples com Turnstile
- [ ] Fetch para a API
- [ ] Barra de progresso

#### React/Next.js

- [ ] `examples/react/DownloadForm.tsx`
- [ ] Hook `useDownload()`
- [ ] Componente de progresso

#### Astro

- [ ] `examples/astro/DownloadSection.astro`
- [ ] Integração com Turnstile
- [ ] Client-side interactivity

### 9.7 Arquivos de Configuração

- [ ] `.env.example` com todas as variáveis documentadas
- [ ] `.gitignore` completo (data/, tmp/, .env, binários)
- [ ] `LICENSE` (MIT)
- [ ] `.golangci.yml` para linting

---

## 🎯 Ordem de Implementação

1. [ ] **Fase 1** - Estrutura e go.mod
2. [ ] **Fase 2** - Segurança (validator, safeclient, ytdlp wrapper)
3. [ ] **Fase 3** - Rate limiting (turnstile, ratelimit, worker pool)
4. [ ] **Fase 6** - SQLite (para persistir jobs)
5. [ ] **Fase 5** - API HTTP (handlers, router)
6. [ ] **Fase 7** - Performance (otimizações aplicadas durante o código)
7. [ ] **Fase 4** - R2 (upload, presigned URLs, cleanup)
8. [ ] **Fase 8** - Docker e docker-compose
9. [ ] **Fase 9** - Developer Experience (README, docs, exemplos)

---

## 📝 Dependências Finais (go.mod)

```go
require (
    github.com/go-chi/chi/v5 v5.x.x
    github.com/go-chi/cors v1.x.x
    nhooyr.io/websocket v1.x.x
    modernc.org/sqlite v1.x.x
    golang.org/x/time v0.x.x
    github.com/joho/godotenv v1.x.x
    github.com/aws/aws-sdk-go-v2 v1.x.x
    github.com/aws/aws-sdk-go-v2/config v1.x.x
    github.com/aws/aws-sdk-go-v2/service/s3 v1.x.x
    github.com/google/uuid v1.x.x
)
```

---

**Pronto para começar?** Diga "Crie o projeto" e eu implemento tudo! 🚀
