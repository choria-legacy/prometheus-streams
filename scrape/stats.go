package scrape

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	jobsGauge = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "prometheus_streams_poller_jobs",
		Help: "How many jobs are configured",
	})

	publishedCtr = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "prometheus_streams_poller_published_count",
		Help: "How many messages containing scrapes were published",
	})

	errorCtr = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "prometheus_streams_poller_publish_errors",
		Help: "Errors encountered during publishes",
	})

	pollErrCtr = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "prometheus_streams_poller_poll_errors",
		Help: "Errors encountered during polls",
	}, []string{"poller_job", "poller_target"})

	publishTime = prometheus.NewSummary(prometheus.SummaryOpts{
		Name: "prometheus_streams_poller_publish_time",
		Help: "How long it took to publish messages to the stream",
	})

	compressTime = prometheus.NewSummary(prometheus.SummaryOpts{
		Name: "prometheus_streams_poller_compress_time",
		Help: "How long it takes to compress messages",
	})

	pollTime = prometheus.NewSummaryVec(prometheus.SummaryOpts{
		Name: "prometheus_streams_poller_poll_time",
		Help: "How long it takes to poll targets",
	}, []string{"poller_job", "poller_target"})

	pollSizeCtr = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "prometheus_streams_poller_bytes_polled",
		Help: "How many bytes were polled from a target",
	}, []string{"poller_job", "poller_target"})

	pauseGauge = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "prometheus_streams_poller_paused",
		Help: "Indicates if the poller is paused",
	})

	targetGauge = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "prometheus_streams_poller_targets",
		Help: "How many targets are configured",
	}, []string{"poller_job"})
)

func init() {
	prometheus.MustRegister(jobsGauge)
	prometheus.MustRegister(publishedCtr)
	prometheus.MustRegister(errorCtr)
	prometheus.MustRegister(pollErrCtr)
	prometheus.MustRegister(publishTime)
	prometheus.MustRegister(pauseGauge)
	prometheus.MustRegister(targetGauge)
	prometheus.MustRegister(compressTime)
	prometheus.MustRegister(pollTime)
	prometheus.MustRegister(pollSizeCtr)
}
