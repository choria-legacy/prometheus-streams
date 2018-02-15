package receiver

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	errorCtr = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "prometheus_streams_receiver_publish_errors",
		Help: "Errors encountered during pushing to PushGateway",
	}, []string{"receiver_job"})

	agedCtr = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "prometheus_streams_receiver_aged_discards",
		Help: "Messages that got discarded due to age",
	}, []string{"receiver_job"})

	msgCtr = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "prometheus_streams_receiver_received",
		Help: "Total number of received messages including too old ones",
	})

	pauseGauge = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "prometheus_streams_receiver_paused",
		Help: "Indicates if the receiver is paused",
	})

	decompressTime = prometheus.NewSummary(prometheus.SummaryOpts{
		Name: "prometheus_streams_receiver_decompress_time",
		Help: "How long it takes to decompress messages",
	})

	publishTime = prometheus.NewSummary(prometheus.SummaryOpts{
		Name: "prometheus_streams_receiver_publish_time",
		Help: "How long it takes to publish to Push Gateway",
	})

	instanceSeenTime = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "prometheus_streams_receiver_seen_time",
		Help: "When last data for a specific job and instance was received",
	}, []string{"receiver_job", "receiver_instance", "poller"})
)

func init() {
	prometheus.MustRegister(errorCtr)
	prometheus.MustRegister(pauseGauge)
	prometheus.MustRegister(agedCtr)
	prometheus.MustRegister(decompressTime)
	prometheus.MustRegister(publishTime)
	prometheus.MustRegister(msgCtr)
	prometheus.MustRegister(instanceSeenTime)
}
