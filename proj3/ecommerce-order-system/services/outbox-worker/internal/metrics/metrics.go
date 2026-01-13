package metrics

import "github.com/prometheus/client_golang/prometheus"

var (
	OutboxSentTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "outbox_sent_total",
		Help: "Total outbox events successfully published",
	})
	OutboxPublishErrorsTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "outbox_publish_errors_total",
		Help: "Total outbox publish errors",
	})
	OutboxPending = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "outbox_pending",
		Help: "Number of pending outbox events",
	})
)

func init() {
	prometheus.MustRegister(OutboxSentTotal, OutboxPublishErrorsTotal, OutboxPending)
}
