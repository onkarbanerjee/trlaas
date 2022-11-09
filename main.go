package main

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"
	"trlaas/api"
	"trlaas/config"
	"trlaas/etcd"
	"trlaas/events"
	"trlaas/kafka"
	"trlaas/logging"
	"trlaas/metrics"
	"trlaas/processavro"
	"trlaas/promutil"
	"trlaas/slt"
	"trlaas/types"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/soheilhy/cmux"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
	"golang.org/x/sync/errgroup"
)

func serveHTTP(l net.Listener, mux http.Handler) error {

	h2s := &http2.Server{}

	server := &http.Server{
		Handler: h2c.NewHandler(mux, h2s),
	}

	if err := server.Serve(l); err != cmux.ErrListenerClosed {
		return err
	}
	return nil
}

func serveHTTPS(l net.Listener, mux http.Handler) error {

	config := &tls.Config{
		InsecureSkipVerify: true,
		NextProtos:         []string{http2.ClientPreface},
	}
	// setup Server
	s := &http.Server{
		Handler:   mux,
		TLSConfig: config,
	}

	if err := s.ServeTLS(l, "server.crt", "server.key"); err != cmux.ErrListenerClosed {
		return err
	}
	return nil
}

func init() {
	log.Println("init........................")
	//logLevel := os.Getenv("LOG_LEVEL")
	//logging.InitializeLogInfo(logLevel)
	//logging.ContainerID = os.Getenv("K8S_CONTAINER_ID")
	getContainerID()
}

func main() {

	defer logging.NatsConn.Close()
	log.Println("TPaaS service starting.........................!")
	logging.NatsConnection()

	logging.LogConf.Info("Started TPaaS service ....")
	config.Load()
	logging.InitializeLogInfo()

	//Performance counters
	promutil.PromEnabled = true
	//init metrics registry by prometheus name and setting the options in default registry
	if err := promutil.Init(); err != nil {
		logging.LogConf.Error("error while registring the prometheus", err)
		return
	}

	logging.LogConf.Info("conf", promutil.PrometheusRegistry)
	//creating counters and setting the specified metrics in default registry
	metrics.MetricInit()

	for {
		responseCode := mtcilRegister()
		if responseCode == 200 {
			break
		}
		logging.LogConf.Error("Error while mtcil registration. Retrying... ")
		time.Sleep(1 * time.Second)
	}

	// Raise the configuration event
	for cnfRetry := 0; cnfRetry < 5; cnfRetry++ {
		if err := events.RaiseEvent("Configured", []string{"Node"}, []string{"TPaaS"}, []string{}, []string{}); err == nil {
			logging.LogConf.Error("Configured event successfully sent to cim.")
			break
		}
		logging.LogConf.Error("Error while raising Configured event to CIM, Retrying.....")
	}
	r := mux.NewRouter()
	port := 7070

	l, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		logging.LogConf.Exception(err.Error())
	}

	tcpm := cmux.New(l)

	// Declare the matchers for different services required.
	httpl := tcpm.Match(cmux.HTTP2(), cmux.HTTP1()) // it will match http and h2c
	httpsl := tcpm.Match(cmux.TLS())                // it will match tls irrespective of http version

	g := new(errgroup.Group)
	g.Go(func() error { return serveHTTP(httpl, r) })
	g.Go(func() error { return serveHTTPS(httpsl, r) })
	g.Go(func() error { return tcpm.Serve() })

	logging.LogConf.Info(fmt.Sprintf("Listening on %d for HTTP/HTTP2 requests", port))
	//cim metric handler
	r.Handle("/metrics", promhttp.HandlerFor(promutil.GetSystemPrometheusRegistry(), promhttp.HandlerOpts{}))
	r.HandleFunc("/updateConfig", config.HandleTpaasConfig).Methods("POST")
	r.HandleFunc("/api/v1/_operations/tpaas/register_schema", processavro.RegisterSchemas)

	slt.EtcdResEnabled = os.Getenv("TEST_ETCDRESILIENCY")
	if len(slt.EtcdResEnabled) == 0 {
		slt.EtcdResEnabled = "false"
	}
	if strings.Contains(slt.EtcdResEnabled, "true") {
		logging.LogConf.Info("TEST_ETCDRESILIENCY is enabled")
		r.HandleFunc("/testetcdresiliency/delaysltoprequest", slt.Delay).Methods(http.MethodPost)
	}

	// Specification Registration
	err = api.RegisterAPI()
	if err != nil {
		logging.LogConf.Exception("Error while registering SLT API's", err.Error())
	}

	slt.AddRoutes(r)

	//go processavro.ProcessTRLMessages()
	if len(types.TpaasConfigObj.TpaasConfig.App.AnalyticKafkaBrokers) != 0 && types.TpaasConfigObj.TpaasConfig.App.SchemaRegistryUrl != "" {
		if len(types.TpaasConfigObj.TpaasConfig.App.AnalyticKafkaBrokers) == 1 && types.TpaasConfigObj.TpaasConfig.App.AnalyticKafkaBrokers[0] == "" {
			logging.LogConf.Error("Analytic Kafka Brokers IP is Missing")
		} else {
			types.MtcilKafkaConsumerStarted = true
			processavro.SchemaClient()
			logging.LogConf.Info("Starting Consumer for TRL and SLT topic in TPaaS service")
			err := kafka.InitializeConfluentConfigs()
			if err != nil {
				types.MtcilKafkaConsumerStarted = false
			} else {
				go processavro.StartConsumer("TRL")
				go processavro.StartConsumer("SLT")
				go kafka.FlushMsgs()
			}
		}
	} else if len(types.TpaasConfigObj.TpaasConfig.App.AnalyticKafkaBrokers) != 0 && types.TpaasConfigObj.TpaasConfig.App.SchemaRegistryUrl == "" {
		if len(types.TpaasConfigObj.TpaasConfig.App.AnalyticKafkaBrokers) == 1 && types.TpaasConfigObj.TpaasConfig.App.AnalyticKafkaBrokers[0] == "" {
			logging.LogConf.Error("Analytic Kafka Brokers IP is Missing")
		}
		logging.LogConf.Error("Schema Registry URL Missing")
	} else {
		logging.LogConf.Error("Analytic Kafka Brokers and Schema Registry URL Missing")
	}
	signals, done := make(chan os.Signal), make(chan struct{})
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM, syscall.SIGABRT, syscall.SIGKILL, syscall.SIGQUIT)
	go func() {
		logging.LogConf.Error("Server error, if any:", g.Wait())
		done <- struct{}{}
	}()

	//establish a connection with etcd
	err = etcd.Connect()
	if err != nil {
		logging.LogConf.Exception("Unable to connect etcd ", err)
	}
	defer etcd.Close()

	//Start ETCD response key watcher
	go slt.GetEtcdResponses()

