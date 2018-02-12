package scrape

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/choria-io/prometheus-streams/config"
	"github.com/choria-io/prometheus-streams/connection"
	log "github.com/sirupsen/logrus"
)

var cfg *config.Config

type Scrape struct {
	Job       string `json:"job"`
	Instance  string `json:"instance"`
	Timestamp int64  `json:"time"`
	Scrape    []byte
}

var outbox = make(chan Scrape, 1000)
var paused bool
var running bool

func Run(ctx context.Context, wg *sync.WaitGroup, scrapeCfg *config.Config) {
	defer wg.Done()

	cfg = scrapeCfg

	stream := connection.NewConnection(ctx, scrapeCfg.PollerStream)

	for name, job := range cfg.Jobs {
		wg.Add(1)
		go jobWorker(ctx, wg, name, job)
	}

	running = true

	for {
		select {
		case m := <-outbox:
			j, err := json.Marshal(m)
			if err != nil {
				log.Errorf("Could not publish data: %s", err)
				continue
			}

			stream.Publish(cfg.PollerStream.Topic, j)
			log.Debugf("Published %d bytes to %s", len(j), "prometheus")

		case <-ctx.Done():
			return
		}
	}
}

func Paused() bool {
	return paused
}

func FlipCircuitBreaker() bool {
	paused = !paused

	if running {
		log.Warnf("Switching the circuit breaker: paused: %t", paused)
	}

	return Paused()
}

func Running() bool {
	return running
}
