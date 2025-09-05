# 🚀 Getting Started with Helios

## Quick Start (3 Steps)

### 1. Start Helios
```bash
# Option A: Quick start (if you have Go)
chmod +x scripts/dev_up.sh
./scripts/dev_up.sh

# Option B: Manual start
go run ./cmd/helios-gateway
```

### 2. Test the APIs
```bash
# Test basic functionality
chmod +x scripts/test_apis.sh
./scripts/test_apis.sh
```

### 3. Start Building!
Your Helios gateway is now running at `http://localhost:8080`

---

## 🎯 Built-in APIs

### Core Rate Limiting APIs

#### ✅ Allow API (Check Rate Limit)
```bash
curl "http://localhost:8080/allow?tenant=myapp&api_key=demo-key"
```
**Response:**
```json
{
  "allowed": true,
  "remaining": 99,
  "limit": 100,
  "reset_time": 1672531200
}
```

#### ✅ Quota API (Get Current Status)
```bash
curl "http://localhost:8080/api/v1/quota/myapp?api_key=demo-key"
```

#### ✅ Health API
```bash
curl "http://localhost:8080/health"
```

#### ✅ Metrics API (Prometheus)
```bash
curl "http://localhost:8080/api/v1/metrics"
```

---

## 🛠️ Adding Your Own APIs

### Step 1: Add Route Handler

Edit `internal/gateway/server.go` around line 147:

```go
// In setupRoutes function, add to the api group:
api := router.Group("/api/v1")
{
    api.GET("/allow", s.handleAllow)
    api.GET("/quota/:tenant", s.handleQuota) 
    api.GET("/metrics", s.handleMetrics)
    
    // 👇 Add your new APIs here
    api.GET("/status/:tenant", s.handleTenantStatus)
    api.POST("/webhook", s.handleWebhook)
    api.GET("/analytics/:tenant", s.handleAnalytics)
}
```

### Step 2: Implement Handler Function

Add your handler functions in `internal/gateway/server.go`:

```go
// Add this after the existing handlers (around line 280)
func (s *Server) handleTenantStatus(c *gin.Context) {
    tenant := c.Param("tenant")
    apiKey := c.GetHeader("X-API-Key")
    
    if tenant == "" {
        c.JSON(400, gin.H{"error": "tenant required"})
        return
    }
    
    // Your custom logic here
    status := map[string]interface{}{
        "tenant": tenant,
        "active": true,
        "requests_today": 1500,
        "limits": map[string]int{
            "api": 1000,
            "uploads": 100,
        },
    }
    
    c.JSON(200, status)
}

func (s *Server) handleWebhook(c *gin.Context) {
    var payload map[string]interface{}
    if err := c.ShouldBindJSON(&payload); err != nil {
        c.JSON(400, gin.H{"error": "invalid JSON"})
        return
    }
    
    // Process webhook
    s.logger.Info("Received webhook", "payload", payload)
    
    c.JSON(200, gin.H{"status": "processed"})
}

func (s *Server) handleAnalytics(c *gin.Context) {
    tenant := c.Param("tenant")
    
    // Mock analytics data
    analytics := gin.H{
        "tenant": tenant,
        "period": "24h",
        "total_requests": 5420,
        "allowed_requests": 5200,
        "blocked_requests": 220,
        "top_resources": []string{"api", "uploads", "downloads"},
    }
    
    c.JSON(200, analytics)
}
```

### Step 3: Test Your New APIs

```bash
# Restart Helios
go run ./cmd/helios-gateway

# Test your new endpoints
curl "http://localhost:8080/api/v1/status/myapp" -H "X-API-Key: demo"
curl "http://localhost:8080/api/v1/analytics/myapp"
curl -X POST "http://localhost:8080/api/v1/webhook" \
  -H "Content-Type: application/json" \
  -d '{"event": "test", "data": {"key": "value"}}'
```

---

## 🎨 Common API Patterns

### 1. With Rate Limiting Protection
```go
func (s *Server) handleProtectedAPI(c *gin.Context) {
    tenant := c.Query("tenant")
    apiKey := c.GetHeader("X-API-Key")
    
    // Check rate limit first
    limiter, err := s.limiterMgr.GetLimiter(tenant, "protected-api")
    if err != nil {
        c.JSON(500, gin.H{"error": "internal error"})
        return
    }
    
    result, err := limiter.Allow(c.Request.Context(), 
        fmt.Sprintf("%s:%s", tenant, apiKey), 1)
    if err != nil {
        c.JSON(500, gin.H{"error": "rate limit check failed"})
        return
    }
    
    if !result.Allowed {
        c.Header("Retry-After", fmt.Sprintf("%d", result.RetryAfterSeconds))
        c.JSON(429, gin.H{"error": "rate limit exceeded"})
        return
    }
    
    // Your API logic here
    c.JSON(200, gin.H{"message": "success", "remaining": result.Remaining})
}
```

### 2. With Authentication
```go
func (s *Server) handleSecureAPI(c *gin.Context) {
    apiKey := c.GetHeader("X-API-Key")
    if apiKey == "" {
        c.JSON(401, gin.H{"error": "API key required"})
        return
    }
    
    // Validate API key (implement your logic)
    if !s.isValidAPIKey(apiKey) {
        c.JSON(401, gin.H{"error": "invalid API key"})
        return
    }
    
    // Your secure API logic
    c.JSON(200, gin.H{"message": "authenticated request"})
}
```

### 3. With JSON Input/Output
```go
func (s *Server) handleDataAPI(c *gin.Context) {
    var request struct {
        Tenant   string            `json:"tenant" binding:"required"`
        Action   string            `json:"action" binding:"required"`
        Metadata map[string]string `json:"metadata"`
    }
    
    if err := c.ShouldBindJSON(&request); err != nil {
        c.JSON(400, gin.H{"error": err.Error()})
        return
    }
    
    // Process request
    response := gin.H{
        "tenant":    request.Tenant,
        "action":    request.Action,
        "processed": true,
        "timestamp": time.Now().Unix(),
    }
    
    c.JSON(200, response)
}
```

---

## 🔧 Configuration

### Environment Variables
```bash
export HELIOS_LOG_LEVEL=debug
export HELIOS_CONSISTENCY_MODE=fast  # or "strong" for Redis
export HELIOS_GATEWAY_ADDRESS=:8080
export HELIOS_METRICS_ENABLED=true
```

### Default Rate Limits
The gateway starts with sensible defaults:
- **100 requests/minute** per tenant
- **Token bucket algorithm** (allows bursts)
- **120 burst capacity**

### Change Default Limits
Edit `internal/limiter/manager.go` around line 30:
```go
config = &Config{
    Algorithm:     TokenBucket,
    Limit:         1000,  // 👈 Change this
    WindowSeconds: 60,
    BurstLimit:    1200,  // 👈 And this
}
```

---

## 🚀 Next Steps

1. **Add your business logic** to the handler functions
2. **Connect to your database** for tenant/user management
3. **Add authentication** with JWT or custom API keys
4. **Set up monitoring** with the built-in Prometheus metrics
5. **Deploy with Docker** using the provided configurations

## 💡 Need Help?

- Check the [README.md](README.md) for full documentation
- Look at [DESIGN.md](DESIGN.md) for architecture details
- Review [SECURITY.md](SECURITY.md) for security best practices
- Examine existing handlers in `internal/gateway/server.go`

Happy building! 🎉