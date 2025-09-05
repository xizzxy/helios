# Security Policy

## Overview

Security is a top priority for Helios. This document outlines our security practices, policies, and procedures for reporting and handling security issues.

## Supported Versions

We provide security updates for the following versions:

| Version | Supported          |
| ------- | ------------------ |
| 1.x.x   | :white_check_mark: |
| < 1.0   | :x:                |

## Reporting a Vulnerability

### How to Report

If you discover a security vulnerability in Helios, please report it responsibly:

1. **DO NOT** create a public GitHub issue
2. Send an email to: security@example.com
3. Include as much detail as possible:
   - Description of the vulnerability
   - Steps to reproduce
   - Potential impact
   - Suggested fix (if available)

### Response Timeline

- **Acknowledgment**: Within 24 hours
- **Initial Assessment**: Within 72 hours
- **Regular Updates**: Every 5 business days
- **Resolution**: Target 30 days for non-critical, 7 days for critical

### Disclosure Policy

- We practice responsible disclosure
- Security advisories published after fix is available
- Credit given to reporters (unless anonymity requested)

## Security Features

### 1. Secure Defaults

Helios ships with secure defaults:

```yaml
# Default security configuration
gateway:
  read_timeout: 30s
  write_timeout: 30s
  max_request_size: 1MB

auth:
  jwt_enabled: false  # Explicit opt-in
  api_key_required: true

observability:
  metrics_enabled: true
  profiling_enabled: false  # Disabled by default
```

### 2. Input Validation

All inputs are validated and sanitized:

```go
// Example validation middleware
func validateTenant(c *gin.Context) {
    tenant := c.Query("tenant")
    if !isValidTenant(tenant) {
        c.JSON(400, gin.H{"error": "invalid tenant"})
        c.Abort()
        return
    }
    c.Next()
}

func isValidTenant(tenant string) bool {
    // Alphanumeric, max 64 chars, no special chars
    matched, _ := regexp.MatchString(`^[a-zA-Z0-9-_]{1,64}$`, tenant)
    return matched
}
```

### 3. Rate Limiting Protection

Built-in protection against abuse:

- API endpoint rate limiting
- Per-tenant isolation
- DDoS protection via load shedding
- Circuit breakers for dependency protection

### 4. Authentication & Authorization

#### API Key Authentication

```http
GET /api/v1/allow?tenant=acme
X-API-Key: hls_1234567890abcdef...
```

- API keys follow format: `hls_` + 32 random chars
- Keys are hashed before storage (SHA-256)
- Regular rotation supported
- Scope-based permissions

#### JWT Support (Optional)

```yaml
auth:
  jwt_enabled: true
  jwt_secret_key: ${HELIOS_JWT_SECRET}
  jwt_issuer: "helios"
  jwt_audience: "helios-api"
```

### 5. Transport Security

- TLS 1.3 for all external communication
- Certificate pinning in production
- HSTS headers for web interfaces
- Secure cookie settings

### 6. Data Protection

#### Encryption at Rest
- Redis with encryption enabled
- etcd with encryption at rest
- Log files with disk encryption

#### Encryption in Transit
- TLS for all network communication
- Redis AUTH and TLS
- etcd client certificates

### 7. Security Headers

Helios adds security headers to all responses:

```go
func securityHeaders() gin.HandlerFunc {
    return func(c *gin.Context) {
        c.Header("X-Content-Type-Options", "nosniff")
        c.Header("X-Frame-Options", "DENY")
        c.Header("X-XSS-Protection", "1; mode=block")
        c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
        c.Header("Content-Security-Policy", "default-src 'self'")
        c.Next()
    }
}
```

## Secrets Management

### Environment Variables Only

**DO NOT** put secrets in:
- Source code
- Configuration files
- Docker images
- Log files
- URLs or query parameters

**DO** use environment variables:
```bash
export HELIOS_REDIS_PASSWORD="$(cat /run/secrets/redis-password)"
export HELIOS_JWT_SECRET="$(cat /run/secrets/jwt-secret)"
```

### Secret Rotation

Support for zero-downtime secret rotation:

```go
// Example: JWT secret rotation
type RotatingJWTValidator struct {
    current  []byte
    previous []byte
    mu       sync.RWMutex
}

func (r *RotatingJWTValidator) ValidateToken(token string) (*Claims, error) {
    r.mu.RLock()
    defer r.mu.RUnlock()
    
    // Try current secret first
    if claims, err := r.validate(token, r.current); err == nil {
        return claims, nil
    }
    
    // Fallback to previous secret
    return r.validate(token, r.previous)
}
```

## Security Scanning

### Automated Scanning

Our CI/CD pipeline includes:

