package promutil

import (
	"fmt"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

var registries = make(map[string]NewRegistry)
var prometheusRegistry = prometheus.NewRegistry()

//NewRegistry is promotheus registry
type NewRegistry func(opts Options) Registry

//Registry holds all of metrics collectors
//name is a unique ID for different type of metrics
type Registry interface {
	CreateCounter(opts CounterOpts) error

	CounterAdd(name string, val float64, labels map[string]string) error
}

var defaultRegistry Registry

//CreateCounter init a new counter type
func CreateCounter(opts CounterOpts) error {
	return defaultRegistry.CreateCounter(opts)
}

//CounterAdd increase value of a collector
func CounterAdd(name string, val float64, labels map[string]string) error {
	return defaultRegistry.CounterAdd(name, val, labels)
}

//CounterOpts is options to create a counter options
type CounterOpts struct {
	//fmt.Println("inside counterOpts struct")
	Name   string
	Help   string
	Labels []string
}

//Options control config
type Options struct {
	FlushInterval time.Duration
}

//InstallPlugin install metrics registry
func InstallPlugin(name string, f NewRegistry) {
	registries[name] = f
}

//Init load the metrics plugin and initialize it
func Init() error {
	var name string
	name = "prometheus"
	f, ok := registries[name]
	if !ok {
		return fmt.Errorf("can not init metrics registry [%s]", name)
	}

	defaultRegistry = f(Options{
		FlushInterval: 10 * time.Second,
	})

	return nil
}

//GetSystemPrometheusRegistry return prometheus registry which cim use
func GetSystemPrometheusRegistry() *prometheus.Registry {
	return prometheusRegistry
}
