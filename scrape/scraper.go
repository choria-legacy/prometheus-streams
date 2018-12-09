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
	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/net/context/ctxhttp"
)

func jobWorker(ctx context.Context, wg *sync.WaitGroup, name string, job *config.Job) {
	defer wg.Done()

	targetGauge.WithLabelValues(name).Set(float64(len(job.Targets)))

	for _, target := range job.Targets {
		wg.Add(1)
		go targetWorker(ctx, wg, name, target)
	}
}

func targetWorker(ctx context.Context, wg *sync.WaitGroup, jobname string, target *config.Target) {
	defer wg.Done()

	interval, err := time.ParseDuration(cfg.Interval)
	if err != nil {
		log.Errorf("Could not parse interval '%s', defaulting to 30s: %s", cfg.Interval, err)
		interval = time.Duration(30 * time.Second)
	}
	client := &http.Client{}

	poll := func() {
		obs := prometheus.NewTimer(pollTime.WithLabelValues(jobname, target.Name))
		defer obs.ObserveDuration()

		if Pausable.Paused() && jobname != "prometheus_streams" {
			log.Warnf("Skipping poll for job %s while paused", jobname)
			return
		}

		log.Debugf("Polling job %s %s @ %s", jobname, target.Name, target.URL)

		timeout, cancel := context.WithTimeout(ctx, interval)
		defer cancel()

		resp, err := ctxhttp.Get(timeout, client, target.URL)

		if err != nil {
			log.Errorf("Could not fetch %s: %s", target.URL, err)
			pollErrCtr.WithLabelValues(jobname, target.Name).Inc()
			return
		}

		if resp == nil {
			log.Errorf("Could not fetch %s: unknown error", target.URL)
			pollErrCtr.WithLabelValues(jobname, target.Name).Inc()
			return
		}

		if resp.StatusCode >= 300 {
			log.Errorf("Could not fetch %s: %s", target.URL, resp.Status)
			pollErrCtr.WithLabelValues(jobname, target.Name).Inc()
			return
		}

		body, err := ioutil.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			log.Errorf("Could not read body of %s: %s", target.URL, err)
			pollErrCtr.WithLabelValues(jobname, target.Name).Inc()
			return
		}

		pollSizeCtr.WithLabelValues(jobname, target.Name).Add(float64(len(body)))

		cbody, err := compress(body)
		if err != nil {
			log.Errorf("Could not compress result for %s: %s", target.URL, err)
			pollErrCtr.WithLabelValues(jobname, target.Name).Inc()
			return
		}

		outbox <- Scrape{
			Job:       jobname,
			Instance:  target.Name,
			Timestamp: time.Now().UTC().Unix(),
			Scrape:    cbody,
			Publisher: cfg.Hostname,
		}

		log.Debugf("Completed poll of job %s", jobname)
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	log.Infof("Polling %s using url %s every %s", target.Name, target.URL, interval)

	poll()

	for {
		select {
		case <-ticker.C:
			poll()
		case <-ctx.Done():
			return
		}
	}
}

func compress(data []byte) ([]byte, error) {
	obs := prometheus.NewTimer(compressTime)
	defer obs.ObserveDuration()

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
