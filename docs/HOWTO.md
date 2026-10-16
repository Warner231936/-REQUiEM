# How to Run and Operate Requiem

## Quick Start

### 1. Prerequisites

| Component | Version | Install Link |
|-----------|---------|--------------|
| Go | 1.22+ | [go.dev](https://go.dev/doc/install) |
| MongoDB | 7.0+ | [mongodb.com](https://www.mongodb.com/docs/manual/administration/install-community/) |
| Ollama | Latest | [ollama.com](https://ollama.com/download) |

### 2. Set REQUIEM_HOME

Must be set to the project root before running:

```bash
# Linux/macOS
export REQUIEM_HOME=/path/to/requiem

# Windows PowerShell
$env:REQUIEM_HOME = "C:\path\to\requiem"

# Windows CMD
set REQUIEM_HOME=C:\path\to\requiem
```

### 3. Build & Run

```bash
# Build
go build -o requiem.exe ./cmd/requiem

# Start MongoDB (standalone for dev)
mongod --dbpath /data/db

# Start MongoDB (replica set for production/transactions)
mongod --replSet rs0 --dbpath /data/db
# In mongosh: rs.initiate({ _id: "rs0", members: [{ _id: 0, host: "127.0.0.1:27017" }] })

# Start Ollama and pull models
ollama serve
ollama pull nomic-embed-text
ollama pull llama3.2:3b
ollama create reqiquick -f Modelfile.reqiquick
# Fast model for stages 0, 4, 5

ollama create requiem1 -f Modelfile.requiem1
# Main model for stages 2, 3
```

### 4. Configure (config/config.yaml)

```yaml
auth:
  enabled: true
  # Bootstrap admin user (Macie)
  bootstrap_admin_email: "macie@requiem.cfd"
  bootstrap_admin_password: "Cw231936!"
  jwt_secret: "your-super-secure-jwt-secret-change-in-production"
```

### 5. Run Requiem

```bash
REQUIEM_HOME=. go run ./cmd/requiem
# or after building:
REQUIEM_HOME=. ./requiem.exe
```

### 6. Verify It's Running

```bash
# Health check
curl http://127.0.0.1:15153/health

# Chat (public endpoint)
curl -X POST http://127.0.0.1:15153/api/chat \
  -H "Content-Type: application/json" \
  -d '{"message": "Hello, what do you know?"}'

# SSE event stream (public)
curl -N http://127.0.0.1:15153/api/events/stream

# Metrics (public)
curl http://127.0.0.1:15153/metrics

# Mind state (requires API key auth)
curl -H "X-API-Key: your-key" http://127.0.0.1:15153/api/mind
```

---

## Operating the Web Interface

### Landing Page (requiem.cfd)

**Access:** `http://localhost:15153/` or `http://requiem.cfd` (with DNS configured)

#### Signup Flow:
1. Click **Sign Up** on the landing page
2. Enter your **name**, **email**, and **password** (min 8 chars)
3. Account is created instantly — no email verification required
4. Log in with your email and password

#### Login Flow:
1. Click **Login** on the landing page
2. Enter your **email** and **password**
3. You'll be authenticated with a JWT session

#### Password Reset:
1. Click **Forgot password?** on the login form
2. Enter your email
3. (Configure SMTP to receive reset emails)
4. Create a new password via the reset link

#### Email Verification Setup

Email verification is not required for signup/login. SMTP is optional and used only for password reset emails. To enable:

```bash
export SMTP_FROM=noreply@requiem.cfd
export SMTP_HOST=smtp.gmail.com
export SMTP_PORT=587
export SMTP_USER=your-email@gmail.com
export SMTP_PASS=your-app-password
requiem.exe
```

### Admin Portal (admin.requiem.cfd)

**Access:** `http://localhost:15153/admin/` or `http://admin.requiem.cfd`

#### Accessing Admin Portal:

1. The bootstrap admin user (Macie) is created automatically on startup with email `macie@requiem.cfd` and password `Cw231936!`
2. Go to `http://localhost:15153/admin/`
3. Enter your **email** (`macie@requiem.cfd`), **password** (`Cw231936!`), and **upload the keyfile** (`requiem.key`)
4. The keyfile is located at `public/keyfiles/requiem.key`

#### Admin Dashboard Features:

- **Statistics:** Live counts of beliefs, memories, hypotheses, and users
- **Cognitive Events:** Real-time feed of the AI's cognitive processes
- **Mind State:** View and manage beliefs, opinions, preferences, goals
- **Memory:** Search and inspect episodic and semantic memory
- **Users:** List and manage user accounts
- **Drift Detection:** Monitor for belief drift and inconsistencies
- **Snapshots:** Create and manage mind state snapshots

#### Dashboard Pages:

| Section | Path | Description |
|---------|------|-------------|
| Dashboard | `/admin/dashboard.html` | Overview with stats and events |
| Mind State | `/admin/` (click "Mind State") | Beliefs, opinions, values |
| Memory | `/admin/` (click "Memory") | Memory search and inspection |
| Users | `/admin/` (click "Users") | User management |
| Drift | `/admin/` (click "Drift Detection") | Drift analysis |
| Snapshots | `/admin/` (click "Snapshots") | Mind state snapshots |
| Settings | `/admin/` (click "Settings") | Configuration |

---

## API Reference

### Authentication Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/auth/signup` | Create new account (email verification required) |
| POST | `/api/auth/login` | Login with email/password |
| POST | `/api/auth/admin-login` | Admin login (requires keyfile) |
| POST | `/api/auth/logout` | End session |
| GET | `/api/auth/verify?token=xxx` | Verify email with token |
| POST | `/api/auth/forgot-password` | Send password reset email |
| POST | `/api/auth/reset-password` | Reset password with token |
| POST | `/api/auth/resend-verification` | Resend verification email |

### Cognitive Endpoints

| Method | Endpoint | Requires Auth | Description |
|--------|----------|---------------|-------------|
| POST | `/api/chat` | No | Send a message to the cognitive pipeline |
| GET | `/health` | No | Health check |
| GET | `/api/metrics` | No | JSON metrics |
| GET | `/metrics` | No | Prometheus metrics |
| GET | `/api/mind` | Yes | Full mind state |
| GET | `/api/mind/beliefs` | Yes | All beliefs |
| GET | `/api/mind/opinions` | Yes | All opinions |
| GET | `/api/mind/preferences` | Yes | All preferences |
| GET | `/api/mind/values` | Yes | All values |
| GET | `/api/mind/goals` | Yes | All goals |
| GET | `/api/mind/hypotheses` | Yes | All hypotheses |
| GET | `/api/mind/contradictions` | Yes | Detected contradictions |
| GET | `/api/mind/history` | Yes | Audit history |
| GET | `/api/mind/drift` | Yes | Drift detection report |
| POST | `/api/mind/snapshot` | Yes | Create mind snapshot |
| GET | `/api/memory` | Yes | Search memories |
| GET | `/api/memory/{id}` | Yes | Get specific memory |
| POST | `/api/embeddings/backfill` | Yes | Backfill missing embeddings |
| GET | `/api/events/stream` | No | SSE event stream |

---

## Configuration

### config.yaml

```yaml
server:
  host: 0.0.0.0
  port: 15153

auth:
  enabled: true
  master_key_env_var: REQUIEM_MASTER_KEY
  secrets_file: data/secrets.json.enc
  allowed_origins:
    - "http://localhost:3000"
    - "http://requiem.cfd"
    - "http://admin.requiem.cfd"
  default_key: "dev-mode-key-change-me"
  jwt_secret: "${JWT_SECRET:-requiem-dev-jwt-secret-change-me}"

database:
  mongodb:
    uri: "mongodb://127.0.0.1:27017"
    database: requiem

llm:
  default_provider: ollama
  providers:
    ollama:
      base_url: http://127.0.0.1:11434

embeddings:
  provider: ollama
  model: nomic-embed-text

pipeline:
  synchronous_consolidation: false
  debug: false
  stage0:
    enabled: true
    provider: ollama
    model: reqiquick
  # ... other stages
```

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `REQUIEM_HOME` | Project root path (required for static files) | Current directory |
| `JWT_SECRET` | JWT signing secret | `requiem-dev-jwt-secret-change-me` |
| `ADMIN_PASSWORD` | Override bootstrap admin password | `Cw231936!` |
| `MONGODB_URI` | MongoDB connection string | `mongodb://127.0.0.1:27017` |
| `MONGODB_DATABASE` | Database name | `requiem` |
| `SMTP_FROM` | Email sender address | (unset) |
| `SMTP_HOST` | SMTP server host | (unset) |
| `SMTP_PORT` | SMTP server port | (unset) |
| `SMTP_USER` | SMTP username | (unset) |
| `SMTP_PASS` | SMTP password | (unset) |
| `COGNITIVE_DEBUG` | Enable debug logging | `false` |
| `REQUIEM_AUTH_DISABLED` | Disable auth | (unset) |

---

## Operational Procedures

### Monitoring

```bash
# Watch SSE events in real-time
curl -N http://127.0.0.1:15153/api/events/stream

# Check drift over time
curl http://127.0.0.1:15153/api/mind/drift

# Monitor metrics
curl http://127.0.0.1:15153/api/metrics | jq
```

### Backups

```bash
# MongoDB backup
mongodump --uri="mongodb://127.0.0.1:27017/requiem" --out=backups/$(date +%Y%m%d)

# Restore
mongorestore --uri="mongodb://127.0.0.1:27017/requiem" backups/$(date +%Y%m%d)/requiem
```

### Keyfile Management

The admin keyfile (`requiem.key`) is located at `public/keyfiles/requiem.key`. It contains:

```json
{
  "key": "7f4a9b2c1e8d5a3f6b0c9e7d4a2f1b8e...",
  "secret": "3a9f8e7d6c5b4a3f2e1d0c9b8a7f6e5d...",
  "cert": "6b0c9e7d4a2f1b8e5c3a0d9f6e7b4a1c...",
  "permissions": ["ADMIN_FULL_ACCESS"]
}
```

To regenerate the keyfile programmatically:

```go
authService := api.NewAuthService(store, jwtSecret, "public/keyfiles/requiem.key", logger)
keyfileData, err := authService.GenerateAdminKeyfile()
```

### Health Monitoring

The `/health` endpoint returns:

```json
{
  "status": "healthy" | "degraded",
  "uptime_seconds": 12345,
  "database": "ok" | "error",
  "llm_providers": {
    "ollama": "ok" | "error"
  },
  "services": {
    "embedding": "ok",
    "event_bus": "ok"
  }
}
```

### Maintenance Mode

Set `COGNITIVE_DEBUG=true` environment variable to enable debug logging.

---

## Troubleshooting

| Problem | Solution |
|---------|----------|
| `connection refused: 27017` | MongoDB not running — start with `mongod` |
| `connection refused: 11434` | Ollama not running — start with `ollama serve` |
| `API key required` | Auth is enabled but no key configured — set `REQUIEM_MASTER_KEY` or `JWT_SECRET` |
| `email not verified` | Verification not required — account created instantly |
| `keyfile not found` | Admin keyfile missing at `public/keyfiles/requiem.key` |
| `invalid keyfile` | Keyfile was modified — restore from repository or regenerate |
| Database migration errors | Ensure MongoDB replica set is properly initiated for transactions |

---

## Cloudflare Tunnel Setup

### Port Mapping

| Service | Port |
|---------|------|
| HTTP API | 15153 |
| MongoDB | 27017 |
| Ollama | 11434 |

### Cloudflare Tunnel Configuration

```bash
# Install cloudflared
# https://developers.cloudflare.com/cloudflare-one/connections/connect-apps/install-cloudflared/

# Generate a tunnel
cloudflared tunnel create requiem-tunnel

# Create config file (config.yml)
cat > config.yml <<EOF
tunnel: <tunnel-uuid>
credentials-file: /root/.cloudflared/<tunnel-uuid>.json

ingress:
  # Admin portal
  - hostname: admin.requiem.cfd
    service: http://localhost:15153
  # Landing page
  - hostname: requiem.cfd
    service: http://localhost:15153
  # Catch-all (optional, for API-only access)
  - service: http://localhost:15153
EOF

# Run the tunnel
cloudflared tunnel run requiem-tunnel
```

### DNS Records

| Type | Name | Target | Proxy Status |
|------|------|--------|--------------|
| CNAME | requiem.cfd | <tunnel-uuid>.cfargotunnel.com | Proxied |
| CNAME | admin.requiem.cfd | <tunnel-uuid>.cfargotunnel.com | Proxied |

## Production Checklist

- [ ] Set `REQUIEM_HOME` environment variable
- [ ] Set strong `JWT_SECRET` environment variable
- [ ] Set `ADMIN_PASSWORD` environment variable (overrides config default)
- [ ] Configure SMTP for password reset emails
- [ ] Replace `dev-mode-key-change-me` API key
- [ ] Set up Cloudflare Tunnel or HTTPS with Let's Encrypt
- [ ] Configure DNS for `requiem.cfd` and `admin.requiem.cfd`
- [ ] Configure MongoDB replica set for transactions
- [ ] Set up automated backups
- [ ] Review and update CORS origins in `config.yaml`
- [ ] Monitor `/health` endpoint
- [ ] Set up Prometheus/Grafana for metrics
