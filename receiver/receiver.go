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

	"github.com/choria-io/prometheus-streams/connection"
	"github.com/choria-io/prometheus-streams/scrape"
	log "github.com/sirupsen/logrus"

	"github.com/choria-io/prometheus-streams/config"
	"github.com/nats-io/go-nats-streaming"
)

var inbox = make(chan scrape.Scrape, 10)
var maxAge int64
var paused bool
var running bool

func Run(ctx context.Context, wg *sync.WaitGroup, cfg *config.Config) {
	defer wg.Done()

	running = true
	maxAge = cfg.MaxAge

	conn := connection.NewConnection(ctx, cfg.ReceiverStream)

	// timed out, ctx cancelled etc, anyway, its dead, nothing can be done
	if conn.Conn == nil {
		return
	}

	opts := []stan.SubscriptionOption{
		stan.DurableName(cfg.ReceiverStream.ClientID),
		stan.DeliverAllAvailable(),
		stan.SetManualAckMode(),
		stan.MaxInflight(10),
	}

	conn.Conn.Subscribe(cfg.ReceiverStream.Topic, handler, opts...)

	go poster(cfg.PushGateway.URL)

	select {
	case <-ctx.Done():
		conn.Conn.Close()
	}
}

func Running() bool {
	return running
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

func poster(url string) {
	tr := &http.Transport{
		MaxIdleConns:    10,
		IdleConnTimeout: 30 * time.Second,
	}

	client := &http.Client{Transport: tr}

	for {
		select {
		case sc := <-inbox:
			target := fmt.Sprintf("%s/metrics/job/%s/instance/%s", url, sc.Job, sc.Instance)

			body, err := uncompress(sc.Scrape)
			if err != nil {
				log.Errorf("Could not uncompress scrape: %s", err)
				continue
			}

			resp, err := client.Post(target, "text/plain", strings.NewReader(string(body)))
			if err != nil {
				if resp != nil && resp.Body != nil {
					resp.Body.Close()
				}

				log.Errorf("Posting to %s failed: %s", target, err)
				continue
			}

			if resp.StatusCode != 202 {
				log.Errorf("Posting to %s failed: %s: %s", target, resp.Status, resp.Body)
				resp.Body.Close()
				continue
			}

			resp.Body.Close()

			log.Infof("Posted %d to %s: %s", len(body), target, resp.Status)
		}
	}
}

func handler(msg *stan.Msg) {
	defer msg.Ack()

	if paused {
		return
	}

	s := scrape.Scrape{}

	err := json.Unmarshal(msg.Data, &s)
	if err != nil {
		log.Errorf("handling failed: %s", err)
		return
	}

	if maxAge > 0 {
		age := time.Now().UTC().Unix() - s.Timestamp

		if age > maxAge {
			log.Warnf("Found %ds old metric for %s discarding due to maxage of %d", age, s.Instance, maxAge)
			return
		}
	}

	inbox <- s
}

func uncompress(data []byte) ([]byte, error) {
	b := bytes.NewBuffer(data)

	r, err := gzip.NewReader(b)
	if err != nil {
		return []byte{}, err
	}
	defer r.Close()

	return ioutil.ReadAll(r)
}
