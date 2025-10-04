package monitor

type semaphore struct {
	C chan struct{}
}

func (s *semaphore) acquire() {
	s.C <- struct{}{}
}

func (s *semaphore) release() {
	<-s.C
}
