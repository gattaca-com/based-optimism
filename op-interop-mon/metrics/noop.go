package metrics

import (
	opmetrics "github.com/ethereum-optimism/optimism/op-service/metrics"
)

type noopMetrics struct {
	opmetrics.NoopRefMetrics
	opmetrics.NoopRPCMetrics
}

var NoopMetrics Metricer = new(noopMetrics)

func (*noopMetrics) RecordInfo(version string) {}
func (*noopMetrics) RecordUp()                 {}
