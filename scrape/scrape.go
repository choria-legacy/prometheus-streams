package scrape

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/choria-io/prometheus-streams/config"
	"github.com/choria-io/prometheus-streams/connection"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
)

var cfg *config.Config

type Scrape struct {
	Job       string `json:"job"`
	Instance  string `json:"instance"`
	Timestamp int64  `json:"time"`
	Publisher string `json:"publisher"`
	Scrape    []byte
}

var outbox = make(chan Scrape, 1000)
var paused bool
var running bool
var stream *connection.Connection
var hostname string

func Run(ctx context.Context, wg *sync.WaitGroup, scrapeCfg *config.Config) {
	defer wg.Done()

	running = true
	cfg = scrapeCfg

	stream = connection.NewConnection(ctx, scrapeCfg.PollerStream)

	jobsGauge.Set(float64(len(cfg.Jobs)))
	pauseGauge.Set(0)

	for name, job := range cfg.Jobs {
		wg.Add(1)
		go jobWorker(ctx, wg, name, job)
	}

	for {
		select {
		case m := <-outbox:
			publish(m)
		case <-ctx.Done():
			return
		}
	}
}

func publish(m Scrape) {
	obs := prometheus.NewTimer(publishTime)
	defer obs.ObserveDuration()

	j, err := json.Marshal(m)
	if err != nil {
		log.Errorf("Could not publish data: %s", err)
		errorCtr.Inc()
		return
	}

	err = stream.Publish(cfg.PollerStream.Topic, j)
	if err != nil {
		log.Errorf("Could not publish data: %s", err)
		errorCtr.Inc()
		return
	}

	publishedCtr.Inc()

	log.Debugf("Published %d bytes to %s", len(j), "prometheus")
}

func Paused() bool {
	return paused
}

func FlipCircuitBreaker() bool {
	paused = !paused

	if paused {
		pauseGauge.Set(1)
	} else {
		pauseGauge.Set(0)
	}

	if running {
		log.Warnf("Switching the circuit breaker: paused: %t", paused)
	}

	return Paused()
}

func Running() bool {
	return running
}
