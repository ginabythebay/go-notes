/**

UUID Benchmarking

I'm playing around with different approaches to thread safety here to see how it impacts performance.

Basically I stole some existing UUID code that was using locking, and made versions that have a single goroutine (which doesn't need to lock) serving back results over a channel.  I tried different channel sizes to see if that mattered.  Some take-aways.

1. It doesn't affect performance all that much.
2. For this case, locks are slightly faster than channels.
3. An unbuffered channel seems to be faster than a buffered one, in this case.

This all might change if I had multiple threads actually requesting UUIDs.

Below are the raw results from running the benchmarks on my desktop:

  BenchmarkNewV1-8            	20000000	       101 ns/op
  BenchmarkSatoriNewV1-8      	20000000	       101 ns/op
  BenchmarkChanneledNewV1/chansize=0-8         	10000000	       134 ns/op
  BenchmarkChanneledNewV1/chansize=1-8         	10000000	       145 ns/op
  BenchmarkChanneledNewV1/chansize=2-8         	10000000	       152 ns/op
  BenchmarkChanneledNewV1/chansize=3-8         	10000000	       163 ns/op
  BenchmarkChanneledNewV1/chansize=4-8         	10000000	       172 ns/op
  BenchmarkChanneledNewV1/chansize=5-8         	10000000	       177 ns/op
  BenchmarkChanneledNewV1/chansize=6-8         	10000000	       183 ns/op
  BenchmarkChanneledNewV1/chansize=7-8         	10000000	       186 ns/op
  BenchmarkChanneledNewV1/chansize=8-8         	10000000	       189 ns/op
  BenchmarkChanneledNewV1/chansize=9-8         	10000000	       191 ns/op
  BenchmarkChanneledNewV1/chansize=10-8        	10000000	       193 ns/op
  BenchmarkChanneledNewV1/chansize=100-8       	10000000	       196 ns/op
  BenchmarkChanneledNewV1/chansize=1000-8      	10000000	       198 ns/op
  BenchmarkNewV1LockFree-8                     	10000000	       199 ns/op


*/

package main

import (
	"fmt"
	"testing"
)

func BenchmarkNewV1(b *testing.B) {
	for n := 0; n < b.N; n++ {
		NewV1()
	}
}

func BenchmarkSatoriNewV1(b *testing.B) {
	g := NewSatoriGenerator()
	for n := 0; n < b.N; n++ {
		g.NewV1()
	}
}

var channelSizes = []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 100, 1000}

func BenchmarkChanneledNewV1(b *testing.B) {
	for _, size := range channelSizes {
		f := func(b *testing.B) {
			g := NewChanneledGenerator(size)
			for n := 0; n < b.N; n++ {
				g.NewV1()
			}
		}
		b.Run(fmt.Sprintf("chansize=%d", size), f)
	}

}

func BenchmarkNewV1LockFree(b *testing.B) {
	for n := 0; n < b.N; n++ {
		NewV1LockFree()
	}
}
