package config

import (
	// "fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	Gateway       GatewayConfig       `yaml:"gateway"`
	Control       ControlConfig       `yaml:"control"`
	Redis         RedisConfig         `yaml:"redis"`
	Etcd          EtcdConfig          `yaml:"etcd"`
	Observability ObservabilityConfig `yaml:"observability"`
	Auth          AuthConfig          `yaml:"auth"`
	Resilience    ResilienceConfig    `yaml:"resilience"`
}

type GatewayConfig struct {
	Address         string        `yaml:"address"`
	GRPCAddress     string        `yaml:"grpc_address"`
	ReadTimeout     time.Duration `yaml:"read_timeout"`
	WriteTimeout    time.Duration `yaml:"write_timeout"`
	ShutdownTimeout time.Duration `yaml:"shutdown_timeout"`
	MaxRequestSize  int64         `yaml:"max_request_size"`
	ConsistencyMode string        `yaml:"consistency_mode"` // "fast" or "strong"
}

type ControlConfig struct {
	Address         string        `yaml:"address"`
	GRPCAddress     string        `yaml:"grpc_address"`
	ReadTimeout     time.Duration `yaml:"read_timeout"`
	WriteTimeout    time.Duration `yaml:"write_timeout"`
	ShutdownTimeout time.Duration `yaml:"shutdown_timeout"`
}

type RedisConfig struct {
	Address      string        `yaml:"address"`
	Password     string        `yaml:"password"`
	Database     int           `yaml:"database"`
	PoolSize     int           `yaml:"pool_size"`
	MinIdleConns int           `yaml:"min_idle_conns"`
	MaxRetries   int           `yaml:"max_retries"`
	DialTimeout  time.Duration `yaml:"dial_timeout"`
	ReadTimeout  time.Duration `yaml:"read_timeout"`
	WriteTimeout time.Duration `yaml:"write_timeout"`
}

type EtcdConfig struct {
	Endpoints   []string      `yaml:"endpoints"`
	DialTimeout time.Duration `yaml:"dial_timeout"`
	Username    string        `yaml:"username"`
	Password    string        `yaml:"password"`
}

type ObservabilityConfig struct {
	MetricsEnabled   bool   `yaml:"metrics_enabled"`
	MetricsAddress   string `yaml:"metrics_address"`
	TracingEnabled   bool   `yaml:"tracing_enabled"`
	JaegerEndpoint   string `yaml:"jaeger_endpoint"`
	ServiceName      string `yaml:"service_name"`
	ServiceVersion   string `yaml:"service_version"`
	LogLevel         string `yaml:"log_level"`
	EnableProfiling  bool   `yaml:"enable_profiling"`
	ProfilingAddress string `yaml:"profiling_address"`
}

type AuthConfig struct {
	JWTEnabled    bool          `yaml:"jwt_enabled"`
	JWTSecretKey  string        `yaml:"jwt_secret_key"`
	JWTIssuer     string        `yaml:"jwt_issuer"`
	JWTAudience   string        `yaml:"jwt_audience"`
	JWTExpiration time.Duration `yaml:"jwt_expiration"`
}

type ResilienceConfig struct {
	CircuitBreaker CircuitBreakerConfig `yaml:"circuit_breaker"`
	Bulkhead       BulkheadConfig       `yaml:"bulkhead"`
	LoadShedding   LoadSheddingConfig   `yaml:"load_shedding"`
	Retry          RetryConfig          `yaml:"retry"`
}

type CircuitBreakerConfig struct {
	Enabled              bool          `yaml:"enabled"`
	FailureThreshold     int           `yaml:"failure_threshold"`
	TimeoutDuration      time.Duration `yaml:"timeout_duration"`
	MaxConcurrentRequest int           `yaml:"max_concurrent_request"`
}

type BulkheadConfig struct {
	Enabled         bool `yaml:"enabled"`
	MaxConcurrency  int  `yaml:"max_concurrency"`
	QueueSize       int  `yaml:"queue_size"`
	TenantIsolation bool `yaml:"tenant_isolation"`
}

