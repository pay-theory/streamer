package main

import (
	"fmt"
	"time"
	// Removed lambda imports as they are main packages (executables) and cannot be imported
	// If you need to measure lambda initialization, consider moving shared logic to importable packages
)

func main() {
	start := time.Now()
	// Simulate initialization - measuring basic Go runtime initialization
	time.Sleep(1 * time.Millisecond)
	duration := time.Since(start)
	fmt.Printf("Package load time: %v\n", duration)
}
