package telemetry

import "github.com/prometheus/client_golang/prometheus"

const livelookNamespace string = "livelook"

var (
	promSessionTotal prometheus.Gauge
)

func init() {
	promSessionTotal = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: livelookNamespace,
		Subsystem: "session",
		Name:      "total",
	})

	prometheus.MustRegister(promSessionTotal)
}

func SessionStarted() {
	promSessionTotal.Add(1)
}
