# Helios Design Document

## Overview

Helios is designed as a high-performance, distributed rate limiter and API gateway that can handle 100k+ requests per second while maintaining sub-20ms latency. This document outlines the architectural decisions, trade-offs, and design principles that guide the implementation.

## Design Goals

### Primary Goals
1. **High Performance**: 100k+ RPS with p99 < 20ms latency
2. **Horizontal Scalability**: Linear scaling across multiple instances
3. **Reliability**: 99.9% uptime with graceful degradation
4. **Flexibility**: Support multiple rate limiting algorithms and deployment patterns
5. **Observability**: Deep insights into system behavior and performance

### Secondary Goals
1. **Developer Experience**: Simple APIs and comprehensive tooling
2. **Operational Excellence**: Easy deployment and maintenance
3. **Cost Efficiency**: Optimal resource utilization
4. **Security**: Built-in security best practices

## Architecture Overview

### System Architecture

```
┌─────────────────┐
│   Load Balancer │
│   (HAProxy/ALB) │
└─────────┬───────┘
          │
    ┌─────▼─────┐
    │  Gateway  │  ◄─── Stateless, horizontally scalable
    │ Instances │
    └─────┬─────┘
          │
    ┌─────▼──────┐      ┌─────────────────┐
    │   Redis    │      │ Control Plane   │
    │  Cluster   │ ◄──► │   (Config)      │
    └────────────┘      └─────────────────┘
          │                       │
    ┌─────▼──────┐      ┌─────────▼───────┐
    │    etcd    │      │   Monitoring    │
    │  (Config)  │      │ (Prom/Grafana)  │
    └────────────┘      └─────────────────┘
```

### Component Architecture

```
┌─────────────────────────────────────────┐
│              Gateway Instance            │
├─────────────────────────────────────────┤
│  HTTP/gRPC Server (Gin + gRPC)          │
├─────────────────────────────────────────┤
│  Middleware Stack                       │
│  ├─ CORS, Auth, Logging                 │
│  ├─ Rate Limiting                       │
│  ├─ Circuit Breaker                     │
│  └─ Load Shedding                       │
├─────────────────────────────────────────┤
│  Rate Limiter Engine                    │
│  ├─ Algorithm Manager                   │
│  ├─ Token Bucket                        │
│  ├─ Sliding Window                      │
│  └─ Leaky Bucket                        │
├─────────────────────────────────────────┤
│  Storage Layer                          │
│  ├─ Local Cache (FAST mode)             │
│  └─ Redis Client (STRONG mode)          │
├─────────────────────────────────────────┤
│  Configuration                          │
│  ├─ Static Config                       │
│  └─ Dynamic Config (etcd watcher)       │
├─────────────────────────────────────────┤
│  Observability                          │
│  ├─ Metrics (Prometheus)                │
│  ├─ Tracing (OpenTelemetry)             │
│  └─ Logging (structured JSON)           │
└─────────────────────────────────────────┘
```

## Key Design Decisions

### 1. Dual Consistency Modes

**Decision**: Implement both FAST (local cache) and STRONG (Redis atomic) consistency modes.

**Rationale**:
- **FAST Mode**: Optimizes for latency and throughput. Uses local in-memory rate limiters with periodic synchronization. Suitable for most use cases where slight over-limit is acceptable.
- **STRONG Mode**: Provides strict rate limit enforcement using Redis atomic operations. Required for billing/quota scenarios where precision is critical.

**Trade-offs**:
| Aspect | FAST Mode | STRONG Mode |
|--------|-----------|-------------|
| Latency | 2-8ms p99 | 8-20ms p99 |
| Throughput | 120k+ RPS | 85k+ RPS |
| Consistency | Eventual | Strong |
| Network Dependency | Low | High |
| Resource Usage | Higher CPU | Higher Network |

### 2. Multi-Algorithm Support

**Decision**: Support multiple rate limiting algorithms (Token Bucket, Sliding Window, Leaky Bucket).

**Rationale**:
- Different use cases require different characteristics
- Token Bucket: Best for burst handling
- Sliding Window: Precise time-based limits
- Leaky Bucket: Smooth traffic shaping

**Implementation**: Strategy pattern with pluggable algorithm implementations.

### 3. Middleware-Based Architecture

**Decision**: Build the gateway using a composable middleware stack.

**Rationale**:
- **Modularity**: Easy to add/remove features
- **Testability**: Each middleware can be tested independently
- **Performance**: Minimal overhead with early short-circuiting
- **Flexibility**: Different middleware chains for different routes

### 4. Redis Lua Scripts for Atomicity

**Decision**: Use Lua scripts in Redis for atomic rate limiting operations.

**Rationale**:
- Ensures atomic read-modify-write operations
- Reduces network round-trips (single Redis call)
- Prevents race conditions in distributed environments
- Maintains consistency across multiple gateway instances

**Example Token Bucket Script**:
```lua
-- Atomic token bucket implementation
local key = KEYS[1]
local limit = tonumber(ARGV[1])
local window = tonumber(ARGV[2])
local cost = tonumber(ARGV[3])

local bucket = redis.call('HMGET', key, 'tokens', 'last_refill')
local tokens = tonumber(bucket[1]) or limit
local last_refill = tonumber(bucket[2]) or redis.call('TIME')[1]

-- Refill logic and consumption check
-- ... (full implementation in codebase)
```

