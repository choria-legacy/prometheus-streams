package receiver

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/choria-io/go-lifecycle"

	"github.com/choria-io/prometheus-streams/circuitbreaker"

	"github.com/choria-io/prometheus-streams/build"
	"github.com/choria-io/prometheus-streams/connection"
	"github.com/choria-io/prometheus-streams/scrape"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"

	"github.com/choria-io/prometheus-streams/config"
	"github.com/nats-io/go-nats-streaming"
)

var inbox = make(chan scrape.Scrape, 10)
var restart = make(chan struct{})
var maxAge int64
var err error
var conn *connection.Connection
var log *logrus.Entry

// Pausable is the circuit breaker for the receiver
var Pausable *circuitbreaker.Pausable

func Run(ctx context.Context, wg *sync.WaitGroup, cfg *config.Config) {
	defer wg.Done()

	log = cfg.Log("receiver")
	maxAge = cfg.MaxAge
	Pausable = circuitbreaker.New(pauseGauge)

	err = connect(ctx, cfg)
	if err != nil {
		log.Errorf("Could not connect: %s", err)
		return
	}

	log.Infof("Choria Prometheus Streams Receiver version %s starting with configuration file %s", build.Version, cfg.ConfigFile)

	opts := []stan.SubscriptionOption{
		stan.DurableName(cfg.ReceiverStream.ClientID),
		stan.DeliverAllAvailable(),
		stan.SetManualAckMode(),
		stan.MaxInflight(10),
	}

	conn.Conn.Subscribe(cfg.ReceiverStream.Topic, handler, opts...)

	go poster(cfg)

	select {
	case <-restart:
		conn.Close()
		err = connect(ctx, cfg)
		if err != nil {
			log.Errorf("Could not connect: %s", err)
			return
		}

	case <-ctx.Done():
		conn.Conn.Close()
	}
}

func connect(ctx context.Context, cfg *config.Config) error {
	conn, err = connection.NewConnection(ctx, cfg.ReceiverStream, cfg.Log("connector"), func(_ stan.Conn, reason error) {
		errorCtr.WithLabelValues("unknown").Inc()
		log.Errorf("Stream connection disconnected, initiating reconnection: %s", reason)
		restart <- struct{}{}
	})
	if err != nil {
		return fmt.Errorf("could not set up middleware connection: %s", err)
	}

	// timed out, ctx cancelled etc, anyway, its dead, nothing can be done
	if conn.Conn == nil {
		return fmt.Errorf("could not set up middleware connection, perhaps due to interrupt")
	}

	log.Infof("Choria Prometheus Streams Receiver version %s starting with configuration file %s", build.Version, cfg.ConfigFile)

	event, err := lifecycle.New(lifecycle.Startup, lifecycle.Identity(cfg.Hostname), lifecycle.Component("prometheus_streams_receiver"), lifecycle.Version(build.Version))
	if err != nil {
		log.Errorf("Could not create startup lifecycle event: %s", err)
	}

	if event != nil {
		err = lifecycle.PublishEvent(event, conn)
		if err != nil {
			log.Errorf("Could not publish lifecycle event: %s", err)
		}
	}

	opts := []stan.SubscriptionOption{
		stan.DurableName(cfg.ReceiverStream.ClientID),
		stan.DeliverAllAvailable(),
		stan.SetManualAckMode(),
		stan.MaxInflight(10),
	}

	conn.Conn.Subscribe(cfg.ReceiverStream.Topic, handler, opts...)

	return nil
}

func poster(cfg *config.Config) {
	tr := &http.Transport{
		MaxIdleConns:    10,
		IdleConnTimeout: 30 * time.Second,
	}

	client := &http.Client{Transport: tr}

	publisher := func(sc scrape.Scrape) {
		obs := prometheus.NewTimer(publishTime)
		defer obs.ObserveDuration()

		var target string

		if cfg.PushGateway.PublisherLabel {
			target = fmt.Sprintf("%s/metrics/job/%s/instance/%s/publisher/%s", cfg.PushGateway.URL, sc.Job, sc.Instance, sc.Publisher)
		} else {
			target = fmt.Sprintf("%s/metrics/job/%s/instance/%s", cfg.PushGateway.URL, sc.Job, sc.Instance)
		}

		body, err := uncompress(sc.Scrape)
		if err != nil {
			log.Errorf("Could not uncompress scrape: %s", err)
			errorCtr.WithLabelValues(sc.Job).Inc()
			return
		}

		resp, err := client.Post(target, "text/plain", strings.NewReader(string(body)))
		if err != nil {
			if resp != nil && resp.Body != nil {
				resp.Body.Close()
			}

			log.Errorf("Posting to %s failed: %s", target, err)
			errorCtr.WithLabelValues(sc.Job).Inc()
			return
		}

		if resp.StatusCode != 202 {
			log.Errorf("Posting to %s failed: %s: %s", target, resp.Status, resp.Body)
			resp.Body.Close()
			errorCtr.WithLabelValues(sc.Job).Inc()
			return
		}

		resp.Body.Close()

		log.Debugf("Posted %d to %s: %s", len(body), target, resp.Status)
	}

	for {
		select {
		case sc := <-inbox:
			publisher(sc)
		}
	}
}

func handler(msg *stan.Msg) {
	defer msg.Ack()

	msgCtr.Inc()

	if Pausable.Paused() {
		return
	}

	s := scrape.Scrape{}

	err := json.Unmarshal(msg.Data, &s)
	if err != nil {
		log.Errorf("handling failed: %s", err)
		errorCtr.WithLabelValues("unknown").Inc()
		return
	}

	if maxAge > 0 {
		age := time.Now().UTC().Unix() - s.Timestamp

		if age > maxAge {
			log.Warnf("Found %ds old metric for %s discarding due to maxage of %d", age, s.Instance, maxAge)
			agedCtr.WithLabelValues(s.Job).Inc()
			return
		}
	}

	instanceSeenTime.WithLabelValues(s.Publisher).Set(float64(time.Now().UTC().Unix()))

	inbox <- s
}

func uncompress(data []byte) ([]byte, error) {
	obs := prometheus.NewTimer(decompressTime)
	defer obs.ObserveDuration()

	b := bytes.NewBuffer(data)

	r, err := gzip.NewReader(b)
	if err != nil {
		return []byte{}, err
	}
	defer r.Close()

	return ioutil.ReadAll(r)
}