1. **GitLeaks**: Scan for hardcoded secrets
2. **Gosec**: Static analysis for Go security issues
3. **Nancy**: Vulnerability scan for Go dependencies
4. **Container Scanning**: Scan Docker images for vulnerabilities

### Example CI Configuration

```yaml
security-scan:
  name: Security Scan
  runs-on: ubuntu-latest
  steps:
  - uses: actions/checkout@v4
  - name: Run GitLeaks
    uses: gitleaks/gitleaks-action@v2
  - name: Run Gosec
    uses: securecodewarrior/github-action-gosec@master
```

## Vulnerability Management

### Dependency Management

- Regular dependency updates via Dependabot
- Security-only updates applied immediately
- Full dependency audit quarterly

### Version Pinning

```dockerfile
# Example: Pin base image versions
FROM golang:1.21.5-alpine3.18 AS builder
FROM scratch
```

```yaml
# Example: Pin action versions
- uses: actions/checkout@v4.1.1
- uses: actions/setup-go@v4.1.0
```

## Security Best Practices

### 1. Deployment Security

#### Container Security
- Non-root user (UID 65534)
- Minimal base images (scratch/distroless)
- Read-only root filesystem
- No shell or debugging tools in production images

```dockerfile
FROM scratch
COPY --from=builder /app/helios-gateway /helios-gateway
USER 65534:65534
ENTRYPOINT ["/helios-gateway"]
```

#### Kubernetes Security
```yaml
apiVersion: apps/v1
kind: Deployment
spec:
  template:
    spec:
      securityContext:
        runAsNonRoot: true
        runAsUser: 65534
        fsGroup: 65534
      containers:
      - name: helios-gateway
        securityContext:
          allowPrivilegeEscalation: false
          readOnlyRootFilesystem: true
          capabilities:
            drop: ["ALL"]
```

### 2. Network Security

- VPC/VNET isolation
- Security groups/NSGs with minimal required access
- Private subnets for internal services
- Network policies in Kubernetes

### 3. Monitoring & Alerting

#### Security Monitoring
- Failed authentication attempts
- Unusual request patterns
- Configuration changes
- Resource exhaustion

#### Security Alerts
```yaml
# Example: Failed auth alert
alert: HighFailedAuthRate
expr: rate(helios_auth_failures_total[5m]) > 10
for: 2m
annotations:
  summary: "High failed authentication rate"
  description: "{{ $value }} failed auth attempts per second"
```

### 4. Incident Response

#### Security Incident Playbook

1. **Detection**: Automated alerts or manual report
2. **Assessment**: Determine scope and impact
3. **Containment**: Isolate affected systems
4. **Eradication**: Remove threat and vulnerabilities
5. **Recovery**: Restore normal operations
6. **Lessons Learned**: Post-incident review

#### Emergency Contacts
- Security Team: security@example.com
- On-call Engineer: +1-555-ONCALL
- Management Escalation: exec@example.com

## Compliance

### Standards Alignment

Helios is designed to support compliance with:
- **SOC 2 Type II**: Security, availability, processing integrity
- **ISO 27001**: Information security management
- **GDPR**: Data protection (where applicable)
- **HIPAA**: Healthcare data protection (with proper configuration)

### Audit Support

We maintain:
- Comprehensive audit logs
- Access control matrices
- Security control documentation
- Regular security assessments

## Security Configuration

### Production Hardening Checklist

- [ ] TLS 1.3 enabled for all services
- [ ] Strong passwords/keys (>32 chars, random)
- [ ] Regular security updates applied
- [ ] Unnecessary services disabled
- [ ] Security monitoring configured
- [ ] Backup and recovery tested
- [ ] Network segmentation implemented
- [ ] Access controls reviewed
- [ ] Security headers enabled
- [ ] Rate limiting configured

### Security Configuration Template

```yaml
# production-security.yaml
gateway:
  tls:
    enabled: true
    min_version: "1.3"
    cert_file: "/etc/tls/cert.pem"
    key_file: "/etc/tls/key.pem"
  
  security:
    api_key_required: true
    cors_enabled: false  # Configure explicitly
    max_request_size: 1MB
    timeout: 30s

  rate_limiting:
    global_limit: 1000
    per_tenant_limit: 100
    burst_limit: 150

observability:
  profiling_enabled: false
  debug_endpoints: false
  security_events: true
```

## Contact

For security-related questions or concerns:

- **Email**: security@example.com
- **PGP Key**: Available at https://example.com/security.asc
- **Bug Bounty**: Details at https://example.com/security/bounty

## Acknowledgments

We thank the security community for their responsible disclosure practices and contributions to making Helios more secure.

---

Last updated: 2024-01-15