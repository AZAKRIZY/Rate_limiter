package main

import (
	"fmt"
	"log"
	"sync"

	"github.com/AZAKRIZY/Rate_limiter/pkg/limiter"
)

func main() {
	fmt.Println("=== Single success ===")
	single()

	fmt.Println("\n=== Burst, mixed allow/deny ===")
	fresh, err := limiter.NewUserLimiter(100, 5, 4000)
	if err != nil {
		log.Fatal(err)
	}
	burst("alice", fresh, 120)

	fmt.Println("\n=== All denied (empty bucket) ===")
	empty, err := limiter.NewUserLimiter(0, 5, 4000)
	if err != nil {
		log.Fatal(err)
	}
	burst("bob", empty, 5)

	fmt.Println("\n=== Concurrent requests ===")
	shared, err := limiter.NewUserLimiter(100, 5, 4000)
	if err != nil {
		log.Fatal(err)
	}
	concurrent(shared, 20)

	fmt.Println("\n=== Multi-user limiter ===")
	multiUser()

	fmt.Println("\n=== Invalid config ===")
	invalidConfig()
}

func single() {
	ul, err := limiter.NewUserLimiter(100, 5, 4000)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("allowed:", ul.Allow())
}

func multiUser() {
	l, err := limiter.NewLimiter(100, 5, 4000)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("alice allowed:", l.Allow("alice"))
	fmt.Println("bob allowed:", l.Allow("bob"))
	fmt.Println("alice allowed again:", l.Allow("alice"))
}

func invalidConfig() {
	_, err := limiter.NewUserLimiter(-1, 5, 4000)
	fmt.Println("error:", err)
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