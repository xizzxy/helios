package control

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"	
	"time"

	"github.com/gin-gonic/gin"
	clientv3 "go.etcd.io/etcd/client/v3"

	"github.com/xizzxy/helios/internal/config"
)

type Server struct {
	config     *config.Config
	logger     *slog.Logger
	etcd       *clientv3.Client
	httpServer *http.Server
}

type TenantConfig struct {
	TenantID  string           `json:"tenant_id"`
	Limits    map[string]Limit `json:"limits"`
	APIKeys   []string         `json:"api_keys"`
	Algorithm string           `json:"algorithm"` // "token_bucket" or "sliding_window"
	Mode      string           `json:"mode"`      // "fast" or "strong"
	Created   time.Time        `json:"created"`
	Updated   time.Time        `json:"updated"`
}

type Limit struct {
	Limit  int64         `json:"limit"`
	Window time.Duration `json:"window"`
	Burst  int64         `json:"burst"`
}

func NewServer(cfg *config.Config, logger *slog.Logger) (*Server, error) {
	// Connect to etcd
	etcdClient, err := clientv3.New(clientv3.Config{
		Endpoints:   cfg.Etcd.Endpoints,
		DialTimeout: cfg.Etcd.DialTimeout,
		Username:    cfg.Etcd.Username,
		Password:    cfg.Etcd.Password,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to etcd: %w", err)
	}

	return &Server{
		config: cfg,
		logger: logger,
		etcd:   etcdClient,
	}, nil
}

func (s *Server) Start(ctx context.Context) error {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())

	// Add logging middleware
	router.Use(func(c *gin.Context) {
		start := time.Now()
		c.Next()

		s.logger.Info("HTTP request",
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", c.Writer.Status(),
			"duration", time.Since(start),
			"remote_addr", c.ClientIP(),
		)
	})

	// Health endpoint
	router.GET("/health", s.healthHandler)

	// Tenant management API
	api := router.Group("/api/v1")
	{
		api.POST("/tenants", s.createTenant)
		api.GET("/tenants/:tenant_id", s.getTenant)
		api.PUT("/tenants/:tenant_id", s.updateTenant)
		api.DELETE("/tenants/:tenant_id", s.deleteTenant)
		api.GET("/tenants", s.listTenants)
	}

	s.httpServer = &http.Server{
		Addr:         s.config.Control.Address,
		Handler:      router,
		ReadTimeout:  s.config.Control.ReadTimeout,
		WriteTimeout: s.config.Control.WriteTimeout,
	}

	go func() {
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.logger.Error("HTTP server error", "error", err)
		}
	}()

	s.logger.Info("Control plane server started", "address", s.config.Control.Address)
	return nil
}

func (s *Server) Stop(ctx context.Context) error {
	s.logger.Info("Shutting down control plane server")

	if s.httpServer != nil {
		if err := s.httpServer.Shutdown(ctx); err != nil {
			return fmt.Errorf("failed to shutdown HTTP server: %w", err)
		}
	}

	if s.etcd != nil {
		return s.etcd.Close()
	}

	return nil
}

func (s *Server) healthHandler(c *gin.Context) {
	// Check etcd connectivity
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	_, err := s.etcd.Status(ctx, s.config.Etcd.Endpoints[0])
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status": "unhealthy",
			"error":  "etcd connectivity issue",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":    "healthy",
		"service":   "helios-control",
		"version":   s.config.Observability.ServiceVersion,
		"timestamp": time.Now().UTC(),
	})
}

