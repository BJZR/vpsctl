# Security Policy

## Reporting a Vulnerability

The vpsctl team takes security seriously. We appreciate your efforts to responsibly disclose any security vulnerabilities you find.

**Please do NOT report security vulnerabilities through public GitHub issues, discussions, or pull requests.**

### Private Disclosure

Please report security vulnerabilities by emailing:

**security@vpsctl.dev**

Include the following information in your report:

- Type of vulnerability (e.g., buffer overflow, SQL injection, cross-site scripting, etc.)
- Full paths of source file(s) related to the vulnerability
- The location of the affected source code (tag/branch/commit or direct URL)
- Any special configuration required to reproduce the issue
- Step-by-step instructions to reproduce the vulnerability
- Proof-of-concept or exploit code (if possible)
- Impact of the vulnerability, including how an attacker might exploit it

### What to Expect

- **Acknowledgment**: We will acknowledge receipt of your vulnerability report within **48 hours**.
- **Assessment**: We will investigate and validate the reported vulnerability within **5 business days**.
- **Updates**: We will keep you informed of our progress as we address the issue.
- **Resolution**: We aim to release a fix within **30 days** of confirmation, depending on complexity.
- **Credit**: With your permission, we will credit you in the release notes for the fix.

## Supported Versions

We provide security updates for the following versions:

| Version | Supported          |
|---------|-------------------|
| 1.x.x   | :white_check_mark: |
| < 1.0   | :x:                |

## Security Measures

### Authentication & Authorization

- **JWT tokens** for stateless authentication with configurable expiration
- **bcrypt** password hashing with a cost factor of 12
- **Rate limiting** on login attempts (configurable)
- **Account lockout** after repeated failed attempts
- **Role-based access control** (admin and standard user roles)
- **Session timeout** with configurable duration

### Transport Security

- **TLS/HTTPS** support with automatic self-signed certificate generation
- **Secure WebSocket** connections with origin validation
- **HSTS headers** (HTTP Strict Transport Security)

### Input Validation & Injection Prevention

- **Parameterized queries** for all database operations
- **Input sanitization** on all API endpoints
- **Content Security Policy** headers
- **X-Frame-Options** set to DENY
- **X-Content-Type-Options** set to nosniff
- **X-XSS-Protection** enabled

### Container & System Security

- **Docker socket isolation** with read-only mount option
- **Non-root execution** in Docker containers
- **Systemd security hardening** (NoNewPrivileges, ProtectSystem, etc.)
- **Capability bounding** to minimize privilege escalation
- **PrivateTmp** to isolate temporary files

### Data Protection

- **Encrypted storage** of sensitive configuration (TLS keys, JWT secrets)
- **Secure file permissions** (600 for keys, 640 for configs)
- **Audit logging** of administrative actions
- **No sensitive data in logs** (passwords, tokens, secrets)
- **SQL injection prevention** via parameterized queries

### API Security

- **CSRF protection** on all state-changing requests
- **Rate limiting** per IP and per user
- **Request size limits** to prevent DoS
- **Timeout handling** for long-running operations
- **Input validation** using structured schemas

## Security Configuration

### Recommended Production Setup

1. **Use a reverse proxy** (Nginx, Caddy) with a valid TLS certificate
2. **Enable firewall rules** — only expose necessary ports
3. **Change default credentials** immediately after installation
4. **Enable two-factor authentication** (when available)
5. **Regularly update** vpsctl to the latest version
6. **Monitor logs** for suspicious activity
7. **Use strong, unique passwords** for all accounts
8. **Limit network access** to the management panel
9. **Backup your data** regularly
10. **Review audit logs** periodically

### Firewall Configuration

```bash
# Allow SSH (port 22)
sudo ufw allow 22/tcp

# Allow HTTP/HTTPS (ports 80, 443)
sudo ufw allow 80/tcp
sudo ufw allow 443/tcp

# Allow vpsctl (port 8080) — only if not using a reverse proxy
sudo ufw allow 8080/tcp

# Enable firewall
sudo ufw enable
```

### TLS Certificate (Let's Encrypt)

For production use, we recommend using a valid TLS certificate from Let's Encrypt:

```bash
# Install certbot
sudo apt install certbot

# Obtain certificate
sudo certbot certonly --standalone -d your-domain.com

# Update vpsctl config
# /etc/vpsctl/config.yaml
# server:
#   tls:
#     cert: "/etc/letsencrypt/live/your-domain.com/fullchain.pem"
#     key: "/etc/letsencrypt/live/your-domain.com/privkey.pem"
```

## Known Security Considerations

### Docker Socket Access

vpsctl requires access to the Docker socket (`/var/run/docker.sock`) for container management. This provides significant control over the host system. To minimize risk:

- Run vpsctl in a Docker container with read-only socket mount
- Use Docker socket proxy solutions for production
- Restrict vpsctl access to trusted users only
- Monitor Docker API access through audit logs

### Root/Sudo Access

Some vpsctl operations require elevated privileges. The systemd service runs as a dedicated `vpsctl` user with minimal capabilities. Only operations that require root are executed through controlled escalation.

### Database Security

- SQLite database files have restricted permissions (600)
- All queries use parameterized statements
- Database backups are encrypted when stored

## Compliance

While vpsctl is not officially certified for any compliance standard, we implement security best practices that align with:

- **OWASP Top 10** mitigation strategies
- **CIS Benchmark** recommendations for Linux systems
- **NIST Cybersecurity Framework** principles

## Updates and Notifications

- Security updates are announced in [GitHub Releases](https://github.com/vpsctl/vpsctl/releases)
- Subscribe to the repository for notifications
- Check the changelog for security-related changes

## Contact

For any security-related questions or concerns:

- **Email**: security@vpsctl.dev
- **GitHub Security Advisories**: [Report a vulnerability](https://github.com/vpsctl/vpsctl/security/advisories/new)

## Credits

We would like to thank security researchers and contributors who help improve vpsctl's security. With permission, contributors are credited in release notes.

---

*Last updated: 2024*
