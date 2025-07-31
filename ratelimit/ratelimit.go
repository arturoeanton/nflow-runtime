package ratelimit

import (
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/arturoeanton/nflow-runtime/engine"
	"github.com/arturoeanton/nflow-runtime/logger"
	"github.com/go-redis/redis"
)

// RateLimiter interface defines the rate limiting operations
type RateLimiter interface {
	// AllowIP checks if an IP address is allowed to make a request
	AllowIP(ip string) (allowed bool, retryAfter time.Duration)

	// Reset resets the rate limiter for an IP
	ResetIP(ip string)

	// Close cleans up resources
	Close()
}

// NewRateLimiter creates a new rate limiter based on configuration
func NewRateLimiter(config *engine.RateLimitConfig, redisClient *redis.Client) RateLimiter {
	if !config.Enabled {
		return &noopRateLimiter{}
	}

	switch config.Backend {
	case "redis":
		if redisClient == nil {
			logger.Error("Redis client not available, falling back to memory backend")
			return newMemoryRateLimiter(config)
		}
		return newRedisRateLimiter(config, redisClient)
	default:
		return newMemoryRateLimiter(config)
	}
}

// noopRateLimiter is used when rate limiting is disabled
type noopRateLimiter struct{}

func (n *noopRateLimiter) AllowIP(ip string) (bool, time.Duration) {
	return true, 0
}

func (n *noopRateLimiter) ResetIP(ip string) {}

func (n *noopRateLimiter) Close() {}

// memoryRateLimiter implements in-memory rate limiting
type memoryRateLimiter struct {
	config        *engine.RateLimitConfig
	ipBuckets     map[string]*bucket
	mu            sync.RWMutex
	cleanupTicker *time.Ticker
	done          chan struct{}
}

// bucket represents a token bucket for rate limiting
type bucket struct {
	tokens   int
	lastFill time.Time
	mu       sync.Mutex
}

func newMemoryRateLimiter(config *engine.RateLimitConfig) RateLimiter {
	rl := &memoryRateLimiter{
		config:    config,
		ipBuckets: make(map[string]*bucket),
		done:      make(chan struct{}),
	}

	// Start cleanup routine
	cleanupInterval := time.Duration(config.CleanupInterval) * time.Minute
	if cleanupInterval <= 0 {
		cleanupInterval = 10 * time.Minute
	}

	rl.cleanupTicker = time.NewTicker(cleanupInterval)
	go rl.cleanup()

	return rl
}

func (m *memoryRateLimiter) AllowIP(ip string) (bool, time.Duration) {
	limit := m.config.IPRateLimit
	window := time.Duration(m.config.IPWindowMinutes) * time.Minute
	burst := m.config.IPBurstSize

	m.mu.Lock()
	b, exists := m.ipBuckets[ip]
	if !exists {
		b = &bucket{
			tokens:   limit,
			lastFill: time.Now(),
		}
		m.ipBuckets[ip] = b
	}
	m.mu.Unlock()

	return m.allowFromBucket(b, limit, window, burst)
}

func (m *memoryRateLimiter) allowFromBucket(b *bucket, limit int, window time.Duration, burst int) (bool, time.Duration) {
	b.mu.Lock()
	defer b.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(b.lastFill)

	// Refill tokens based on elapsed time
	tokensToAdd := int(elapsed / window * time.Duration(limit))
	if tokensToAdd > 0 {
		b.tokens = min(b.tokens+tokensToAdd, limit+burst)
		b.lastFill = now
	}

	// Check if we have tokens available
	if b.tokens > 0 {
		b.tokens--
		return true, 0
	}

	// Calculate retry after
	retryAfter := window - elapsed%window
	return false, retryAfter
}

func (m *memoryRateLimiter) ResetIP(ip string) {
	m.mu.Lock()
	delete(m.ipBuckets, ip)
	m.mu.Unlock()
}

func (m *memoryRateLimiter) Close() {
	close(m.done)
	m.cleanupTicker.Stop()
}

