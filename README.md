# Helios – Multi-Tenant Rate Limiting Service

Helios is a **Go-based distributed rate limiting system** with support for:

- In-memory _FAST_ mode
- Redis-backed _STRONG_ consistency mode
- REST + gRPC APIs
- Observability via Prometheus + Grafana

---

## 🔧 Quick Start (Local Demo)

1. **Clone & enter the repo**

   ```powershell
   git clone https://github.com/YOUR_USERNAME/helios.git
   cd helios
   ```

2. **Build & start all services**

   ```powershell
   docker compose -f .\deploy\docker-compose.yml up -d --build
   ```

3. **Check running services**

   ```powershell
   docker ps
   ```

   You should see:

   - `helios-gateway` (ports 8080, 9080, 2112)
   - `helios-control` (ports 8081, 9081)
   - Redis, etcd, Prometheus, Grafana, Jaeger

4. **Health check**

   ```powershell
   curl.exe "http://localhost:8080/health"
   ```

---

## 🎬 Demo Scenarios

### 1. Rate Limiting in Action

```powershell
1..30 | % {
  curl.exe "http://localhost:8080/allow?tenant=acme&resource=demo&cost=1&api_key=test-key"
  Start-Sleep -Milliseconds 150
}
```

- Watch `remaining` decrease.
- Once the limit is reached, API will return:

  ```json
  { "allowed": false, "error": "rate limit exceeded", "retry_after_seconds": N }
  ```

### 2. Quota Endpoint

```powershell
curl.exe "http://localhost:8080/api/v1/quota/acme?resource=demo&api_key=test-key"
```

### 3. Metrics Endpoint

```powershell
curl.exe http://localhost:8080/metrics
```

- Look for `helios_requests_total` and `helios_rate_limits_total`.

---

## 📊 Grafana Dashboard

- Open: [http://localhost:3000](http://localhost:3000)
- Login: **admin / (password from deploy/.env.grafana)**
- Add Prometheus datasource: `http://prometheus:9090`
- Import a dashboard to visualize `helios_*` metrics.

---

## 🏗️ Architectural Overview

Helios is structured into multiple components designed for **scalability, resilience, and observability**:

### Core Services

- **Helios Gateway (`helios-gateway`)**

  - Entry point for API requests
  - Exposes HTTP and gRPC endpoints
  - Implements rate limiting using in-memory or Redis backend
  - Publishes metrics for Prometheus

- **Helios Control (`helios-control`)**

  - Configuration and coordination service
  - Integrates with etcd for service discovery and consistency
  - Manages policies, tenants, and synchronization between gateways

### Supporting Infrastructure

- **Redis** – Used when running in _STRONG_ consistency mode
- **etcd** – Service discovery and distributed configuration
- **Prometheus** – Metrics collection
- **Grafana** – Visualization dashboards
- **Jaeger** – Distributed tracing

### Flow of a Request

1. A client sends a request → `helios-gateway`.
2. Gateway extracts **tenant**, **API key**, and **resource**.
3. Gateway calls **limiter** logic (in-memory or Redis).
4. Result is returned to the client with headers:

   - `X-RateLimit-Limit`
   - `X-RateLimit-Remaining`
   - `X-RateLimit-Reset`

5. Metrics are exported to Prometheus → Grafana for dashboards.
6. Control plane (`helios-control`) manages global consistency & coordination.

---

## 🔑 Configuration

- **API Keys**:
  Configure allowed API keys with environment variable:

  ```env
  HELIOS_ALLOWED_API_KEYS="test-key,demo-key,admin-key"
  ```

- **Modes**:

  - FAST (default): in-memory limiter
  - STRONG: set `GATEWAY_CONSISTENCY_MODE=strong` to use Redis

- **Optional TLS** (future-ready):

  ```
  GATEWAY_TLS_ENABLED=true
  GATEWAY_CERT_PATH=/certs/server.crt
  GATEWAY_KEY_PATH=/certs/server.key
  ```

---

## 🛑 Stop Services

```powershell
docker compose -f .\deploy\docker-compose.yml down
```

---

## 🚀 Production Hardening Checklist

- [ ] Replace demo API keys with real config
- [ ] Use env-driven Grafana credentials
- [ ] Remove host-exposed ports for Redis & etcd
- [ ] Enable TLS everywhere
- [ ] Pin Docker image versions (avoid `latest`)
- [ ] Add resource limits in docker-compose
- [ ] Set up monitoring & alerting

---

## 🤝 Contributing

- Fork, branch, commit, PR.
- Security policy: see `SECURITY.md`.
- Pre-commit scans: run `make security-scan` if available.

---

**Helios** - Built for scale, designed for reliability. ⚡
