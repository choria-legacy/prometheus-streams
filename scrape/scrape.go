package scrape

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	lifecycle "github.com/choria-io/go-lifecycle"
	"github.com/choria-io/prometheus-streams/build"
	"github.com/choria-io/prometheus-streams/circuitbreaker"
	"github.com/choria-io/prometheus-streams/config"
	"github.com/choria-io/prometheus-streams/connection"
	"github.com/nats-io/go-nats-streaming"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
)

var cfg *config.Config
var log *logrus.Entry

type Scrape struct {
	Job       string `json:"job"`
	Instance  string `json:"instance"`
	Timestamp int64  `json:"time"`
	Publisher string `json:"publisher"`
	Scrape    []byte
}

var outbox = make(chan Scrape, 1000)
var restart = make(chan struct{}, 1)
var stream *connection.Connection
var hostname string
var err error
var Pausable *circuitbreaker.Pausable

func Run(ctx context.Context, wg *sync.WaitGroup, scrapeCfg *config.Config) {
	defer wg.Done()

	log = scrapeCfg.Log("poller")

	log.Infof("Choria Prometheus Streams Poller version %s starting with configuration file %s", build.Version, scrapeCfg.ConfigFile)

	cfg = scrapeCfg
	Pausable = circuitbreaker.New(pauseGauge)

	stream, err = connect(ctx, scrapeCfg)
	if err != nil {
		log.Errorf("Could not start scrape: %s", err)
		return
	}

	jobsGauge.Set(float64(len(cfg.Jobs)))
	pauseGauge.Set(0)

	for name, job := range cfg.Jobs {
		wg.Add(1)
		go jobWorker(ctx, wg, name, job)
	}

	for {
		select {
		case <-restart:
			stream, err = connect(ctx, scrapeCfg)
			if err != nil {
				log.Errorf("Could not start scrape: %s", err)
				return
			}

		case m := <-outbox:
			publish(m)

		case <-ctx.Done():
			return
		}
	}
}

func connect(ctx context.Context, scrapeCfg *config.Config) (*connection.Connection, error) {
	stream, err = connection.NewConnection(ctx, scrapeCfg.PollerStream, scrapeCfg.Log("connector"), func(_ stan.Conn, reason error) {
		errorCtr.Inc()
		log.Errorf("Stream connection disconnected, initiating reconnection: %s", reason)
		restart <- struct{}{}
	})

	if err != nil {
		return nil, fmt.Errorf("Could not start scrape: %s", err)
	}

	event, err := lifecycle.New(lifecycle.Startup, lifecycle.Identity(cfg.Hostname), lifecycle.Component("prometheus_streams_poller"), lifecycle.Version(build.Version))
	if err != nil {
		log.Errorf("Could not create startup lifecycle event: %s", err)
	}

	if event != nil {
		err = lifecycle.PublishEvent(event, stream)
		if err != nil {
			log.Errorf("Could not publish lifecycle event: %s", err)
		}
	}

	return stream, nil
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

	log.Debugf("Published %d bytes to %s for job %s", len(j), cfg.PollerStream.Topic, m.Job)
}