func (s *Server) createTenant(c *gin.Context) {
	var tenantConfig TenantConfig
	if err := c.ShouldBindJSON(&tenantConfig); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tenantConfig.Created = time.Now().UTC()
	tenantConfig.Updated = tenantConfig.Created

	// Set defaults if not provided
	if tenantConfig.Algorithm == "" {
		tenantConfig.Algorithm = "token_bucket"
	}
	if tenantConfig.Mode == "" {
		tenantConfig.Mode = "fast"
	}
	if tenantConfig.Limits == nil {
		tenantConfig.Limits = map[string]Limit{
			"default": {
				Limit:  100,
				Window: time.Minute,
				Burst:  120,
			},
		}
	}

	// Store in etcd
	key := fmt.Sprintf("/helios/tenants/%s", tenantConfig.TenantID)
	data, err := json.Marshal(tenantConfig)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to marshal config"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	_, err = s.etcd.Put(ctx, key, string(data))
	if err != nil {
		s.logger.Error("Failed to store tenant config", "tenant_id", tenantConfig.TenantID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to store config"})
		return
	}

	c.JSON(http.StatusCreated, tenantConfig)
}

func (s *Server) getTenant(c *gin.Context) {
	tenantID := c.Param("tenant_id")

	key := fmt.Sprintf("/helios/tenants/%s", tenantID)
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	resp, err := s.etcd.Get(ctx, key)
	if err != nil {
		s.logger.Error("Failed to get tenant config", "tenant_id", tenantID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve config"})
		return
	}

	if len(resp.Kvs) == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "tenant not found"})
		return
	}

	var tenantConfig TenantConfig
	if err := json.Unmarshal(resp.Kvs[0].Value, &tenantConfig); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to parse config"})
		return
	}

	c.JSON(http.StatusOK, tenantConfig)
}

func (s *Server) updateTenant(c *gin.Context) {
	tenantID := c.Param("tenant_id")

	var updates TenantConfig
	if err := c.ShouldBindJSON(&updates); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get existing config
	key := fmt.Sprintf("/helios/tenants/%s", tenantID)
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	resp, err := s.etcd.Get(ctx, key)
	if err != nil {
		s.logger.Error("Failed to get tenant config", "tenant_id", tenantID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve config"})
		return
	}

	if len(resp.Kvs) == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "tenant not found"})
		return
	}

	var tenantConfig TenantConfig
	if err := json.Unmarshal(resp.Kvs[0].Value, &tenantConfig); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to parse config"})
		return
	}

	// Update fields
	if updates.Limits != nil {
		tenantConfig.Limits = updates.Limits
	}
	if updates.APIKeys != nil {
		tenantConfig.APIKeys = updates.APIKeys
	}
	if updates.Algorithm != "" {
		tenantConfig.Algorithm = updates.Algorithm
	}
	if updates.Mode != "" {
		tenantConfig.Mode = updates.Mode
	}
	tenantConfig.Updated = time.Now().UTC()

	// Store updated config
	data, err := json.Marshal(tenantConfig)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to marshal config"})
		return
	}

	_, err = s.etcd.Put(ctx, key, string(data))
	if err != nil {
		s.logger.Error("Failed to update tenant config", "tenant_id", tenantID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update config"})
		return
	}

	c.JSON(http.StatusOK, tenantConfig)
}

func (s *Server) deleteTenant(c *gin.Context) {
	tenantID := c.Param("tenant_id")

	key := fmt.Sprintf("/helios/tenants/%s", tenantID)
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	_, err := s.etcd.Delete(ctx, key)
	if err != nil {
		s.logger.Error("Failed to delete tenant config", "tenant_id", tenantID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete config"})
		return
	}

	c.JSON(http.StatusNoContent, nil)
}

func (s *Server) listTenants(c *gin.Context) {
	prefix := "/helios/tenants/"
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	resp, err := s.etcd.Get(ctx, prefix, clientv3.WithPrefix())
	if err != nil {
		s.logger.Error("Failed to list tenants", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list tenants"})
		return
	}

	tenants := make([]TenantConfig, 0, len(resp.Kvs))
	for _, kv := range resp.Kvs {
		var tenantConfig TenantConfig
		if err := json.Unmarshal(kv.Value, &tenantConfig); err != nil {
			s.logger.Warn("Failed to parse tenant config", "key", string(kv.Key), "error", err)
			continue
		}
		tenants = append(tenants, tenantConfig)
	}

	c.JSON(http.StatusOK, gin.H{
		"tenants": tenants,
		"count":   len(tenants),
	})
}


