# API Documentation

## Overview

A yt-dl-api-go fornece uma API REST simples para download de vídeos de plataformas populares usando yt-dlp.

## Base URL

```
http://localhost:8080/api
```

## Authentication

A API usa Cloudflare Turnstile para proteção anti-bot. Inclua o token no header:

```
X-Turnstile-Token: your-turnstile-token
```

## Rate Limiting

- **Limite padrão**: 5 requests/minuto por IP
- **Burst**: 2 requests
- Header `Retry-After` indica quando tentar novamente
- Header `X-RateLimit-Remaining` indica requests restantes

---

## Endpoints

### POST /api/download

Inicia um novo download de vídeo.

**Request Headers:**

```
Content-Type: application/json
X-Turnstile-Token: <token>
```

**Request Body:**

```json
{
  "url": "https://www.youtube.com/watch?v=dQw4w9WgXcQ"
}
```

**Response (202 Accepted):**

```json
{
  "job_id": "550e8400-e29b-41d4-a716-446655440000"
}
```

**Error Responses:**

| Status | Code                | Description                           |
| ------ | ------------------- | ------------------------------------- |
| 400    | `INVALID_URL`       | URL inválida ou domínio não permitido |
| 400    | `INVALID_BODY`      | Body da request inválido              |
| 403    | `TURNSTILE_INVALID` | Token Turnstile inválido              |
| 429    | `RATE_LIMIT`        | Rate limit excedido                   |
| 503    | `QUEUE_FULL`        | Servidor ocupado                      |

---

### GET /api/status/{job_id}

Consulta o status de um job de download.

**Parameters:**

- `job_id` (path) - UUID do job

**Response (200 OK):**

```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "status": "processing",
  "progress": 45,
  "title": "Video Title",
  "download_url": null,
  "error": null,
  "created_at": "2026-01-09T12:00:00Z",
  "completed_at": null
}
```

**Job Status Values:**

| Status       | Description                                   |
| ------------ | --------------------------------------------- |
| `pending`    | Job na fila, aguardando processamento         |
| `processing` | Download em progresso                         |
| `done`       | Download concluído, `download_url` disponível |
| `error`      | Erro no download, veja `error` para detalhes  |

**Error Responses:**

| Status | Code             | Description                |
| ------ | ---------------- | -------------------------- |
| 400    | `INVALID_JOB_ID` | Formato de job_id inválido |
| 404    | `JOB_NOT_FOUND`  | Job não encontrado         |

---

### GET /api/health

Health check endpoint.

**Response (200 OK):**

```json
{
  "status": "ok",
  "queue_size": 3,
  "workers": 3
}
```

---

## Exemplos

### cURL

```bash
# Iniciar download
curl -X POST http://localhost:8080/api/download \
  -H "Content-Type: application/json" \
  -H "X-Turnstile-Token: your-token" \
  -d '{"url": "https://www.youtube.com/watch?v=dQw4w9WgXcQ"}'

# Verificar status
curl http://localhost:8080/api/status/550e8400-e29b-41d4-a716-446655440000

# Health check
curl http://localhost:8080/api/health
```

### JavaScript (Fetch)

```javascript
async function downloadVideo(url, turnstileToken) {
  const response = await fetch("http://localhost:8080/api/download", {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      "X-Turnstile-Token": turnstileToken,
    },
    body: JSON.stringify({ url }),
  });

  if (!response.ok) {
    const error = await response.json();
    throw new Error(error.error);
  }

  return response.json();
}

async function checkStatus(jobId) {
  const response = await fetch(`http://localhost:8080/api/status/${jobId}`);
  return response.json();
}

// Polling para aguardar conclusão
async function waitForDownload(jobId, interval = 2000) {
  while (true) {
    const status = await checkStatus(jobId);

    if (status.status === "done") {
      return status.download_url;
    }

    if (status.status === "error") {
      throw new Error(status.error);
    }

    await new Promise((resolve) => setTimeout(resolve, interval));
  }
}
```

---

## Plataformas Suportadas

| Plataforma  | Domínios                             |
| ----------- | ------------------------------------ |
| YouTube     | youtube.com, youtu.be, m.youtube.com |
| Twitter/X   | twitter.com, x.com                   |
| TikTok      | tiktok.com, vm.tiktok.com            |
| Instagram   | instagram.com                        |
| Facebook    | facebook.com, fb.watch               |
| Vimeo       | vimeo.com, player.vimeo.com          |
| Reddit      | reddit.com, v.redd.it                |
| Twitch      | twitch.tv, clips.twitch.tv           |
| Dailymotion | dailymotion.com                      |

---

## Limitações

- **Tamanho máximo**: 500MB por arquivo
- **Duração máxima**: 30 minutos
- **Qualidade máxima**: 1080p
- **Playlists**: Não suportadas (apenas vídeos individuais)
- **URLs com credenciais**: Não permitidas

---

## Error Response Format

Todas as respostas de erro seguem este formato:

```json
{
  "error": "Mensagem de erro legível",
  "code": "CODIGO_ERRO"
}
```
