package metrics

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

func TestObserveHarborRequestRecordsStatusAndDuration(t *testing.T) {
	const method = "UNIT_TEST"
	const endpoint = "/unit-test/observe-harbor-request"

	for _, test := range []struct {
		name   string
		status int
		label  string
	}{
		{name: "HTTP response", status: 503, label: "503"},
		{name: "transport error", status: 0, label: "error"},
	} {
		t.Run(test.name, func(t *testing.T) {
			counter := harborAPIRequestsTotal.WithLabelValues(method, endpoint, test.label)
			var beforeCounter dto.Metric
			if err := counter.Write(&beforeCounter); err != nil {
				t.Fatalf("read request counter before observation: %v", err)
			}
			before := beforeCounter.GetCounter().GetValue()
			histogram := harborAPIRequestDurationSeconds.WithLabelValues(method, endpoint, test.label)
			var beforeMetric dto.Metric
			histogramMetric, ok := histogram.(prometheus.Metric)
			if !ok {
				t.Fatal("duration observer does not implement prometheus.Metric")
			}
			if err := histogramMetric.Write(&beforeMetric); err != nil {
				t.Fatalf("read duration histogram before observation: %v", err)
			}
			beforeCount := beforeMetric.GetHistogram().GetSampleCount()

			ObserveHarborRequest(method, endpoint, test.status, 1.25)

			var afterCounter dto.Metric
			if err := counter.Write(&afterCounter); err != nil {
				t.Fatalf("read request counter after observation: %v", err)
			}
			if got := afterCounter.GetCounter().GetValue() - before; got != 1 {
				t.Fatalf("request counter delta = %v, want 1", got)
			}
			var afterMetric dto.Metric
			if err := histogramMetric.Write(&afterMetric); err != nil {
				t.Fatalf("read duration histogram after observation: %v", err)
			}
			if got := afterMetric.GetHistogram().GetSampleCount() - beforeCount; got != 1 {
				t.Fatalf("duration histogram sample delta = %d, want 1", got)
			}
			if got := afterMetric.GetHistogram().GetSampleSum() - beforeMetric.GetHistogram().GetSampleSum(); got != 1.25 {
				t.Fatalf("duration histogram sum delta = %v, want 1.25", got)
			}
		})
	}
}

func TestObserveHarborHTTPClientCacheUsesOnlyBoundedEventLabels(t *testing.T) {
	for _, event := range []string{"hit", "miss", "invalidation", "eviction"} {
		counter := harborHTTPClientCacheEvents.WithLabelValues(event)
		var beforeMetric dto.Metric
		if err := counter.Write(&beforeMetric); err != nil {
			t.Fatal(err)
		}
		before := beforeMetric.GetCounter().GetValue()
		ObserveHarborHTTPClientCache(event)
		var afterMetric dto.Metric
		if err := counter.Write(&afterMetric); err != nil {
			t.Fatal(err)
		}
		if got := afterMetric.GetCounter().GetValue() - before; got != 1 {
			t.Fatalf("cache event %q counter delta = %v, want 1", event, got)
		}
	}
}
