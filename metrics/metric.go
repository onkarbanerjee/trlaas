package metrics

import (
	"fmt"
	"trlaas/logging"
	"trlaas/promutil"
)

//MetricInit function to register log metrics
func MetricInit() {

	fmt.Println("inside metrics.go")
	var err error

	err = promutil.CreateCounter(promutil.CounterOpts{
		Name:   "tpaas_slt_sessions_total",
		Help:   "Total number of SLT session requests received by TPaaS.",
		Labels: []string{"sourceIP", "nfId", "traceSessionId", "action"},
	})

	if err != nil {
		logging.LogConf.Error("error while creating counter metric for tpaas_slt_sessions_total", err)
		return
	}

	err = promutil.CreateCounter(promutil.CounterOpts{
		Name:   "tpaas_slt_sessions_error_total",
		Help:   "Total number of SLT session requests for which TPaaS sent an erorr response.",
		Labels: []string{"sourceIP", "nfId", "traceSessionId", "action", "responseCode"},
	})

	if err != nil {
		logging.LogConf.Error("error while creating counter metric for tpaas_slt_sessions_error_total", err)
		return
	}

	err = promutil.CreateCounter(promutil.CounterOpts{
		Name:   "tpaas_slt_sessions_suspend_total",
		Help:   "Total number of SLT session suspend requests processed by TPaaS.",
		Labels: []string{"sourceIP", "nfId", "operationId", "transactionId", "action"},
	})

	if err != nil {
		logging.LogConf.Error("error while creating counter metric for tpaas_slt_sessions_suspend_total", err)
		return
	}

	err = promutil.CreateCounter(promutil.CounterOpts{
		Name:   "tpaas_slt_sessions_resume_total",
		Help:   "Total number of SLT session resume requests processed by TPaaS.",
		Labels: []string{"sourceIP", "nfId", "operationId", "transactionId", "action"},
	})

	if err != nil {
		logging.LogConf.Error("error while creating counter metric for tpaas_slt_sessions_resume_total", err)
		return
	}

	err = promutil.CreateCounter(promutil.CounterOpts{
		Name:   "tpaas_trldata_handled_total",
		Help:   "This metric keeps track of the total number of TRL data handled by TPaaS on the data path.",
		Labels: []string{"mNfType", "mNfId"},
	})

	if err != nil {
		logging.LogConf.Error("error while creating counter metric for tpaas_trldata_handled_total", err)
		return
	}

	err = promutil.CreateCounter(promutil.CounterOpts{
		Name:   "tpaas_sltdata_handled_total",
		Help:   "This metric keeps track of the total number of SLT data handled by TPaaS on the data path.",
		Labels: []string{"mNfType", "mNfId"},
	})

	if err != nil {
		logging.LogConf.Error("error while creating counter metric for tpaas_sltdata_handled_total", err)
		return
	}

	err = promutil.CreateCounter(promutil.CounterOpts{
		Name:   "tpaas_schema_registration_total",
		Help:   "This metric keeps track of the total number of schema registrations handled by TPaaS on the control path.",
		Labels: []string{"mNfType", "mNfId", "recordType", "statusCode"},
	})

	if err != nil {
		logging.LogConf.Error("error while creating counter metric for tpaas_schema_registration_total", err)
		return
	}

	err = promutil.CreateCounter(promutil.CounterOpts{
		Name:   "tpaas_slt_requests_total",
		Help:   "This metric keeps track of the total number of slt requests from the CMS handled by TPaaS on the control path.",
		Labels: []string{"mNfId", "statusCode", "action"},
	})

	if err != nil {
		logging.LogConf.Error("error while creating counter metric for tpaas_slt_requests_total", err)
		return
	}

	err = promutil.CreateCounter(promutil.CounterOpts{
		Name:   "tpaas_total_messages_consumed",
		Help:   "Total number of messages consumed by TPaaS.",
		Labels: []string{"topic"},
	})

	if err != nil {
		logging.LogConf.Error("error while creating counter metric for tpaas_total_messages_consumed", err)
	}

	err = promutil.CreateCounter(promutil.CounterOpts{
		Name:   "tpaas_total_messages_processed",
		Help:   "Total number of messages processed by TPaaS.",
		Labels: []string{"topic"},
	})

	if err != nil {
		logging.LogConf.Error("error while creating counter metric for tpaas_total_messages_processed", err)
	}
}
