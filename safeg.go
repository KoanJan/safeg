package safeg

import (
	"context"
	"log"
	"sync"
	"sync/atomic"
	"time"
)

const (
	stateForever uint8 = iota
	stateReady
	stateRunning
	statePanic
	stateDone
)

var (
	root *sg                              // root sg
	id   uint64 = 0                       // id counter
	pid         = make(map[uint64]uint64) // parent id map
	pidm        = sync.RWMutex{}
)

func defaultPanic(c interface{}) {
	log.Printf("sgo: panic '%v'\n", c)
}

type sg struct {
	id     uint64
	ncsg   uint64
	csg    map[uint64]*sg // csg is a skip-list of child sg
	m      sync.RWMutex
	state  uint8
	ctx    context.Context
	cancel context.CancelFunc
}

func sgo(p *sg, f func(), timeout time.Duration, onPanic func(interface{})) uint64 {
	newid := atomic.AddUint64(&id, 1)
	p.m.Lock()
	p.csg[newid] = &sg{
		id:    id,
		ncsg:  0,
		csg:   make(map[uint64]*sg),
		state: stateReady,
	}
	if timeout > 0 {
		p.csg[newid].ctx, p.csg[newid].cancel = context.WithTimeout(p.ctx, timeout)
	} else {
		p.csg[newid].ctx, p.csg[newid].cancel = context.WithCancel(p.ctx)
	}
	p.m.Unlock()
	atomic.AddUint64(&p.ncsg, 1)
	pidm.Lock()
	pid[newid] = p.id
	pidm.Unlock()
	go func() {
		p.m.RLock()
		ctx := p.csg[newid].ctx
		p.m.RUnlock()
		select {
		case <-ctx.Done():
			p.m.Lock()
			p.csg[newid].cancel()
			p.m.Unlock()
		case <-func() chan bool {
			finish := make(chan bool, 1)
			defer func() {
				if c := recover(); c != nil {
					p.m.Lock()
					p.csg[newid].state = statePanic
					p.m.Unlock()
					onPanic(c)
				}
			}()
			p.m.Lock()
			p.csg[newid].state = stateRunning
			p.m.Unlock()
			f()
			finish <- true
			return finish
		}():
			// done
		}
		// update state
		p.m.RLock()
		isPanic := p.csg[newid].state == statePanic
		p.m.RUnlock()
		if isPanic {
			p.m.Lock()
			p.csg[newid].state = stateDone
			p.m.Unlock()
		}
		// delete child sg info
		delInfo(newid, p)

	}()
	return newid
}

func delInfo(id uint64, pg *sg) {
	pg.m.RLock()
	g := pg.csg[id]
	pg.m.RUnlock()
	pg.m.Lock()
	delete(pg.csg, id)
	pg.m.Unlock()
	pidm.Lock()
	delete(pid, id)
	pidm.Unlock()
	for cid, cg := range g.csg {
		cg.m.RLock()
		hasChildren := len(cg.csg) > 0
		cg.m.RUnlock()
		if hasChildren {
			delInfo(cid, cg)
		}
	}
}

func Go(f func()) uint64 {
	return sgo(root, f, 0, defaultPanic)
}

func GoChild(parent uint64, f func()) uint64 {
	return sgo(getsg(parent), f, 0, defaultPanic)
}

func getsg(id uint64) *sg {
	idlist := []uint64{id}
	pidm.RLock()
	for pid[id] != 0 {
		idlist = append(idlist, pid[id])
		id = pid[id]
	}
	pidm.RUnlock()
	psg := root.csg[id]
	for i := len(idlist) - 2; i >= 0; i-- {
		psg.m.RLock()
		newpsg := psg.csg[idlist[i]]
		psg.m.RUnlock()
		psg = newpsg
	}
	return psg
}

func Kill(id uint64) {
	if g := getsg(id); g != nil {
		g.cancel()
	}
}

func init() {
	root = &sg{
		id:    0,
		state: stateForever,
		csg:   make(map[uint64]*sg),
		m:     sync.RWMutex{},
		ncsg:  0,
		ctx:   context.TODO(),
		cancel: func() {
			panic("root sg cannot be cancel!")
		},
	}
}
