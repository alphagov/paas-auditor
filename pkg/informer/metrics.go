package informer

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	InformerCFAuditEventsTotal = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "informer_cf_audit_events_total",
		Help: "Number of CF audit events in the database",
	})

	InformerLatestCFAuditEventTimestamp = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "informer_latest_cf_audit_event_timestamp",
		Help: "Unix epoch seconds of most recent event in the database",
	})
)

func initMetrics() {
	prometheus.MustRegister(InformerCFAuditEventsTotal)
	prometheus.MustRegister(InformerLatestCFAuditEventTimestamp)
}
