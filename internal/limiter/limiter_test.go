package limiter

import (
	"sync"
	"testing"
	"time"
)

func TestNewUserLimiter_Validation(t *testing.T) {
	tests := []struct {
		name       string
		maxTokens  float64
		refillRate float64
		dailyLimit int
		wantErr    error
	}{
		{"valid config", 100, 5, 4000, nil},
		{"zero maxTokens is valid", 0, 5, 4000, nil},
		{"zero dailyLimit is valid", 100, 5, 0, nil},
		{"negative maxTokens", -1, 5, 4000, ErrInvalidMaxTokens},
		{"zero refillRate", 100, 0, 4000, ErrInvalidRefillRate},
		{"negative refillRate", 100, -5, 4000, ErrInvalidRefillRate},
		{"negative dailyLimit", 100, 5, -1, ErrInvalidDailyLimit},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewUserLimiter(tt.maxTokens, tt.refillRate, tt.dailyLimit)
			if err != tt.wantErr {
				t.Errorf("got err=%v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestAllow_FullBucketAllowsRequest(t *testing.T) {
	ul, err := NewUserLimiter(100, 5, 4000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !ul.Allow() {
		t.Fatal("expected first request on a full bucket to be allowed")
	}
}

func TestAllow_DrainsBucket(t *testing.T) {
	ul, err := NewUserLimiter(5, 5, 4000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	allowed := 0
	for i := 0; i < 10; i++ {
		if ul.Allow() {
			allowed++
		}
	}

	if allowed > 5 {
		t.Errorf("got %d allowed out of a 5-token bucket, want at most 5 (plus negligible refill)", allowed)
	}
	if allowed == 0 {
		t.Error("expected at least some requests to be allowed")
	}
}

func TestAllow_EmptyBucketDeniesRequest(t *testing.T) {
	ul, err := NewUserLimiter(0, 5, 4000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if ul.Allow() {
		t.Fatal("expected request on a zero-token bucket to be denied")
	}
}

func TestAllow_RefillsOverTime(t *testing.T) {
	ul, err := NewUserLimiter(1, 5, 4000) // 5 tokens/sec refill
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !ul.Allow() {
		t.Fatal("expected first request to be allowed")
	}
	if ul.Allow() {
		t.Fatal("expected second immediate request to be denied, bucket should be empty")
	}

	time.Sleep(250 * time.Millisecond) // > 200ms needed to refill 1 token at 5/sec

	if !ul.Allow() {
		t.Fatal("expected request to be allowed after enough time for a refill")
	}
}

func TestAllow_DailyLimitEnforced(t *testing.T) {
	ul, err := NewUserLimiter(1000, 1000, 3) // huge token bucket, tiny daily limit
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for i := 0; i < 3; i++ {
		if !ul.Allow() {
			t.Fatalf("expected request %d to be allowed within daily limit", i+1)
		}
	}

	if ul.Allow() {
		t.Fatal("expected request beyond daily limit to be denied")
	}
}

func TestAllow_ConcurrentAccessIsSafe(t *testing.T) {
	ul, err := NewUserLimiter(50, 5, 4000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	allowed := 0

	const goroutines = 100
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if ul.Allow() {
				mu.Lock()
				allowed++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	if allowed > 50 {
		t.Errorf("got %d allowed out of a 50-token bucket under concurrency, want at most 50 (plus negligible refill)", allowed)
	}
}

func TestLimiter_UsersAreIsolated(t *testing.T) {
	l, err := NewLimiter(1, 5, 4000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !l.Allow("alice") {
		t.Fatal("expected alice's first request to be allowed")
	}
	if l.Allow("alice") {
		t.Fatal("expected alice's second immediate request to be denied")
	}
	if !l.Allow("bob") {
		t.Fatal("expected bob to have his own independent bucket")
	}
}

func TestNewLimiter_InvalidConfig(t *testing.T) {
	_, err := NewLimiter(-1, 5, 4000)
	if err != ErrInvalidMaxTokens {
		t.Errorf("got err=%v, want %v", err, ErrInvalidMaxTokens)
	}
}