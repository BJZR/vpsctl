<div align="center">

# 🖥️ vpsctl

**Secure, modern web panel for VPS management**

[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Go Version](https://img.shields.io/badge/Go-1.22-00ADD8.svg)](https://golang.org/)
[![Release](https://img.shields.io/github/v/release/vpsctl/vpsctl)](https://github.com/vpsctl/vpsctl/releases)
[![Docker](https://img.shields.io/badge/Docker-24.0-blue.svg)](https://www.docker.com/)
[![Platform](https://img.shields.io/badge/Platform-Linux-lightgrey.svg)]()

---

A lightweight, self-hosted web control panel for managing your VPS infrastructure. Built with Go for performance, security, and reliability.

</div>

## ✨ Features

| Feature | Description |
|---------|-------------|
| 🐳 **Docker Management** | Deploy, manage, and monitor Docker containers and compose stacks |
| 👤 **User Management** | Role-based access control with admin and user roles |
| 📊 **System Monitoring** | Real-time CPU, memory, disk, and network metrics |
| 🌐 **Nginx Proxy Manager** | Automated reverse proxy configuration with Let's Encrypt |
| 🗄️ **Database Management** | MySQL, PostgreSQL, and Redis management |
| 🔒 **SSL/TLS** | Automatic SSL certificate management |
| 📁 **File Manager** | Browse, edit, and upload files on your server |
| 🖥️ **Terminal** | Web-based SSH terminal access |
| 📋 **Logs** | View application and system logs |
| 🔧 **DNS Management** | DNS zone and record management |
| 📦 **One-Click Apps** | Deploy popular apps like WordPress, Grafana, etc. |
| 🔄 **Backups** | Automated backup and restore functionality |
| 🚀 **Fast** | Compiled Go binary — minimal resource usage |
| 🔐 **Secure** | JWT authentication, CSRF protection, rate limiting |

## 📸 Screenshots

<div align="center">

> **Screenshots coming soon!**

| Dashboard | Container Management | Terminal |
|-----------|---------------------|----------|
| ![Dashboard](docs/screenshots/dashboard.png) | ![Containers](docs/screenshots/containers.png) | ![Terminal](docs/screenshots/terminal.png) |

</div>

## 🚀 Quick Start

### One-liner Install (Linux amd64/arm64)

```bash
curl -fsSL https://raw.githubusercontent.com/vpsctl/vpsctl/main/scripts/install.sh | sudo bash
```

### Manual Install

```bash
# Download the latest release
wget https://github.com/vpsctl/vpsctl/releases/latest/download/vpsctl_linux_amd64.tar.gz
tar -xzf vpsctl_linux_amd64.tar.gz
sudo mv vpsctl /usr/local/bin/

# Create config directory
sudo mkdir -p /etc/vpsctl /var/lib/vpsctl

# Generate default config
sudo vpsctl generate-config > /etc/vpsctl/config.yaml

# Start the server
sudo vpsctl server
```

### Docker

```bash
docker run -d \
  --name vpsctl \
  -p 8080:8080 \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -v /var/lib/vpsctl:/var/lib/vpsctl \
  ghcr.io/vpsctl/vpsctl:latest
```

## ⚠️ Default Credentials

> **IMPORTANT**: Change these credentials immediately after first login!

| Field | Value |
|-------|-------|
| **URL** | `https://YOUR_SERVER_IP:8080` |
| **Username** | `admin` |
| **Password** | `admin` |

The default password is randomly generated on first startup and printed in the installation output. **Never use a weak or default password in production.**

## 🏗️ Architecture

```
┌─────────────────────────────────────────────────────────┐
│                      User's Browser                     │
│                   (HTTPS + WebSocket)                    │
└──────────────────────────┬──────────────────────────────┘
                           │
                           ▼
┌─────────────────────────────────────────────────────────┐
│                     vpsctl Server                       │
│                  (Go Binary :8080)                       │
│                                                         │
│  ┌───────────┐  ┌───────────┐  ┌───────────────────┐  │
│  │    API    │  │   Auth    │  │   WebSocket Hub   │  │
│  │  Router   │  │  (JWT)    │  │   (Real-time)     │  │
│  └─────┬─────┘  └─────┬─────┘  └────────┬──────────┘  │
│        │              │                  │              │
│  ┌─────▼──────────────▼──────────────────▼──────────┐  │
│  │              Service Layer                       │  │
│  │  ┌──────────┐ ┌────────┐ ┌──────┐ ┌──────────┐  │  │
│  │  │ Docker   │ │System  │ │Nginx │ │ Database │  │  │
│  │  │ Service  │ │Monitor │ │Proxy │ │ Manager  │  │  │
│  │  └────┬─────┘ └───┬────┘ └──┬───┘ └────┬─────┘  │  │
│  └───────┼───────────┼─────────┼───────────┼────────┘  │
│          │           │         │           │            │
│  ┌───────▼───────────▼─────────▼───────────▼────────┐  │
│  │              System Layer                        │  │
│  │  ┌──────────┐ ┌────────┐ ┌──────┐ ┌──────────┐  │  │
│  │  │ Docker   │ │Systemd │ │Nginx │ │ MySQL/PG │  │  │
│  │  │ Engine   │ │        │ │      │ │ Redis    │  │  │
│  │  └──────────┘ └────────┘ └──────┘ └──────────┘  │  │
│  └──────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────┘
```

## 🛠️ Tech Stack

| Component | Technology |
|-----------|------------|
| **Backend** | Go 1.22 |
| **Frontend** | React / TypeScript |
| **Database** | SQLite (embedded) / PostgreSQL |
| **Auth** | JWT + bcrypt |
| **Container Runtime** | Docker API |
| **Web Server** | Go net/http + TLS |
| **WebSocket** | Gorilla WebSocket |
| **Build** | GoReleaser |
| **CI/CD** | GitHub Actions |
| **Container** | Docker + Alpine Linux |

## ⚙️ Configuration

Configuration file: `/etc/vpsctl/config.yaml`

```yaml
# Server configuration
server:
  host: "0.0.0.0"
  port: 8080
  tls:
    enabled: true
    cert: "/etc/vpsctl/certs/server.crt"
    key: "/etc/vpsctl/certs/server.key"

# Authentication
auth:
  jwt_secret: "auto-generated-on-install"  # Do NOT change after setup
  session_timeout: "24h"
  max_login_attempts: 5
  lockout_duration: "15m"

# Database
database:
  driver: "sqlite"              # sqlite or postgres
  path: "/var/lib/vpsctl/data.db"
  # postgres:
  #   host: "localhost"
  #   port: 5432
  #   name: "vpsctl"
  #   user: "vpsctl"
  #   password: ""

# Docker
docker:
  socket: "/var/run/docker.sock"

# Logging
logging:
  level: "info"                 # debug, info, warn, error
  file: "/var/log/vpsctl/vpsctl.log"
  max_size: "100MB"
  max_backups: 5

# Backup
backup:
  enabled: true
  path: "/var/lib/vpsctl/backups"
  schedule: "0 2 * * *"        # Daily at 2 AM
  retention: "30d"
```

### Environment Variables

All configuration values can be overridden with environment variables:

```bash
VPSCTL_SERVER_PORT=8080
VPSCTL_AUTH_JWT_SECRET=your-secret
VPSCTL_DATABASE_DRIVER=sqlite
VPSCTL_DATABASE_PATH=/var/lib/vpsctl/data.db
VPSCTL_DOCKER_SOCKET=/var/run/docker.sock
VPSCTL_LOGGING_LEVEL=info
```

## 📡 API Documentation

### Authentication

```bash
# Login
POST /api/v1/auth/login
{
  "username": "admin",
  "password": "your-password"
}
# Response: { "token": "eyJhbG...", "expires_at": "2024-..." }

# All subsequent requests require:
Authorization: Bearer <token>
```

### Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| `POST` | `/api/v1/auth/login` | Login and get JWT token |
| `POST` | `/api/v1/auth/logout` | Logout and invalidate token |
| `GET` | `/api/v1/system/info` | Get system information |
| `GET` | `/api/v1/system/stats` | Get real-time system stats |
| `GET` | `/api/v1/docker/containers` | List all containers |
| `POST` | `/api/v1/docker/containers` | Create and start a container |
| `GET` | `/api/v1/docker/containers/:id` | Get container details |
| `POST` | `/api/v1/docker/containers/:id/start` | Start a container |
| `POST` | `/api/v1/docker/containers/:id/stop` | Stop a container |
| `DELETE` | `/api/v1/docker/containers/:id` | Remove a container |
| `GET` | `/api/v1/docker/containers/:id/logs` | Get container logs |
| `GET` | `/api/v1/docker/images` | List Docker images |
| `GET` | `/api/v1/docker/networks` | List Docker networks |
| `GET` | `/api/v1/docker/compose` | List compose stacks |
| `POST` | `/api/v1/docker/compose` | Deploy a compose stack |
| `GET` | `/api/v1/users` | List all users |
| `POST` | `/api/v1/users` | Create a new user |
| `PUT` | `/api/v1/users/:id` | Update a user |
| `DELETE` | `/api/v1/users/:id` | Delete a user |
| `GET` | `/api/v1/files/browse` | Browse filesystem |
| `POST` | `/api/v1/files/upload` | Upload files |
| `GET` | `/api/v1/files/download` | Download a file |
| `GET` | `/api/v1/nginx/sites` | List Nginx sites |
| `POST` | `/api/v1/nginx/sites` | Create Nginx config |
| `WS` | `/api/v1/ws/terminal` | WebSocket terminal session |
| `WS` | `/api/v1/ws/stats` | WebSocket real-time stats |

### WebSocket Terminal

```javascript
const ws = new WebSocket('wss://your-server:8080/api/v1/ws/terminal');
ws.onopen = () => {
  ws.send(JSON.stringify({ type: 'auth', token: 'your-jwt-token' }));
  ws.send(JSON.stringify({ type: 'resize', cols: 80, rows: 24 }));
  ws.send(JSON.stringify({ type: 'input', data: 'ls -la\n' }));
};
ws.onmessage = (event) => {
  const msg = JSON.parse(event.data);
  console.log(msg.data); // Terminal output
};
```

## 🔐 Security Features

- **JWT Authentication** with configurable expiration
- **bcrypt** password hashing (cost factor 12)
- **Rate limiting** on login attempts (configurable lockout)
- **CSRF protection** on all state-changing requests
- **TLS/HTTPS** with automatic self-signed cert generation
- **Content Security Policy** headers
- **X-Frame-Options** and **X-Content-Type-Options** headers
- **Input validation** and sanitization on all endpoints
- **SQL injection prevention** via parameterized queries
- **Docker socket isolation** — optional socket proxy support
- **Audit logging** of all administrative actions
- **Role-based access control** (admin vs. standard user)
- **Secure WebSocket** connections with origin validation

## 🤝 Contributing

We welcome contributions! Please see [CONTRIBUTING.md](CONTRIBUTING.md) for details.

```bash
# Clone the repo
git clone https://github.com/vpsctl/vpsctl.git
cd vpsctl

# Install dependencies
go mod download

# Run in development mode
make dev

# Run tests
make test

# Lint
make lint
```

## 📄 License

This project is licensed under the MIT License — see the [LICENSE](LICENSE) file for details.

## ⚖️ Disclaimer

**vpsctl** is provided "as is" without warranty of any kind. By using this software, you acknowledge that:

- You are solely responsible for the security of your server
- You should review all configurations before applying them in production
- The authors are not responsible for any data loss, security breaches, or damages
- Always maintain proper backups of your data
- Use strong, unique passwords and enable two-factor authentication where possible
- This software requires root/sudo access — use it responsibly
- Test thoroughly in a non-production environment first

---

<div align="center">

**Built with ❤️ by the vpsctl community**

[Report a Bug](https://github.com/vpsctl/vpsctl/issues) · [Request a Feature](https://github.com/vpsctl/vpsctl/issues) · [Join the Discussion](https://github.com/vpsctl/vpsctl/discussions)

</div>
