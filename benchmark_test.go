package main

import (
	"testing"

	"uk.ac.bris.cs/gameoflife/gol"
)

func BenchmarkGol(b *testing.B) {
	p := gol.Params{
		Turns:       5000,
		Threads:     4,
		ImageWidth:  512,
		ImageHeight: 512,
	}

	events := make(chan gol.Event)
	gol.Run(p, events, nil)
	final := false

	for !final {
		event := <-events
		_, final = event.(gol.FinalTurnComplete)

	}
}