### 5. etcd for Configuration Management

**Decision**: Use etcd for dynamic configuration management with local caching.

**Rationale**:
- **Consistency**: Strong consistency for critical configuration
- **Watch API**: Real-time configuration updates
- **Reliability**: Proven in production (Kubernetes uses etcd)
- **Performance**: Local caching minimizes lookup latency

### 6. Comprehensive Observability

**Decision**: Built-in support for metrics, tracing, and logging.

**Rationale**:
- **Prometheus Metrics**: Industry standard, pull-based model
- **OpenTelemetry Tracing**: Vendor-agnostic distributed tracing
- **Structured Logging**: Machine-readable, searchable logs
- **Health Endpoints**: Deep health checks for load balancers

### 7. Resilience Patterns

**Decision**: Implement circuit breaker, bulkhead, load shedding, and retry patterns.

**Rationale**:
- **Circuit Breaker**: Prevents cascade failures
- **Bulkhead**: Isolates tenant workloads
- **Load Shedding**: Graceful degradation under pressure
- **Retries**: Handle transient failures

## Performance Optimizations

### 1. Memory Pool Management

```go
// Pre-allocated pools for frequent allocations
var responsePool = sync.Pool{
    New: func() interface{} {
        return &Response{}
    },
}

func getResponse() *Response {
    return responsePool.Get().(*Response)
}

func putResponse(resp *Response) {
    resp.Reset()
    responsePool.Put(resp)
}
```

### 2. Connection Pooling

- Redis: Configured with optimal pool size and connection reuse
- HTTP: Keep-alive connections and proper timeouts
- gRPC: Connection multiplexing and load balancing

### 3. Lock-Free Data Structures

- Rate limiter state uses atomic operations where possible
- Lock-free counters for high-frequency operations
- Copy-on-write for configuration updates

### 4. Efficient Serialization

- protobuf for gRPC (binary, schema evolution)
- JSON for REST (human readable, widely supported)
- MessagePack for internal communication (compact, fast)

## Scalability Considerations

### Horizontal Scaling

**Gateway Instances**:
- Stateless design enables unlimited horizontal scaling
- Load balancer distributes requests across instances
- Instance discovery via service registry

**Redis Scaling**:
- Redis Cluster for horizontal data partitioning
- Read replicas for read-heavy workloads
- Consistent hashing for even distribution

### Vertical Scaling

**CPU Optimization**:
- Goroutine pools to prevent goroutine explosion
- CPU-aware concurrency limits
- Profile-guided optimization (PGO) in Go 1.21+

**Memory Optimization**:
- Object pooling for frequent allocations
- TTL-based eviction for local caches
- Memory-mapped files for large datasets

## Security Design

### 1. Defense in Depth

- **Network**: VPC isolation, security groups
- **Transport**: TLS 1.3 for all external communication
- **Application**: Input validation, rate limiting, authentication
- **Data**: Encryption at rest and in transit

### 2. API Security

- **Authentication**: API key based authentication
- **Authorization**: Role-based access control (RBAC)
- **Rate Limiting**: Prevent abuse and DoS attacks
- **Input Validation**: Comprehensive request validation

### 3. Secrets Management

- All secrets via environment variables or secret management systems
- No hardcoded credentials or keys
- Regular secret rotation support
- Audit logging for secret access

## Operational Considerations

### 1. Deployment Strategy

- **Blue-Green Deployment**: Zero-downtime updates
- **Canary Releases**: Gradual rollout with monitoring
- **Circuit Breakers**: Automatic rollback on failures
- **Health Checks**: Deep health validation

### 2. Monitoring and Alerting

**Key Metrics**:
- Request rate, latency, error rate (RED metrics)
- Saturation metrics (CPU, memory, connections)
- Business metrics (rate limit hit ratios, tenant usage)

**Alerting Rules**:
- P99 latency > 100ms for 2 minutes
- Error rate > 1% for 5 minutes
- Redis connection failures
- Memory usage > 85%

### 3. Disaster Recovery

- **Multi-region deployment**: Active-passive setup
- **Data replication**: Redis cluster with cross-region replication
- **Backup strategy**: Regular etcd snapshots
- **Recovery procedures**: Automated failover and manual recovery steps

## Future Considerations

### 1. Machine Learning Integration

- **Anomaly Detection**: Unusual traffic patterns
- **Predictive Scaling**: Proactive capacity management
- **Intelligent Rate Limiting**: Dynamic limits based on patterns

### 2. Advanced Features

- **GraphQL Support**: Native GraphQL rate limiting
- **WebSocket Support**: Real-time connection limiting
- **gRPC Streaming**: Stream-aware rate limiting

### 3. Multi-Cloud Support

- **Cloud-agnostic design**: Support AWS, GCP, Azure
- **Managed services integration**: Cloud-native storage options
- **Cost optimization**: Multi-cloud cost analysis

## Conclusion

Helios is designed with performance, reliability, and scalability as primary concerns while maintaining simplicity and operational excellence. The architecture supports both current requirements and future growth, with clear extension points for additional features.

The dual consistency modes provide flexibility for different use cases, while comprehensive observability ensures operational visibility. The modular design allows for easy testing and maintenance, and the resilience patterns provide confidence in production environments.

This design document serves as a living guide for development and operational decisions, and should be updated as the system evolves.