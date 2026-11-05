package yaegikernel

import (
	"fmt"
	"sync"
)

// One uid per activation (MA6.2). The desk manifest's uid names the desk and
// still gates the spawn (a role without a cell identity is refused); the uid
// a worker actually runs as is drawn from a pool and is held by no other live
// worker, so two workers of one desk cannot signal, trace or read each
// other's process or tmp dir through shared credentials. The uid is numeric
// and needs no passwd entry. Released at reap.
const (
	activationUIDBase  uint32 = 200000
	activationUIDCount uint32 = 60000
)

type uidPool struct {
	mu   sync.Mutex
	next uint32
	live map[uint32]bool
}

var activationUIDs = &uidPool{live: map[uint32]bool{}}

// acquire returns an unused uid and its release.
func (p *uidPool) acquire() (uint32, func(), error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if uint32(len(p.live)) >= activationUIDCount {
		return 0, nil, fmt.Errorf("desk cell confinement: %d live workers hold every activation uid", len(p.live))
	}
	for {
		uid := activationUIDBase + p.next%activationUIDCount
		p.next++
		if !p.live[uid] {
			p.live[uid] = true
			var once sync.Once
			return uid, func() { once.Do(func() { p.mu.Lock(); delete(p.live, uid); p.mu.Unlock() }) }, nil
		}
	}
}
