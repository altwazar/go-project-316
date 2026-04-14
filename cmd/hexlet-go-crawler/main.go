package main

import (
	"code"
	"context"
	"fmt"
)

func main() {
	ctx := context.Background()
	opts := code.Options{}
	out, err := code.Analyze(ctx, opts)
	if err == nil {
		fmt.Printf("out: %v", out)
	}
}