func (m *memoryRateLimiter) cleanup() {
	for {
		select {
		case <-m.cleanupTicker.C:
			m.cleanupBuckets()
		case <-m.done:
			return
		}
	}
}

func (m *memoryRateLimiter) cleanupBuckets() {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	windowIP := time.Duration(m.config.IPWindowMinutes) * time.Minute

	// Clean up IP buckets
	for ip, b := range m.ipBuckets {
		b.mu.Lock()
		if now.Sub(b.lastFill) > windowIP*2 {
			delete(m.ipBuckets, ip)
		}
		b.mu.Unlock()
	}

	logger.Verbosef("Rate limiter cleanup: %d IP buckets", len(m.ipBuckets))
}

// redisRateLimiter implements Redis-based rate limiting
type redisRateLimiter struct {
	config      *engine.RateLimitConfig
	redisClient *redis.Client
}

func newRedisRateLimiter(config *engine.RateLimitConfig, redisClient *redis.Client) RateLimiter {
	return &redisRateLimiter{
		config:      config,
		redisClient: redisClient,
	}
}

func (r *redisRateLimiter) AllowIP(ip string) (bool, time.Duration) {
	key := fmt.Sprintf("ratelimit:ip:%s", ip)
	limit := r.config.IPRateLimit
	window := time.Duration(r.config.IPWindowMinutes) * time.Minute

	return r.checkLimit(key, limit, window)
}

func (r *redisRateLimiter) checkLimit(key string, limit int, window time.Duration) (bool, time.Duration) {
	now := time.Now()
	windowStart := now.Add(-window)

	// Remove old entries
	r.redisClient.ZRemRangeByScore(key, "0", fmt.Sprintf("%d", windowStart.Unix()))

	// Count current entries
	count, err := r.redisClient.ZCard(key).Result()
	if err != nil {
		logger.Error("Redis rate limit error:", err)
		return true, 0 // Fail open
	}

	if count >= int64(limit) {
		// Get oldest entry to calculate retry after
		oldest, err := r.redisClient.ZRange(key, 0, 0).Result()
		if err == nil && len(oldest) > 0 {
			// Calculate retry after based on oldest entry
			retryAfter := window - now.Sub(windowStart)
			return false, retryAfter
		}
		return false, window
	}

	// Add current request
	r.redisClient.ZAdd(key, redis.Z{
		Score:  float64(now.Unix()),
		Member: now.UnixNano(),
	})
	r.redisClient.Expire(key, window)

	return true, 0
}

func (r *redisRateLimiter) ResetIP(ip string) {
	key := fmt.Sprintf("ratelimit:ip:%s", ip)
	r.redisClient.Del(key)
}

func (r *redisRateLimiter) Close() {
	// Redis client is managed externally
}

// Helper functions

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// IsIPExcluded checks if an IP is in the exclusion list
func IsIPExcluded(ip string, excludedIPs string) bool {
	if excludedIPs == "" {
		return false
	}

	clientIP := net.ParseIP(ip)
	if clientIP == nil {
		return false
	}

	for _, excluded := range strings.Split(excludedIPs, ",") {
		excluded = strings.TrimSpace(excluded)
		if excluded == "" {
			continue
		}

		// Check for CIDR notation
		if strings.Contains(excluded, "/") {
			_, ipNet, err := net.ParseCIDR(excluded)
			if err == nil && ipNet.Contains(clientIP) {
				return true
			}
		} else {
			// Direct IP comparison
			if excluded == ip {
				return true
			}
		}
	}

	return false
}

// IsPathExcluded checks if a path is in the exclusion list
func IsPathExcluded(path string, excludedPaths string) bool {
	if excludedPaths == "" {
		return false
	}

	for _, excluded := range strings.Split(excludedPaths, ",") {
		excluded = strings.TrimSpace(excluded)
		if excluded != "" && strings.HasPrefix(path, excluded) {
			return true
		}
	}

	return false
}
