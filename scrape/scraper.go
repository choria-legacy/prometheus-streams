package scrape

import (
	"bytes"
	"compress/gzip"
	"context"
	"io/ioutil"
	"net/http"
	"sync"
	"time"

	"github.com/choria-io/prometheus-streams/config"
	log "github.com/sirupsen/logrus"
	"golang.org/x/net/context/ctxhttp"
)

func jobWorker(ctx context.Context, wg *sync.WaitGroup, name string, job *config.Job) {
	defer wg.Done()

	for _, target := range job.Targets {
		wg.Add(1)
		go targetWorker(ctx, wg, name, target)
	}
}

func targetWorker(ctx context.Context, wg *sync.WaitGroup, jobname string, target *config.Target) {
	defer wg.Done()

	interval, _ := time.ParseDuration(cfg.Interval)

	log.Infof("Polling %s using url %s every %s", target.Name, target.URL, interval)

	timer := time.NewTicker(interval)

	client := &http.Client{}

	for {
		select {
		case <-timer.C:
			if paused {
				continue
			}

			log.Debugf("Polling %s @ %s", target.Name, target.URL)
			timeout, _ := context.WithTimeout(ctx, interval)
			resp, err := ctxhttp.Get(timeout, client, target.URL)
			if err != nil {
				log.Errorf("Could not fetch %s: %s", target.URL, err)
				continue
			}

			if resp.StatusCode >= 300 {
				log.Errorf("Could not fetch %s: %s", target.URL, resp.Status)
				continue
			}

			body, err := ioutil.ReadAll(resp.Body)
			resp.Body.Close()
			if err != nil {
				log.Errorf("Could not read body of %s: %s", target.URL, err)
				continue
			}

			cbody, err := compress(body)
			if err != nil {
				log.Errorf("Could not compress result for %s: %s", target.URL, err)
				continue
			}

			outbox <- Scrape{
				Job:       jobname,
				Instance:  target.Name,
				Timestamp: time.Now().UTC().Unix(),
				Scrape:    cbody,
			}
		case <-ctx.Done():
			return
		}
	}
}

func compress(data []byte) ([]byte, error) {
	var b bytes.Buffer

	gz := gzip.NewWriter(&b)

	_, err := gz.Write(data)
	if err != nil {
		return []byte{}, err
	}

	err = gz.Flush()
	if err != nil {
		return []byte{}, err
	}

	err = gz.Close()
	if err != nil {
		return []byte{}, err
	}

	return b.Bytes(), nil
}
