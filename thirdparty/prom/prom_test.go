package prom

import (
	"testing"
)

func TestRead(t *testing.T) {
	sConfig := Section{
		RemoteWrite: []RemoteConfig{},
		RemoteRead: []RemoteConfig{
			{Name: "prom", Url: "http://10.150.30.6:9090/api/v1/read", RemoteTimeoutSecond: 5},
		},
	}
	pd := NewPromDataSource(sConfig)
	pd.Init()
	pd.QueryData(`avg(rate(node_cpu_seconds_total{mode="system"}[1m])) by (instance) *100`)
}
