# ratelimiter

A per-user rate limiter built in Go, combining a **token bucket** (for burst control) with a **daily quota** (for long-term usage caps). This is my first concrete Go project — built to get hands-on with goroutines, mutexes, and concurrency-safe design.

## What it does

Given a user identity, `Allow()` answers a simple question: **should this request go through, right now?**

Two independent limits are enforced together:

| Limiter | Purpose | Scale |
|---|---|---|
| Token bucket | Prevents short-term bursts | seconds |
| Daily quota | Caps total usage per user | 24 hours |

A request is only allowed if **both** checks pass.

## Design

- **Token bucket:** capacity of 100 tokens, refilled at 1 token per 200ms (5 tokens/sec). Refill is calculated lazily on each request based on elapsed time — no background goroutine ticking constantly.
- **Daily quota:** hard cap of 4,000 requests per user per day, tracked with a reset timestamp (fixed window).
- **Concurrency-safe:** each user's limiter state is protected by a `sync.Mutex`, so it's safe to call `Allow()` from multiple goroutines concurrently.
- **Zero dependencies:** built entirely with Go's standard library (`sync`, `time`).

### Trade-offs

The daily quota uses a fixed reset window rather than a sliding window. This means a user could theoretically make close to 4,000 requests right before the reset and another 4,000 right after — a known limitation of fixed-window counters, accepted here for simplicity. A sliding window would remove this edge case at the cost of higher memory usage (tracking individual request timestamps).

## Usage

```go
limiter := ratelimiter.New(ratelimiter.Config{
    MaxTokens:  100,
    RefillRate: time.Millisecond * 200,
    DailyLimit: 4000,
})

if limiter.Allow("user-123") {
    // process request
} else {
    // reject: too many requests
}
```

*(API surface subject to change as the project develops.)*

## Project structure

```
ratelimiter/
├── internal/limiter/   # core rate limiting logic
├── cmd/demo/           # CLI demo simulating concurrent requests
└── README.md
```

## Running tests

```bash
go test ./...
```

## Status

🚧 Work in progress — built as a learning project on the path toward backend/systems engineering depth.

## Why token bucket

Token bucket was chosen over fixed-window or sliding-window counters because it naturally allows short bursts of traffic (up to bucket capacity) while still enforcing a steady average rate over time — matching real-world API traffic patterns better than a naive per-window counter. It's also the algorithm used, in some form, by most production rate limiters (AWS, Stripe, Cloudflare).