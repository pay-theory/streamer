package main

import (
    "fmt"
    "time"
    _ "github.com/pay-theory/streamer/lambda/connect"
    _ "github.com/pay-theory/streamer/lambda/disconnect"
    _ "github.com/pay-theory/streamer/lambda/router"
    _ "github.com/pay-theory/streamer/lambda/processor"
)

func main() {
    start := time.Now()
    // Simulate initialization
    time.Sleep(1 * time.Millisecond)
    duration := time.Since(start)
    fmt.Printf("Package load time: %v\n", duration)
}
