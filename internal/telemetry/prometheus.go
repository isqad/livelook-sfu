package telemetry

import "github.com/prometheus/client_golang/prometheus"

const livelookNamespace string = "livelook"

var (
	promSessionTotal        prometheus.Gauge
	ServiceOperationCounter *prometheus.CounterVec
)

func init() {
	promSessionTotal = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: livelookNamespace,
		Subsystem: "session",
		Name:      "total",
	})

	ServiceOperationCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace:   livelookNamespace,
			Subsystem:   "node",
			Name:        "service_operation",
			ConstLabels: prometheus.Labels{"node_id": "1"},
		},
		[]string{"type", "status", "error_type"},
	)

	prometheus.MustRegister(promSessionTotal)
	prometheus.MustRegister(ServiceOperationCounter)
}

func SessionStarted() {
	promSessionTotal.Inc()
}

func SessionStopped() {
	promSessionTotal.Dec()
}
