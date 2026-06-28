package vpn

import (
	"sync"
	"testing"
)

func TestStateCacheConcurrent(t *testing.T) {
	c := newStateCache()
	c.set(Status{Connected: true, ForwardedPort: 1234})
	if got := c.get(); !got.Connected || got.ForwardedPort != 1234 {
		t.Fatalf("unexpected: %+v", got)
	}

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(2)
		go func() { defer wg.Done(); c.set(Status{ForwardedPort: 5}) }()
		go func() { defer wg.Done(); _ = c.get() }()
	}
	wg.Wait() // -race must stay clean
}
