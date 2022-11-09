package promutil

import (
	"fmt"
	"sync"
	"time"

	"trlaas/logging"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	onceEnable sync.Once

	// PromEnabled can be true if Promotheus enabled for scraping purpose
	PromEnabled        bool 
	// PrometheusRegistry is Registry
	PrometheusRegistry Registry
)

//PrometheusExporter is a prom exporter for cim
type PrometheusExporter struct {
	FlushInterval time.Duration
	lc            sync.RWMutex
	counters      map[string]*prometheus.CounterVec
}

func init() {
	registries["prometheus"] = NewPrometheusExporter
}

//NewPrometheusExporter create a prometheus exporter
func NewPrometheusExporter(options Options) Registry {
	if PromEnabled {
		onceEnable.Do(func() {
			EnableRunTimeMetrics()
			logging.LogConf.Info("go runtime metrics is exported")
		})

	}
	return &PrometheusExporter{
		FlushInterval: options.FlushInterval,
		lc:            sync.RWMutex{},
		counters:      make(map[string]*prometheus.CounterVec),
	}
}

// EnableRunTimeMetrics enable runtime metrics
func EnableRunTimeMetrics() {
	GetSystemPrometheusRegistry().MustRegister(prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}))
	GetSystemPrometheusRegistry().MustRegister(prometheus.NewGoCollector())
}

//CreateCounter create collector
func (c *PrometheusExporter) CreateCounter(opts CounterOpts) error {
	c.lc.RLock()
	_, ok := c.counters[opts.Name]
	c.lc.RUnlock()
	if ok {
		return fmt.Errorf("metric [%s] is duplicated", opts.Name)
	}
	c.lc.Lock()
	defer c.lc.Unlock()
	v := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: opts.Name,
		Help: opts.Help,
	}, opts.Labels)
	c.counters[opts.Name] = v
	GetSystemPrometheusRegistry().MustRegister(v)
	return nil
}

//CounterAdd increase value
func (c *PrometheusExporter) CounterAdd(name string, val float64, labels map[string]string) error {
	c.lc.RLock()
	v, ok := c.counters[name]
	c.lc.RUnlock()
	if !ok {
		return fmt.Errorf("metrics do not exists, create it first")
	}
	v.With(labels).Add(val)
	return nil
}
