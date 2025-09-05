package gateway

import (
	"context"
	"fmt"
	"log/slog"
	
	"net"
	"net/http"
	"strconv"
	"time"
	"sync/atomic"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"github.com/xizzxy/helios/internal/config"
	"github.com/xizzxy/helios/internal/limiter"
	"github.com/xizzxy/helios/internal/store"
)

type Server struct {
	config     *config.Config
	httpServer *http.Server
	grpcServer *grpc.Server
	limiterMgr *limiter.LocalManager
	redisStore *store.Client
	logger     *slog.Logger
}
// --- simple in-process counters for demo metrics ---
var (
    reqTotal   uint64
    reqAllowed uint64
    reqDenied  uint64
)


func NewServer(cfg *config.Config, logger *slog.Logger) (*Server, error) {
	var redisStore *store.Client

	// Strong mode would use Redis. In FAST mode we keep redisStore nil (nop).
	if cfg.Gateway.ConsistencyMode == "strong" {
		c, err := store.NewClientFromEnv()
		if err != nil {
			return nil, fmt.Errorf("failed to create Redis client: %w", err)
		}
		redisStore = c
		logger.Info("Using Redis-based rate limiting (strong mode)")
	} else {
		logger.Info("Using in-memory rate limiting (fast mode)")
	}

	// Demo default policy. Your LocalManager takes a single tenant key.
	limiterMgr := limiter.NewLocalManager(limiter.Config{
		Limit:     100,
		Burst:     100,
		Window:    time.Minute,
		Algorithm: limiter.AlgoTokenBucket,
	})

	// Gin router
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(LoggerMiddleware(logger))
	router.Use(CORSMiddleware())

	s := &Server{
		config:     cfg,
		limiterMgr: limiterMgr,
		redisStore: redisStore,
		logger:     logger,
	}

	s.setupRoutes(router)

	// HTTP server
	s.httpServer = &http.Server{
		Addr:         cfg.Gateway.Address,
		Handler:      router,
		ReadTimeout:  cfg.Gateway.ReadTimeout,
		WriteTimeout: cfg.Gateway.WriteTimeout,
	}

	// gRPC server (reflection only for now)
	s.grpcServer = grpc.NewServer(
		grpc.UnaryInterceptor(s.unaryInterceptor),
	)
	reflection.Register(s.grpcServer)

	return s, nil
}

func (s *Server) Start(ctx context.Context) error {
	// HTTP
	go func() {
		s.logger.Info("Starting HTTP server", "address", s.config.Gateway.Address)
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.logger.Error("HTTP server error", "error", err)
		}
	}()

	// gRPC
	go func() {
		lis, err := net.Listen("tcp", s.config.Gateway.GRPCAddress)
		if err != nil {
			s.logger.Error("Failed to listen for gRPC", "error", err)
			return
		}
		s.logger.Info("Starting gRPC server", "address", s.config.Gateway.GRPCAddress)
		if err := s.grpcServer.Serve(lis); err != nil {
			s.logger.Error("gRPC server error", "error", err)
		}
	}()

	return nil
}

func (s *Server) Stop(ctx context.Context) error {
	s.logger.Info("Shutting down gateway server")

	// Stop HTTP
	if err := s.httpServer.Shutdown(ctx); err != nil {
		s.logger.Error("HTTP server shutdown error", "error", err)
	}

	// Stop gRPC
	s.grpcServer.GracefulStop()

	// Close Redis store (if any)
	if s.redisStore != nil {
		if err := s.redisStore.Close(); err != nil {
			s.logger.Error("Failed to close Redis store", "error", err)
		}
	}

	return nil
}

func (s *Server) setupRoutes(router *gin.Engine) {
	// Health
	router.GET("/health", s.handleHealth)

	// REST API
	api := router.Group("/api/v1")
	{
		api.GET("/allow", s.handleAllow)
		api.GET("/quota/:tenant", s.handleQuota)
		api.GET("/metrics", s.handleMetrics)
	}

	// Back-compat
	router.GET("/allow", s.handleAllow)

	// Very basic Prometheus text endpoint (placeholder)
	router.GET("/metrics", s.handlePrometheusMetrics)
}

func (s *Server) handleHealth(c *gin.Context) {
	status := "healthy"
	checks := make(map[string]string)

	// Redis (only if configured)
	if s.redisStore != nil {
		if err := s.redisStore.Ping(); err != nil {
			status = "unhealthy"
			checks["redis"] = fmt.Sprintf("error: %v", err)
		} else {
			checks["redis"] = "healthy"
		}
	}

	checks["limiter"] = "healthy"

	c.JSON(http.StatusOK, gin.H{
		"status":  status,
		"version": s.config.Observability.ServiceVersion,
		"checks":  checks,
	})
}

