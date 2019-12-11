package testhelpers

import (
	"fmt"

	"github.com/onsi/gomega/matchers"
	"github.com/onsi/gomega/types"

	"github.com/prometheus/client_golang/prometheus"
	putil "github.com/prometheus/client_golang/prometheus/testutil"
)

func CurrentMetricValue(metric prometheus.Collector) float64 {
	return putil.ToFloat64(metric)
}

func MetricIncrementedBy(
	before float64,
	comparator string,
	expected float64,
) types.GomegaMatcher {
	return &metricIncrementedByMatcher{
		before:     before,
		comparator: comparator,
		expected:   expected,
	}
}

type metricIncrementedByMatcher struct {
	before     float64
	comparator string
	expected   float64
}

func (m *metricIncrementedByMatcher) wrapped() *matchers.BeNumericallyMatcher {
	return &matchers.BeNumericallyMatcher{
		Comparator: m.comparator,
		CompareTo:  []interface{}{m.before + m.expected},
	}
}

func (m *metricIncrementedByMatcher) Match(act interface{}) (bool, error) {
	metric, ok := act.(prometheus.Collector)

	if !ok {
		return false, fmt.Errorf("%v is not a prometheus.Collector", act)
	}

	return m.wrapped().Match(putil.ToFloat64(metric))
}

func (m *metricIncrementedByMatcher) FailureMessage(act interface{}) string {
	metric, ok := act.(prometheus.Collector)

	if !ok {
		return fmt.Sprintf("%v is not a prometheus.Collector", act)
	}

	return m.wrapped().FailureMessage(putil.ToFloat64(metric))
}

func (m *metricIncrementedByMatcher) NegatedFailureMessage(act interface{}) string {
	metric, ok := act.(prometheus.Collector)

	if !ok {
		return fmt.Sprintf("%v is not a prometheus.Collector", act)
	}

	return m.wrapped().NegatedFailureMessage(putil.ToFloat64(metric))
}
