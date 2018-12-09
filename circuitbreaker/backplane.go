package circuitbreaker

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
)

// Pausable implements backplane.Pausable
type Pausable struct {
	sync.Mutex
	paused bool
	gauge  prometheus.Gauge
}

// New initialize a circuit breaker
func New(g prometheus.Gauge) *Pausable {
	return &Pausable{
		gauge: g,
	}
}

// Pause implements backplane.Pausable
func (p *Pausable) Pause() {
	p.Lock()
	defer p.Unlock()

	p.paused = true
	p.logState()
}

// Resume implements backplane.Pausable
func (p *Pausable) Resume() {
	p.Lock()
	defer p.Unlock()

	p.paused = false
	p.logState()
}

// Flip implements backplane.Pausable
func (p *Pausable) Flip() {
	p.Lock()
	defer p.Unlock()

	p.paused = !p.paused
	p.logState()
}

// Paused implements backplane.Pausable
func (p *Pausable) Paused() bool {
	p.Lock()
	defer p.Unlock()

	return p.paused
}

func (p *Pausable) logState() {
	if p.paused {
		p.gauge.Set(1)
		log.Warnf("Pausing via the circuit breaker")
	} else {
		p.gauge.Set(0)
		log.Warnf("Resuming via the circuit breaker")
	}
}
