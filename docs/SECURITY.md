# Security Guide

## Overview

A yt-dl-api-go foi projetada com segurança em mente, implementando múltiplas camadas de proteção.

## Camadas de Segurança

### 1. Validação de URL

**Arquivo:** `internal/transport/http/middleware/validator.go`

A API mantém uma **allowlist de domínios** permitidos:

```
youtube.com, youtu.be, twitter.com, x.com, tiktok.com,
instagram.com, facebook.com, vimeo.com, reddit.com, twitch.tv
```

**Verificações realizadas:**

- ✅ URL é válida (parsing)
- ✅ Scheme é HTTPS
- ✅ Domínio está na allowlist
- ✅ Sem credenciais na URL (`user:pass@host`)
- ✅ Subdomínios são verificados contra domínio pai

### 2. Proteção SSRF

**Arquivo:** `pkg/safeclient/client.go`

O cliente HTTP bloqueia conexões para:

```
10.0.0.0/8       (Private Class A)
172.16.0.0/12    (Private Class B)
192.168.0.0/16   (Private Class C)
127.0.0.0/8      (Loopback)
169.254.0.0/16   (Link-local)
224.0.0.0/4      (Multicast)
169.254.169.254  (Cloud metadata)
fc00::/7         (IPv6 private)
::1              (IPv6 loopback)
```

**Proteção contra DNS Rebinding:**

- A validação do IP ocorre **no momento da conexão** (não no DNS lookup)
- Usa `net.Dialer.Control` para verificar o IP resolvido
- Previne ataques onde DNS retorna IP público inicialmente e IP privado depois

### 3. Rate Limiting

**Arquivo:** `internal/transport/http/middleware/ratelimit.go`

**Configuração padrão:**

- 5 requests por minuto por IP
- Burst de 2 requests
- Cleanup de IPs antigos a cada 10 minutos

**Headers de resposta:**

- `X-RateLimit-Remaining`: Requests restantes
- `Retry-After`: Segundos para tentar novamente (quando limitado)

**Identificação de IP:**

1. `CF-Connecting-IP` (Cloudflare)
2. `X-Real-IP`
3. `X-Forwarded-For`
4. `RemoteAddr` (fallback)

### 4. Cloudflare Turnstile

**Arquivo:** `internal/transport/http/middleware/turnstile.go`

O Turnstile é um CAPTCHA invisível que:

- Verifica se o usuário é humano
- Não requer interação na maioria dos casos
- Token válido por poucos segundos

**Configuração:**

1. Crie um site no [Cloudflare Dashboard](https://dash.cloudflare.com/turnstile)
2. Copie a Secret Key para `TURNSTILE_SECRET_KEY`
3. Use a Site Key no frontend

**Bypass em desenvolvimento:**

```env
TURNSTILE_SKIP=true
```

### 5. yt-dlp Seguro

**Arquivo:** `internal/service/downloader/ytdlp.go`

**Flags de segurança obrigatórias:**

```
--no-playlist           # Impede download de playlists
--max-filesize 500M     # Limite de tamanho
--match-filter "duration<1800"  # Máximo 30 min
--newline               # Progress em linhas separadas
--no-cache-dir          # Sem cache (segurança)
--socket-timeout 30     # Timeout de conexão
--retries 3             # Tentativas limitadas
```

**Execução segura:**

- Usa `exec.CommandContext` com timeout de 10 minutos
- Argumentos passados como slice (não shell)
- Output parseado linha por linha

---

## Configuração para Produção

### 1. Variáveis de Ambiente

```env
# Obrigatório em produção
ENV=production
TURNSTILE_SECRET_KEY=sua-chave-secreta
TURNSTILE_SKIP=false

# Rate limiting ajustado para produção
RATE_LIMIT_RPM=3
RATE_LIMIT_BURST=1

# Origins específicas
ALLOWED_ORIGINS=https://seu-site.com
```

### 2. Cloudflare Proxy

Recomenda-se usar o Cloudflare como proxy reverso:

1. **DNS Proxied**: Ative o proxy laranja no DNS
2. **SSL/TLS**: Configure como "Full (strict)"
3. **WAF**: Ative regras managed do Cloudflare
4. **Under Attack Mode**: Disponível se necessário

### 3. Atualizações

```bash
# Manter yt-dlp atualizado (vulnerabilidades)
pip install --upgrade yt-dlp

# Dentro do container
docker compose exec api pip install --upgrade yt-dlp
```

---

## Recomendações Adicionais

1. **Não exponha a API diretamente** - Use Cloudflare ou outro proxy reverso
2. **Monitore os logs** - Ataques deixam rastros
3. **Limite recursos** - Use os limits do Docker Compose
4. **Backup do SQLite** - Faça backup regular do `data/jobs.db`
5. **Rotação de credenciais** - Troque as chaves do R2 periodicamente

---

## Reporting Vulnerabilities

Se encontrar uma vulnerabilidade, por favor:

1. **Não abra uma issue pública**
2. Envie um email para: security@seu-dominio.com
3. Descreva o problema detalhadamente
4. Aguarde resposta antes de divulgar

Vulnerabilidades confirmadas serão corrigidas e creditadas no changelog.
