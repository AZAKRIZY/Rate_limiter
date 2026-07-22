package limiter

import (
	"errors"
	"sync"
	"time"
)

var (
	ErrInvalidMaxTokens  = errors.New("limiter: maxTokens must not be negative")
	ErrInvalidRefillRate = errors.New("limiter: refillRate must be greater than 0")
	ErrInvalidDailyLimit = errors.New("limiter: dailyLimit must not be negative")
)

// UserLimiter combines a token bucket (burst control) with a daily quota
// (long-term usage cap) for a single user identity.
type UserLimiter struct {
	mu sync.Mutex

	// token bucket
	tokens         float64
	maxTokens      float64   // e.g. 100 tokens
	refillRate     float64   // tokens added per second (e.g. 5.0 == 1 token/200ms)
	lastRefillTime time.Time

	// daily limit
	dailyLimit    int
	dailyCount    int // e.g. capped at 4000
	lastResetTime time.Time
}

// NewUserLimiter creates a UserLimiter starting with a full bucket and a
func NewUserLimiter(maxTokens float64, refillRate float64, dailyLimit int) (*UserLimiter, error) {
	if maxTokens < 0 {
		return nil, ErrInvalidMaxTokens
	}
	if refillRate <= 0 {
		return nil, ErrInvalidRefillRate
	}
	if dailyLimit < 0 {
		return nil, ErrInvalidDailyLimit
	}

	now := time.Now()
	return &UserLimiter{
		tokens:         maxTokens, // start full, so a new user isn't throttled immediately
		maxTokens:      maxTokens,
		refillRate:     refillRate,
		lastRefillTime: now,

		dailyLimit:    dailyLimit,
		dailyCount:    0,
		lastResetTime: now,
	}, nil
}

// Allow reports whether a request should be permitted right now. It is safe
// to call concurrently from multiple goroutines.
func (ul *UserLimiter) Allow() bool {
	ul.mu.Lock()
	defer ul.mu.Unlock()

	now := time.Now()

	ul.resetDailyIfNeeded(now)
	if ul.dailyCount >= ul.dailyLimit {
		return false
	}

	ul.refill(now)
	if ul.tokens < 1 {
		return false
	}

	ul.tokens--
	ul.dailyCount++
	return true
}

// refill tops up the token bucket based on elapsed time since the last
// refill, capped at maxTokens. Must be called with mu already held.
func (ul *UserLimiter) refill(now time.Time) {
	elapsed := now.Sub(ul.lastRefillTime).Seconds()
	if elapsed <= 0 {
		return
	}

	tokensToAdd := elapsed * ul.refillRate
	ul.tokens += tokensToAdd
	if ul.tokens > ul.maxTokens {
		ul.tokens = ul.maxTokens
	}

	ul.lastRefillTime = now
}

// resetDailyIfNeeded resets the daily counter if 24 hours have passed since
func (ul *UserLimiter) resetDailyIfNeeded(now time.Time) {
	if now.Sub(ul.lastResetTime) >= 24*time.Hour {
		ul.dailyCount = 0
		ul.lastResetTime = now
	}
}

// Limiter manages one UserLimiter per user identity.
type Limiter struct {
	mu    sync.Mutex
	users map[string]*UserLimiter

	maxTokens  float64
	refillRate float64
	dailyLimit int
}

// NewLimiter creates a multi-user limiter. Every user gets the same
func NewLimiter(maxTokens, refillRate float64, dailyLimit int) (*Limiter, error) {
		if _, err := NewUserLimiter(maxTokens, refillRate, dailyLimit); err != nil {
		return nil, err
	}

	return &Limiter{
		users:      make(map[string]*UserLimiter),
		maxTokens:  maxTokens,
		refillRate: refillRate,
		dailyLimit: dailyLimit,
	}, nil
}

// Allow reports whether a request from userID should be permitted right now.
// A new UserLimiter is created on first use.
func (l *Limiter) Allow(userID string) bool {
	l.mu.Lock()
	ul, exists := l.users[userID]
	if !exists {
		// Error ignored: config was already validated in NewLimiter, so this
		// can't fail in practice.
		ul, _ = NewUserLimiter(l.maxTokens, l.refillRate, l.dailyLimit)
		l.users[userID] = ul
	}
	l.mu.Unlock()

	return ul.Allow()
}

// Tokens returns the current token count, mainly useful for debugging/tests.
func (ul *UserLimiter) Tokens() float64 {
	ul.mu.Lock()
	defer ul.mu.Unlock()
	ul.refill(time.Now())
	return ul.tokens
}

// DailyCount returns how many requests have been counted in the current
func (ul *UserLimiter) DailyCount() int {
	ul.mu.Lock()
	defer ul.mu.Unlock()
	ul.resetDailyIfNeeded(time.Now())
	return ul.dailyCount
}