package main

import (
	"fmt"
	"sync"

	"github.com/AZAKRIZY/Rate_limiter/internal/limiter"
)

func main() {
	fmt.Println("=== Single success ===")
	single()

	fmt.Println("\n=== Burst, mixed allow/deny ===")
	fresh := limiter.NewUserLimiter(100, 5, 4000)
	burst("alice", fresh, 120)

	fmt.Println("\n=== All denied (empty bucket) ===")
	empty := limiter.NewUserLimiter(0, 5, 4000)
	burst("bob", empty, 5)

	fmt.Println("\n=== Concurrent requests ===")
	shared := limiter.NewUserLimiter(100, 5, 4000)
	concurrent(shared, 20)
}

func single() {
	ul := limiter.NewUserLimiter(100, 5, 4000)
	fmt.Println("allowed:", ul.Allow())
}

func burst(user string, ul *limiter.UserLimiter, n int) {
	allowed, denied := 0, 0
	for i := 0; i < n; i++ {
		if ul.Allow() {
			allowed++
		} else {
			denied++
		}
	}
	fmt.Printf("user=%s requests=%d allowed=%d denied=%d tokens_left=%.2f\n",
		user, n, allowed, denied, ul.Tokens())
}

func concurrent(ul *limiter.UserLimiter, n int) {
	var wg sync.WaitGroup
	var mu sync.Mutex
	allowed, denied := 0, 0

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ok := ul.Allow()
			mu.Lock()
			if ok {
				allowed++
			} else {
				denied++
			}
			mu.Unlock()
		}()
	}

	wg.Wait()
	fmt.Printf("requests=%d allowed=%d denied=%d tokens_left=%.2f\n",
		n, allowed, denied, ul.Tokens())
}