OUTER:
	for {
		select {
		case <-done:
			logging.LogConf.Info("All go routines stopped, exiting")
			break OUTER
		case <-signals:
			logging.LogConf.Info("Received signal to stop")
			break OUTER
		}
	}

	kafka.Close()
	logging.LogConf.Info("Stopping TPaaS service...")
}

func mtcilRegister() int {

	cimport := "6060"

	registerDetails := map[string]string{
		"container_name": "tpaas",
		"container_id":   logging.ContainerID,
	}
	jsonValue, err := json.Marshal(registerDetails)
	if err != nil {
		logging.LogConf.Error("error while marshalling the request data to be sent for mtcil registration", err.Error())
		return http.StatusBadRequest
	}
	response, err := http.Post("http://localhost:"+cimport+"/api/v1/_operations/mtcil/register", "application/json", bytes.NewBuffer(jsonValue))
	if err != nil {
		logging.LogConf.Error("The HTTP POST request for mtcil reg. failed with error", err.Error())
		return http.StatusInternalServerError
	}

	logging.LogConf.Info("mtcil registration successfull ", strconv.Itoa(response.StatusCode))
	return response.StatusCode
}

//getContainerID get containerid from script
func getContainerID() {
	out, err := exec.Command("/bin/bash", "-c", "/opt/bin/get_containerid.sh").Output()
	if err != nil {
		fmt.Println("error while getting the container id from get_containerid.sh", err)
		logging.ContainerID = "0000"
		os.Setenv("K8S_CONTAINER_ID", logging.ContainerID)
		return
	}
	log.Println("container id is :", string(out))
	logging.ContainerID = strings.TrimSpace(string(out))
	logging.LogConf.Info("Received container Id-", logging.ContainerID)
	os.Setenv("K8S_CONTAINER_ID", logging.ContainerID)

}
