# Deployment Guide: Requiem Web Frontend

## Overview

The Requiem platform now includes a web frontend with two main entry points:
- **requiem.cfd** — Landing page with login/signup
- **admin.requiem.cfd** — Admin control panel

## Architecture

```
requiem.cfd (port 15153)
├── /                    → Landing page (login/signup)
├── /api/auth/*          → Authentication endpoints
├── /api/chat            → Chat API
├── /api/metrics         → Metrics
├── /health              → Health checks
├── /protected/*         → Authenticated API endpoints
│
admin.requiem.cfd (port 15153, same server)
├── /admin/              → Admin portal static files
│   ├── /admin/dashboard.html  → Dashboard
│   └── /admin/          → Index (login page)
└── /api/auth/admin-login      → Admin login with keyfile
```

## DNS Setup

Both domains point to the same server IP. Use your DNS provider (Cloudflare, Route53, etc.) to configure:

```
Type    Name             Value
A       requiem.cfd      <YOUR_SERVER_IP>
A       admin.requiem.cfd  <YOUR_SERVER_IP>
```

For local development, add to `/etc/hosts` (or `C:\Windows\System32\drivers\etc\hosts` on Windows):

```
127.0.0.1 requiem.cfd
127.0.0.1 admin.requiem.cfd
```

## Nginx Reverse Proxy (Recommended for Production)

```nginx
# /etc/nginx/sites-available/requiem
server {
    listen 80;
    server_name requiem.cfd;

    location / {
        proxy_pass http://127.0.0.1:15153;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }

    location /api/ {
        proxy_pass http://127.0.0.1:15153;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }

    location /api/events/stream {
        proxy_pass http://127.0.0.1:15153;
        proxy_set_header Host $host;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
    }

    # Redirect HTTP to HTTPS
    return 301 https://$host$request_uri;
}

server {
    listen 443 ssl;
    server_name requiem.cfd;

    ssl_certificate /path/to/cert.pem;
    ssl_certificate_key /path/to/key.pem;

    location / {
        proxy_pass http://127.0.0.1:15153;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}

server {
    listen 80;
    server_name admin.requiem.cfd;
    return 301 https://$host$request_uri;
}

server {
    listen 443 ssl;
    server_name admin.requiem.cfd;

    ssl_certificate /path/to/cert.pem;
    ssl_certificate_key /path/to/key.pem;

    location / {
        proxy_pass http://127.0.0.1:15153;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

## Environment Variables

```bash
# JWT Secret (change in production!)
export JWT_SECRET="your-super-secure-random-jwt-secret-here"

# SMTP Settings for email verification
export SMTP_FROM="noreply@requiem.cfd"
export SMTP_HOST="smtp.gmail.com"
export SMTP_PORT="587"
export SMTP_USER="your-email@gmail.com"
export SMTP_PASS="your-app-password"
```

## Keyfile for Admin Access

The admin portal requires a keyfile (`requiem.key`) located at `public/keyfiles/requiem.key`. This file contains cryptographic credentials that must be presented during admin login.

The keyfile is pre-generated with the following structure:
```json
{
  "key": "7f4a9b2c1e8d5a3f6b0c9e7d4a2f1b8e...",
  "secret": "3a9f8e7d6c5b4a3f2e1d0c9b8a7f6e5d...",
  "cert": "6b0c9e7d4a2f1b8e5c3a0d9f6e7b4a1c...",
  "permissions": ["ADMIN_FULL_ACCESS"]
}
```

**Important**: The keyfile must be present on the server at `public/keyfiles/requiem.key` for admin login to work. Without it, admin authentication will always fail.

## HTTPS (Let's Encrypt)

```bash
sudo certbot --nginx -d requiem.cfd -d admin.requiem.cfd
```

## CORS Configuration

Update `config/config.yaml` to include your domains:

```yaml
auth:
  allowed_origins:
    - "https://requiem.cfd"
    - "https://admin.requiem.cfd"
```
