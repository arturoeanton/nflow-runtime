package ratelimit

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/arturoeanton/nflow-runtime/engine"
	"github.com/arturoeanton/nflow-runtime/logger"
	"github.com/labstack/echo/v4"
)

// Middleware returns an Echo middleware function for rate limiting
func Middleware(config *engine.RateLimitConfig, rateLimiter RateLimiter) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// Skip if rate limiting is disabled
			if !config.Enabled {
				return next(c)
			}

			// Check if path is excluded
			path := c.Request().URL.Path
			if IsPathExcluded(path, config.ExcludedPaths) {
				return next(c)
			}

			// Get client IP
			ip := getClientIP(c)
			
			// Check if IP is excluded
			if IsIPExcluded(ip, config.ExcludedIPs) {
				return next(c)
			}

			// Check rate limit
			allowed, retryAfter := rateLimiter.AllowIP(ip)
			
			if !allowed {
				// Log rate limit exceeded
				logger.Verbosef("Rate limit exceeded for IP: %s, path: %s", ip, path)
				
				// Set headers
				if config.RetryAfterHeader && retryAfter > 0 {
					c.Response().Header().Set("Retry-After", strconv.Itoa(int(retryAfter.Seconds())))
				}
				c.Response().Header().Set("X-RateLimit-Limit", strconv.Itoa(config.IPRateLimit))
				c.Response().Header().Set("X-RateLimit-Remaining", "0")
				c.Response().Header().Set("X-RateLimit-Reset", strconv.FormatInt(time.Now().Add(retryAfter).Unix(), 10))
				
				// Return error response
				message := config.ErrorMessage
				if message == "" {
					message = "Rate limit exceeded. Please try again later."
				}
				
				return c.JSON(http.StatusTooManyRequests, map[string]interface{}{
					"error": message,
					"retry_after": int(retryAfter.Seconds()),
				})
			}

			// Continue to next handler
			return next(c)
		}
	}
}

// getClientIP extracts the client IP from the request
func getClientIP(c echo.Context) string {
	// Check X-Real-IP header first
	ip := c.Request().Header.Get("X-Real-IP")
	if ip != "" {
		return ip
	}
	
	// Check X-Forwarded-For header
	xff := c.Request().Header.Get("X-Forwarded-For")
	if xff != "" {
		// Take the first IP if there are multiple
		if idx := strings.Index(xff, ","); idx != -1 {
			return strings.TrimSpace(xff[:idx])
		}
		return strings.TrimSpace(xff)
	}
	
	// Fall back to RemoteAddr
	ip = c.Request().RemoteAddr
	
	// Remove port if present
	if idx := strings.LastIndex(ip, ":"); idx != -1 {
		// Check if it's IPv6
		if strings.Count(ip, ":") > 1 {
			// IPv6 - only remove port if it's [::1]:port format
			if strings.HasPrefix(ip, "[") && strings.Contains(ip, "]:") {
				if endIdx := strings.LastIndex(ip, "]:"); endIdx != -1 {
					return ip[1:endIdx]
				}
			}
			return ip
		} else {
			// IPv4 - remove port
			return ip[:idx]
		}
	}
	
	return ip
}