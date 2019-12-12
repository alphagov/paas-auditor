package shippers

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	CFAuditEventsToSplunkShipperErrorsTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "cf_audit_events_to_splunk_shipper_errors_total",
		Help: "Number of errors encountered by CF Audit Events to Splunk shipper",
	})

	CFAuditEventsToSplunkShipperEventsShippedTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "cf_audit_events_to_splunk_shipper_events_shipped_total",
		Help: "Number of CF audit events shipped to Splunk by CF Audit Events to Splunk shipper",
	})

	CFAuditEventsToSplunkShipperLatestEventTimestamp = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "cf_audit_events_to_splunk_shipper_latest_event_timestamp",
		Help: "Unix epoch seconds of most recent event shipped to Splunk",
	})

	CFAuditEventsToSplunkShipperShipDurationTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "cf_audit_events_to_splunk_shipper_ship_duration_total",
		Help: "Number of seconds spent shipping events by CF Audit Events to Splunk Shipper",
	})
)

func initMetrics() {
	prometheus.MustRegister(CFAuditEventsToSplunkShipperErrorsTotal)
	prometheus.MustRegister(CFAuditEventsToSplunkShipperEventsShippedTotal)
	prometheus.MustRegister(CFAuditEventsToSplunkShipperLatestEventTimestamp)
	prometheus.MustRegister(CFAuditEventsToSplunkShipperShipDurationTotal)
}
