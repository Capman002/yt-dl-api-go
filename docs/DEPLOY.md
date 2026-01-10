# Deployment Guide

## Requisitos

- Docker e Docker Compose
- Ou: Go 1.23+, yt-dlp, ffmpeg
- Conta Cloudflare (para Turnstile e R2)

---

## Deploy com Docker (Recomendado)

### 1. Clone e Configure

```bash
git clone https://github.com/seu-user/yt-dl-api-go
cd yt-dl-api-go
cp .env.example .env
```

### 2. Edite `.env`

```env
# Produção
ENV=production
PORT=8080

# Segurança
TURNSTILE_SECRET_KEY=sua-chave-secreta
RATE_LIMIT_RPM=3

# R2 Storage
R2_ACCOUNT_ID=seu-account-id
R2_ACCESS_KEY_ID=sua-access-key
R2_SECRET_ACCESS_KEY=sua-secret-key
R2_BUCKET_NAME=seu-bucket
R2_PUBLIC_URL=https://seu-bucket.r2.dev

# Origins (seu frontend)
ALLOWED_ORIGINS=https://seu-site.com
```

### 3. Inicie

```bash
docker compose up -d
```

### 4. Verifique

```bash
curl http://localhost:8080/api/health
```

---

## Deploy por Plataforma

### Fly.io

```bash
# Instalar flyctl
curl -L https://fly.io/install.sh | sh

# Login
fly auth login

# Criar app
fly launch

# Configurar secrets
fly secrets set TURNSTILE_SECRET_KEY=xxx
fly secrets set R2_ACCESS_KEY_ID=xxx
fly secrets set R2_SECRET_ACCESS_KEY=xxx

# Deploy
fly deploy
```

**fly.toml:**

```toml
app = "yt-dl-api"
primary_region = "gru"

[build]
  dockerfile = "Dockerfile"

[http_service]
  internal_port = 8080
  force_https = true

[mounts]
  source = "data"
  destination = "/app/data"

[[vm]]
  cpu_kind = "shared"
  cpus = 1
  memory_mb = 1024
```

### Railway

1. Conecte seu repositório GitHub
2. Configure variáveis de ambiente no dashboard
3. Deploy automático em cada push

### Hetzner/VPS

```bash
# SSH no servidor
ssh root@seu-servidor

# Instale Docker
curl -fsSL https://get.docker.com | sh

# Clone e configure
git clone https://github.com/seu-user/yt-dl-api-go
cd yt-dl-api-go
cp .env.example .env
nano .env

# Inicie
docker compose up -d

# Configure Nginx como proxy reverso (opcional)
apt install nginx certbot python3-certbot-nginx
```

**Nginx config:**

```nginx
server {
    server_name api.seu-site.com;

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

```bash
# SSL
certbot --nginx -d api.seu-site.com
```

### Oracle Cloud Free Tier

1. Crie uma instância ARM (4 OCPUs, 24GB RAM - grátis!)
2. Abra porta 8080 no Security List
3. Siga as instruções do VPS acima

---

## Configuração do Cloudflare

### 1. DNS

```
Tipo    Nome        Valor           Proxy
A       api         IP-do-servidor  ✅ Proxied
```

### 2. SSL/TLS

- Modo: Full (strict)
- Always Use HTTPS: ✅

### 3. Turnstile

1. Vá para Turnstile no dashboard
2. Crie um novo site
3. Copie Site Key (frontend) e Secret Key (.env)

### 4. R2

1. Vá para R2 no dashboard
2. Crie um bucket
3. Vá em Settings > S3 API
4. Copie Account ID e gere tokens de API

---

## Monitoramento

### Logs

```bash
# Docker
docker compose logs -f

# Formato JSON em produção
docker compose logs -f | jq
```

### Health Check

```bash
curl http://localhost:8080/api/health
```

### Métricas

O SQLite armazena todos os jobs. Você pode consultar:

```bash
# Entrar no container
docker compose exec api sh

# Query SQLite
sqlite3 /app/data/jobs.db "SELECT status, COUNT(*) FROM jobs GROUP BY status"
```

---

## Troubleshooting

### yt-dlp não encontrado

```bash
docker compose exec api yt-dlp --version
# Se falhar, reinstale:
docker compose exec api pip install --upgrade yt-dlp
```

### Erro de permissão no volume

```bash
# No host
chown -R 1000:1000 ./data ./tmp
```

### Rate limit muito restritivo

```env
RATE_LIMIT_RPM=10
RATE_LIMIT_BURST=3
```

### Downloads falhando

```bash
# Testar yt-dlp diretamente
docker compose exec api yt-dlp --no-download --print-json "URL"
```
