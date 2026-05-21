package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	StatsPullTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "stats_pull_total",
			Help: "Total number of successful stats pulls",
		},
		[]string{"platform"},
	)
	StatsPullFailTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "stats_pull_fail_total",
			Help: "Total number of failed stats pulls",
		},
		[]string{"platform", "reason"},
	)
	SchedulerCycleTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "scheduler_cycle_total",
			Help: "Total number of scheduler cycles executed",
		},
	)
	SchedulerLastSuccessTimestamp = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "scheduler_last_success_timestamp",
			Help: "Unix timestamp of last successful cycle",
		},
	)
)

func Register() {
	prometheus.MustRegister(
		StatsPullTotal,
		StatsPullFailTotal,
		SchedulerCycleTotal,
		SchedulerLastSuccessTimestamp,
	)
}

func Handler() http.HandlerFunc {
	return promhttp.Handler().ServeHTTP
}
