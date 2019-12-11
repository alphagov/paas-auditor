package collectors

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	CFAuditEventCollectorErrorsTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "cf_audit_event_collector_errors_total",
		Help: "Number of errors encountered by CF Audit Event Collector",
	})

	CFAuditEventCollectorEventsCollectedTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "cf_audit_event_collector_events_collected_total",
		Help: "Number of events collected and saved to the DB by CF Audit Event Collector",
	})

	CFAuditEventCollectorEventsCollectDurationTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "cf_audit_event_collector_collect_duration_total",
		Help: "Number of seconds spent collecting events by CF Audit Event Collector",
	})
)

func initMetrics() {
	prometheus.MustRegister(CFAuditEventCollectorErrorsTotal)
	prometheus.MustRegister(CFAuditEventCollectorEventsCollectedTotal)
	prometheus.MustRegister(CFAuditEventCollectorEventsCollectDurationTotal)
}
