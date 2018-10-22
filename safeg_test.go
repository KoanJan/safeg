package safeg

import (
	"log"
	"sync/atomic"
	"testing"
	"time"
)

func TestGo(t *testing.T) {
	Go(func() {
		time.Sleep(time.Second)
		log.Println("hello1")
	})
	Go(func() {
		time.Sleep(time.Second)
		log.Println("hello2")
	})
	Go(func() {
		time.Sleep(time.Second)
		log.Println("hello3")
	})
	time.Sleep(2 * time.Second)
}

func BenchmarkGo(b *testing.B) {
	var sum int64
	for i := 0; i < b.N; i++ {
		Go(func() {
			atomic.AddInt64(&sum, 1)
			atomic.AddInt64(&sum, -1)
		})
	}
	for atomic.LoadInt64(&sum) > 0 {
	}
}

func TestGoChild(t *testing.T) {
	psgid := Go(func() {
		time.Sleep(time.Second)
		log.Println("parent sg done")
	})
	GoChild(psgid, func() {
		log.Println("child sg done")
	})
	time.Sleep(2 * time.Second)
}

func TestKill(t *testing.T) {
	id := Go(func() {
		time.Sleep(time.Second)
		log.Println("done")
	})
	Kill(id)
	log.Println("killed")
	time.Sleep(time.Second)
}