type LoadSheddingConfig struct {
	Enabled              bool          `yaml:"enabled"`
	CPUThreshold         float64       `yaml:"cpu_threshold"`
	MemoryThreshold      float64       `yaml:"memory_threshold"`
	LatencyThreshold     time.Duration `yaml:"latency_threshold"`
	QueueLengthThreshold int           `yaml:"queue_length_threshold"`
}

type RetryConfig struct {
	Enabled         bool          `yaml:"enabled"`
	MaxRetries      int           `yaml:"max_retries"`
	InitialInterval time.Duration `yaml:"initial_interval"`
	MaxInterval     time.Duration `yaml:"max_interval"`
	Multiplier      float64       `yaml:"multiplier"`
}

// LoadConfig loads configuration from environment variables with defaults
func LoadConfig() *Config {
	return &Config{
		Gateway: GatewayConfig{
			Address:         getEnv("HELIOS_GATEWAY_ADDRESS", ":8080"),
			GRPCAddress:     getEnv("HELIOS_GATEWAY_GRPC_ADDRESS", ":9080"),
			ReadTimeout:     getEnvDuration("HELIOS_GATEWAY_READ_TIMEOUT", 30*time.Second),
			WriteTimeout:    getEnvDuration("HELIOS_GATEWAY_WRITE_TIMEOUT", 30*time.Second),
			ShutdownTimeout: getEnvDuration("HELIOS_GATEWAY_SHUTDOWN_TIMEOUT", 30*time.Second),
			MaxRequestSize:  getEnvInt64("HELIOS_GATEWAY_MAX_REQUEST_SIZE", 1024*1024), // 1MB
			ConsistencyMode: getEnv("HELIOS_CONSISTENCY_MODE", "fast"),
		},
		Control: ControlConfig{
			Address:         getEnv("HELIOS_CONTROL_ADDRESS", ":8081"),
			GRPCAddress:     getEnv("HELIOS_CONTROL_GRPC_ADDRESS", ":9081"),
			ReadTimeout:     getEnvDuration("HELIOS_CONTROL_READ_TIMEOUT", 30*time.Second),
			WriteTimeout:    getEnvDuration("HELIOS_CONTROL_WRITE_TIMEOUT", 30*time.Second),
			ShutdownTimeout: getEnvDuration("HELIOS_CONTROL_SHUTDOWN_TIMEOUT", 30*time.Second),
		},
		Redis: RedisConfig{
			Address:      getEnv("HELIOS_REDIS_ADDRESS", "localhost:6379"),
			Password:     getEnv("HELIOS_REDIS_PASSWORD", ""),
			Database:     getEnvInt("HELIOS_REDIS_DATABASE", 0),
			PoolSize:     getEnvInt("HELIOS_REDIS_POOL_SIZE", 100),
			MinIdleConns: getEnvInt("HELIOS_REDIS_MIN_IDLE_CONNS", 10),
			MaxRetries:   getEnvInt("HELIOS_REDIS_MAX_RETRIES", 3),
			DialTimeout:  getEnvDuration("HELIOS_REDIS_DIAL_TIMEOUT", 5*time.Second),
			ReadTimeout:  getEnvDuration("HELIOS_REDIS_READ_TIMEOUT", 3*time.Second),
			WriteTimeout: getEnvDuration("HELIOS_REDIS_WRITE_TIMEOUT", 3*time.Second),
		},
		Etcd: EtcdConfig{
			Endpoints:   getEnvStringSlice("HELIOS_ETCD_ENDPOINTS", []string{"localhost:2379"}),
			DialTimeout: getEnvDuration("HELIOS_ETCD_DIAL_TIMEOUT", 5*time.Second),
			Username:    getEnv("HELIOS_ETCD_USERNAME", ""),
			Password:    getEnv("HELIOS_ETCD_PASSWORD", ""),
		},
		Observability: ObservabilityConfig{
			MetricsEnabled:   getEnvBool("HELIOS_METRICS_ENABLED", true),
			MetricsAddress:   getEnv("HELIOS_METRICS_ADDRESS", ":2112"),
			TracingEnabled:   getEnvBool("HELIOS_TRACING_ENABLED", false),
			JaegerEndpoint:   getEnv("HELIOS_JAEGER_ENDPOINT", "http://localhost:14268/api/traces"),
			ServiceName:      getEnv("HELIOS_SERVICE_NAME", "helios-gateway"),
			ServiceVersion:   getEnv("HELIOS_SERVICE_VERSION", "dev"),
			LogLevel:         getEnv("HELIOS_LOG_LEVEL", "info"),
			EnableProfiling:  getEnvBool("HELIOS_ENABLE_PROFILING", false),
			ProfilingAddress: getEnv("HELIOS_PROFILING_ADDRESS", ":6060"),
		},
		Auth: AuthConfig{
			JWTEnabled:    getEnvBool("HELIOS_JWT_ENABLED", false),
			JWTSecretKey:  getEnv("HELIOS_JWT_SECRET_KEY", ""),
			JWTIssuer:     getEnv("HELIOS_JWT_ISSUER", "helios"),
			JWTAudience:   getEnv("HELIOS_JWT_AUDIENCE", "helios-api"),
			JWTExpiration: getEnvDuration("HELIOS_JWT_EXPIRATION", 24*time.Hour),
		},
		Resilience: ResilienceConfig{
			CircuitBreaker: CircuitBreakerConfig{
				Enabled:              getEnvBool("HELIOS_CIRCUIT_BREAKER_ENABLED", true),
				FailureThreshold:     getEnvInt("HELIOS_CIRCUIT_BREAKER_FAILURE_THRESHOLD", 10),
				TimeoutDuration:      getEnvDuration("HELIOS_CIRCUIT_BREAKER_TIMEOUT", 60*time.Second),
				MaxConcurrentRequest: getEnvInt("HELIOS_CIRCUIT_BREAKER_MAX_CONCURRENT", 100),
			},
			Bulkhead: BulkheadConfig{
				Enabled:         getEnvBool("HELIOS_BULKHEAD_ENABLED", true),
				MaxConcurrency:  getEnvInt("HELIOS_BULKHEAD_MAX_CONCURRENCY", 1000),
				QueueSize:       getEnvInt("HELIOS_BULKHEAD_QUEUE_SIZE", 10000),
				TenantIsolation: getEnvBool("HELIOS_BULKHEAD_TENANT_ISOLATION", true),
			},
			LoadShedding: LoadSheddingConfig{
				Enabled:              getEnvBool("HELIOS_LOAD_SHEDDING_ENABLED", true),
				CPUThreshold:         getEnvFloat64("HELIOS_LOAD_SHEDDING_CPU_THRESHOLD", 0.8),
				MemoryThreshold:      getEnvFloat64("HELIOS_LOAD_SHEDDING_MEMORY_THRESHOLD", 0.85),
				LatencyThreshold:     getEnvDuration("HELIOS_LOAD_SHEDDING_LATENCY_THRESHOLD", 100*time.Millisecond),
				QueueLengthThreshold: getEnvInt("HELIOS_LOAD_SHEDDING_QUEUE_THRESHOLD", 1000),
			},
			Retry: RetryConfig{
				Enabled:         getEnvBool("HELIOS_RETRY_ENABLED", true),
				MaxRetries:      getEnvInt("HELIOS_RETRY_MAX_RETRIES", 3),
				InitialInterval: getEnvDuration("HELIOS_RETRY_INITIAL_INTERVAL", 100*time.Millisecond),
				MaxInterval:     getEnvDuration("HELIOS_RETRY_MAX_INTERVAL", 5*time.Second),
				Multiplier:      getEnvFloat64("HELIOS_RETRY_MULTIPLIER", 2.0),
			},
		},
	}
}

// Helper functions for environment variable parsing
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getEnvInt64(key string, defaultValue int64) int64 {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.ParseInt(value, 10, 64); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getEnvFloat64(key string, defaultValue float64) float64 {
	if value := os.Getenv(key); value != "" {
		if floatValue, err := strconv.ParseFloat(value, 64); err == nil {
			return floatValue
		}
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return boolValue
		}
	}
	return defaultValue
}

func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return defaultValue
}

func getEnvStringSlice(key string, defaultValue []string) []string {
	if value := os.Getenv(key); value != "" {
		return []string{value} // Simplified - could parse comma-separated values
	}
	return defaultValue
}
