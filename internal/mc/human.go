package mc

import (
	"math/rand"
	"sync"
	"time"
)

var (
	humanMu  sync.Mutex
	humanRnd = rand.New(rand.NewSource(time.Now().UnixNano()))
)

func HumanDelay(minMs, maxMs int) time.Duration {
	if maxMs <= minMs {
		maxMs = minMs + 1
	}
	humanMu.Lock()
	n := minMs + humanRnd.Intn(maxMs-minMs)
	humanMu.Unlock()
	d := time.Duration(n) * time.Millisecond
	time.Sleep(d)
	return d
}

func HumanJitter(base time.Duration, pct int) time.Duration {
	if pct <= 0 {
		return base
	}
	humanMu.Lock()
	delta := humanRnd.Int63n(int64(base) * int64(pct) * 2 / 100)
	neg := humanRnd.Intn(2) == 0
	humanMu.Unlock()
	if neg {
		return base - time.Duration(delta)
	}
	return base + time.Duration(delta)
}

func HumanSleep(base time.Duration, pct int) {
	time.Sleep(HumanJitter(base, pct))
}
