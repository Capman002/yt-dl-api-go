# yt-dl-api-go

[![Go Version](https://img.shields.io/badge/Go-1.23+-00ADD8?style=flat&logo=go)](https://go.dev)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Docker](https://img.shields.io/badge/Docker-Ready-2496ED?style=flat&logo=docker)](Dockerfile)

🚀 **API REST para download de vídeos via yt-dlp**

Uma API open-source, segura e pronta para produção que permite baixar vídeos de YouTube, Twitter, TikTok, Instagram e mais.

## ✨ Características

- 🔒 **Segurança**: Proteção SSRF, validação de URL, rate limiting
- ⚡ **Performance**: Worker pool, respostas non-blocking, SQLite otimizado
- ☁️ **Cloudflare R2**: Upload automático com URLs presigned (egress grátis!)
- 🛡️ **Turnstile**: Proteção anti-bot integrada
- 🐳 **Docker Ready**: Deploy com um único comando
- 📊 **Observabilidade**: Logs estruturados (slog), health checks

## 🚀 Quick Start

### Opção 1: Docker (Recomendado)

```bash
# Clone o repositório
git clone https://github.com/seu-user/yt-dl-api-go
cd yt-dl-api-go

# Configure as variáveis de ambiente
cp .env.example .env
# Edite .env com suas credenciais

# Inicie
docker compose up -d
```

### Opção 2: Go Local

```bash
# Requisitos: Go 1.23+, yt-dlp, ffmpeg

# Clone
git clone https://github.com/seu-user/yt-dl-api-go
cd yt-dl-api-go

# Configure
cp .env.example .env

# Instale dependências e rode
go mod download
go run ./cmd/api
```

## 📡 API Endpoints

| Método | Endpoint               | Descrição              |
| ------ | ---------------------- | ---------------------- |
| `POST` | `/api/download`        | Inicia um download     |
| `GET`  | `/api/status/{job_id}` | Consulta status do job |
| `GET`  | `/api/health`          | Health check           |

### POST /api/download

```bash
curl -X POST http://localhost:8080/api/download \
  -H "Content-Type: application/json" \
  -H "X-Turnstile-Token: your-token" \
  -d '{"url": "https://www.youtube.com/watch?v=dQw4w9WgXcQ"}'
```

**Response:**

```json
{
  "job_id": "550e8400-e29b-41d4-a716-446655440000"
}
```

### GET /api/status/{job_id}

```bash
curl http://localhost:8080/api/status/550e8400-e29b-41d4-a716-446655440000
```

**Response:**

```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "status": "done",
  "progress": 100,
  "title": "Never Gonna Give You Up",
  "download_url": "https://your-bucket.r2.dev/..."
}
```

## ⚙️ Configuração

Veja [`.env.example`](.env.example) para todas as variáveis disponíveis.

### Variáveis Principais

| Variável                    | Descrição                         | Default                 |
| --------------------------- | --------------------------------- | ----------------------- |
| `PORT`                      | Porta do servidor                 | `8080`                  |
| `ENV`                       | Ambiente (development/production) | `development`           |
| `ALLOWED_ORIGINS`           | Origins CORS permitidas           | `http://localhost:3000` |
| `TURNSTILE_SECRET_KEY`      | Chave secreta do Turnstile        | -                       |
| `DOWNLOAD_RATE_LIMIT_RPM`   | Downloads por minuto por IP       | `5`                     |
| `DOWNLOAD_RATE_LIMIT_BURST` | Burst máximo para downloads       | `2`                     |
| `STATUS_RATE_LIMIT_RPM`     | Status polls por minuto por IP    | `60`                    |
| `STATUS_RATE_LIMIT_BURST`   | Burst máximo para status          | `10`                    |
| `R2_*`                      | Credenciais Cloudflare R2         | -                       |

## 🔒 Segurança

- **Validação de URL**: Apenas domínios permitidos (YouTube, Twitter, TikTok, etc.)
- **Proteção SSRF**: Bloqueio de IPs privados e internos
- **Rate Limiting**: Por IP, configurável
- **Turnstile**: Verificação anti-bot do Cloudflare
- **yt-dlp Seguro**: Flags de segurança obrigatórias

Veja [`docs/SECURITY.md`](docs/SECURITY.md) para mais detalhes.

## 📚 Documentação

- [API Documentation](docs/API.md)
- [Security Guide](docs/SECURITY.md)
- [Deployment Guide](docs/DEPLOY.md)

## 🛠️ Desenvolvimento

```bash
# Rodar em modo dev
make dev

# Build
make build

# Testes
make test

# Lint
make lint
```

## 📦 Plataformas Suportadas

- YouTube (youtube.com, youtu.be)
- Twitter/X (twitter.com, x.com)
- TikTok (tiktok.com)
- Instagram (instagram.com)
- Facebook (facebook.com)
- Vimeo (vimeo.com)
- Reddit (reddit.com)
- Twitch (twitch.tv)
- Dailymotion (dailymotion.com)

## 📄 License

MIT - veja [LICENSE](LICENSE) para detalhes.

---

**Feito com ❤️ e Go**
