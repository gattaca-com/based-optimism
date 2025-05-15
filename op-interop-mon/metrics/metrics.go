package metrics

import (
	"github.com/prometheus/client_golang/prometheus"

	opmetrics "github.com/ethereum-optimism/optimism/op-service/metrics"
)

const Namespace = "op_interop_mon"

var _ opmetrics.RegistryMetricer = (*Metrics)(nil)

type Metricer interface {
	RecordInfo(version string)
	RecordUp()

	opmetrics.RefMetricer
	opmetrics.RPCMetricer
}

type Metrics struct {
	ns       string
	registry *prometheus.Registry
	factory  opmetrics.Factory

	opmetrics.RefMetrics
	opmetrics.RPCMetrics

	info prometheus.GaugeVec
	up   prometheus.Gauge
}

var _ Metricer = (*Metrics)(nil)

func NewMetrics(procName string) *Metrics {
	if procName == "" {
		procName = "default"
	}
	ns := Namespace + "_" + procName

	registry := opmetrics.NewRegistry()
	factory := opmetrics.With(registry)

	return &Metrics{
		ns:       ns,
		registry: registry,
		factory:  factory,

		RefMetrics: opmetrics.MakeRefMetrics(ns, factory),
		RPCMetrics: opmetrics.MakeRPCMetrics(ns, factory),

		info: *factory.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: ns,
			Name:      "info",
			Help:      "Information about the monitor",
		}, []string{
			"version",
		}),
		up: factory.NewGauge(prometheus.GaugeOpts{
			Namespace: ns,
			Name:      "up",
			Help:      "1 if the op-interop-mon has finished starting up",
		}),
	}
}

func (m *Metrics) Registry() *prometheus.Registry {
	return m.registry
}

func (m *Metrics) RecordInfo(version string) {
	m.info.WithLabelValues(version).Set(1)
}

func (m *Metrics) RecordUp() {
	m.up.Set(1)
}

func (m *Metrics) Document() []opmetrics.DocumentedMetric {
	return m.factory.Document()
}
