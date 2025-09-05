package limiter

import "sync"

// LocalManager hands out one limiter config for now.
type LocalManager struct {
	mu      sync.RWMutex
	limiter Limiter
	cfg     Config
}

func NewLocalManager(defaultCfg Config) *LocalManager {
	var limiter Limiter
	switch defaultCfg.Algorithm {
	case AlgoSlidingWindow:
		limiter = NewSlidingWindowLimiter(defaultCfg)
	default:
		limiter = NewTokenBucketLimiter(defaultCfg)
	}

	return &LocalManager{
		cfg:     defaultCfg,
		limiter: limiter,
	}
}

func (m *LocalManager) ForTenant(tenant string) Limiter {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.limiter
}

// GetLimiter returns a limiter for the given tenant and resource
func (m *LocalManager) GetLimiter(tenant, resource string) (Limiter, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.limiter, nil
}