func (s *Server) handleAllow(c *gin.Context) {
    tenant := c.Query("tenant")
    if tenant == "" {
        c.JSON(http.StatusBadRequest, gin.H{"error": "tenant parameter is required"})
        return
    }

    apiKey := c.GetHeader("X-API-Key")
    if apiKey == "" {
        apiKey = c.Query("api_key")
    }
    if apiKey == "" {
        c.JSON(http.StatusUnauthorized, gin.H{"error": "API key required"})
        return
    }

    // demo keys — replace with etcd/config validation later
    switch apiKey {
    case "test-key", "demo-key", "admin-key":
    default:
        c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid API key"})
        return
    }

    resource := c.Query("resource")
    if resource == "" {
        resource = "default"
    }

    // parse cost
    cost := 1
    if costStr := c.Query("cost"); costStr != "" {
        if n, err := strconv.Atoi(costStr); err == nil && n > 0 {
            cost = n
        } else {
            c.JSON(http.StatusBadRequest, gin.H{"error": "invalid cost parameter"})
            return
        }
    }

    // get limiter for tenant (your LocalManager takes only tenant)
    rl := s.limiterMgr.ForTenant(tenant)

    // key used by the limiter
    key := fmt.Sprintf("%s:%s:%s", tenant, resource, apiKey)
	// count request
    atomic.AddUint64(&reqTotal, 1)

    // ✅ NEW: two-value return and int64 cost
    res, err := rl.Allow(c.Request.Context(), key, int64(cost))

	if res.Allowed {
		atomic.AddUint64(&reqAllowed, 1)
	} else {
		atomic.AddUint64(&reqDenied, 1)
	}
	
    if err != nil {
        s.logger.Error("Rate limit check failed", "tenant", tenant, "resource", resource, "error", err)
        c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
        return
    }

    // headers
	c.Header("X-RateLimit-Limit", fmt.Sprintf("%d", res.Limit))
	c.Header("X-RateLimit-Remaining", fmt.Sprintf("%d", res.Remaining))
	c.Header("X-RateLimit-Reset", fmt.Sprintf("%d", res.ResetTime.Unix()))
	c.Header("X-Helios-Mode", s.config.Gateway.ConsistencyMode)
	
	if !res.Allowed {
		// If your Result struct doesn’t have RetryAfterSeconds, compute it from ResetTime:
		retryAfter := int(time.Until(res.ResetTime).Seconds())
		if retryAfter < 0 {
			retryAfter = 0
		}
		c.Header("Retry-After", strconv.Itoa(retryAfter))
		c.JSON(http.StatusTooManyRequests, gin.H{
			"allowed":             false,
			"error":               "rate limit exceeded",
			"retry_after_seconds": retryAfter,
		})
		return
	}
	

    c.JSON(http.StatusOK, gin.H{
        "allowed":    true,
        "remaining":  res.Remaining,
        "limit":      res.Limit,
        "reset_time": res.ResetTime.Unix(),
    })
}


func (s *Server) handleQuota(c *gin.Context) {
    tenant := c.Param("tenant")
    if tenant == "" {
        c.JSON(http.StatusBadRequest, gin.H{"error": "tenant parameter is required"})
        return
    }

    apiKey := c.GetHeader("X-API-Key")
    if apiKey == "" {
        apiKey = c.Query("api_key")
    }
    if apiKey == "" {
        c.JSON(http.StatusUnauthorized, gin.H{"error": "API key required"})
        return
    }

    // Demo keys
    valid := map[string]struct{}{"test-key": {}, "demo-key": {}, "admin-key": {}}
    if _, ok := valid[apiKey]; !ok {
        c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid API key"})
        return
    }

    resource := c.Query("resource")
    if resource == "" {
        resource = "default"
    }

    // Get limiter and read current state (cost=0)
    rl := s.limiterMgr.ForTenant(tenant)
    id := fmt.Sprintf("%s:%s:%s", tenant, resource, apiKey)

    res, err := rl.Allow(c.Request.Context(), id, int64(0))
    if err != nil {
        s.logger.Error("Get quota failed", "id", id, "error", err)
        c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
        return
    }

    c.JSON(http.StatusOK, gin.H{
        "remaining":  res.Remaining,
        "limit":      res.Limit,
        "reset_time": res.ResetTime.Unix(),
    })
}

func (s *Server) handleMetrics(c *gin.Context) {
	metrics := make(map[string]interface{})
	metrics["timestamp"] = time.Now().Unix()

	// Redis metrics (if any)
	if s.redisStore != nil {
		stats := s.redisStore.GetStats()
		metrics["redis"] = stats
	}

	c.JSON(http.StatusOK, metrics)
}

func (s *Server) handlePrometheusMetrics(c *gin.Context) {
	metrics := fmt.Sprintf(`# HELP helios_requests_total Total number of requests
# TYPE helios_requests_total counter
helios_requests_total{method="GET",path="/allow"} %d

# HELP helios_rate_limits_total Total number of rate limit checks
# TYPE helios_rate_limits_total counter
helios_rate_limits_total{result="allowed"} %d
helios_rate_limits_total{result="denied"} %d

# HELP helios_up Whether the service is up
# TYPE helios_up gauge
helios_up 1
`,
        atomic.LoadUint64(&reqTotal),
        atomic.LoadUint64(&reqAllowed),
        atomic.LoadUint64(&reqDenied),
    )
	c.Header("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	c.String(http.StatusOK, metrics)
}

func (s *Server) unaryInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	start := time.Now()
	resp, err := handler(ctx, req)
	duration := time.Since(start)

	s.logger.Info("gRPC request completed",
		"method", info.FullMethod,
		"duration", duration,
		"error", err,
	)
	return resp, err
}




