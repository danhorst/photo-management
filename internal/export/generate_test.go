package export

import (
	"sync"
	"testing"
)

func TestGeneratorConfigConcurrent(t *testing.T) {
	g := &Generator{}
	defer g.Close()

	var wg sync.WaitGroup
	paths := make([]string, 20)
	errs := make([]error, 20)
	for i := range paths {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			paths[i], errs[i] = g.config()
		}(i)
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Fatalf("call %d: %v", i, err)
		}
		if paths[i] == "" || paths[i] != paths[0] {
			t.Fatalf("call %d returned %q, want the same path every call", i, paths[i])
		}
	}
}
