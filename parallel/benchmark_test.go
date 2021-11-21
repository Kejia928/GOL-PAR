package main

import (
	"fmt"
	"os"
	"testing"
	"uk.ac.bris.cs/gameoflife/gol"
)

func BenchmarkGol(b *testing.B) {
	// Disable all program output apart from benchmark results
	os.Stdout = nil

	// Use a for-loop to run sub-benchmarks, with 1-16 workers.
	for threads := 1; threads <= 16; threads++ {

		p := gol.Params{ImageWidth: 16, ImageHeight: 16, Turns: 100, Threads: threads}
		b.Run(fmt.Sprintf("%d_workers", threads), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				events := make(chan gol.Event)
				go gol.Run(p, events, nil)
				for range events {
				}
			}
		})
	}
}
