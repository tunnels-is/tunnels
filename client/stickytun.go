package client

import (
	"os"
	"sync"
	"sync/atomic"
	"time"

	"golang.zx2c4.com/wireguard/tun"
)

// stickyTUN keeps the OS TUN open when wireguard-go Device.Close() calls
// tun.Close(). Read is unblocked with a deadline so Close() does not deadlock.
// Release() is the real teardown (manual disconnect).
//
// Windows NativeTun.File() is nil, so we do not wrap those devices.
type stickyTUN struct {
	inner tun.Device

	mu       sync.Mutex
	stop     chan struct{}
	ev       chan tun.Event
	pumpDone chan struct{}

	stopped  atomic.Bool
	released atomic.Bool
}

func newStickyTUN(inner tun.Device) *stickyTUN {
	s := &stickyTUN{inner: inner}
	s.startPump()
	return s
}

func (s *stickyTUN) startPump() {
	s.mu.Lock()
	s.stop = make(chan struct{})
	s.ev = make(chan tun.Event, 16)
	s.pumpDone = make(chan struct{})
	stop, ev, done, inner := s.stop, s.ev, s.pumpDone, s.inner
	s.mu.Unlock()
	s.stopped.Store(false)

	go func() {
		defer close(done)
		defer close(ev)
		for {
			select {
			case <-stop:
				return
			case e, ok := <-inner.Events():
				if !ok {
					return
				}
				select {
				case ev <- e:
				case <-stop:
					return
				}
			}
		}
	}()
}

func (s *stickyTUN) eventsChan() <-chan tun.Event {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.ev
}

func (s *stickyTUN) File() *os.File           { return s.inner.File() }
func (s *stickyTUN) MTU() (int, error)        { return s.inner.MTU() }
func (s *stickyTUN) Name() (string, error)    { return s.inner.Name() }
func (s *stickyTUN) BatchSize() int           { return s.inner.BatchSize() }
func (s *stickyTUN) Events() <-chan tun.Event { return s.eventsChan() }
func (s *stickyTUN) Write(bufs [][]byte, off int) (int, error) {
	return s.inner.Write(bufs, off)
}

func (s *stickyTUN) Read(bufs [][]byte, sizes []int, offset int) (int, error) {
	n, err := s.inner.Read(bufs, sizes, offset)
	if s.stopped.Load() || s.released.Load() {
		return 0, os.ErrClosed
	}
	return n, err
}

func (s *stickyTUN) Close() error {
	if s.released.Load() {
		return nil
	}
	if !s.stopped.CompareAndSwap(false, true) {
		return nil
	}
	s.mu.Lock()
	stop := s.stop
	done := s.pumpDone
	s.mu.Unlock()
	close(stop)
	if f := s.inner.File(); f != nil {
		_ = f.SetReadDeadline(time.Now())
	}
	if done != nil {
		<-done
	}
	return nil
}

func (s *stickyTUN) ResetForReuse() error {
	if s.released.Load() {
		return os.ErrClosed
	}
	if !s.stopped.Load() {
		return nil
	}
	if f := s.inner.File(); f != nil {
		if err := f.SetReadDeadline(time.Time{}); err != nil {
			return err
		}
	}
	s.startPump()
	return nil
}

func (s *stickyTUN) Release() error {
	if !s.released.CompareAndSwap(false, true) {
		return nil
	}
	_ = s.Close()
	return s.inner.Close()
}

func (s *stickyTUN) CanReuse() bool {
	if s == nil || s.released.Load() {
		return false
	}
	if s.inner.File() == nil {
		return false
	}
	name, err := s.inner.Name()
	return err == nil && name != ""
}
