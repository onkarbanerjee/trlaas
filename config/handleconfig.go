package config

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"strings"
	"time"
	"trlaas/events"
	"trlaas/kafka"
	"trlaas/logging"
	"trlaas/processavro"
	"trlaas/types"
	"trlaas/utility"
)

var (
	failureEventRaised = false
)

func prepareCommitDetails(key string, revision string, err error) map[string]string {
	commitMsg := make(map[string]string)
	commitMsg["change-set-key"] = key
	commitMsg["revision"] = revision
	if err != nil {
		commitMsg["status"] = "failure"
		commitMsg["remarks"] = "failed to apply the config patch"
	} else {
		commitMsg["status"] = "success"
		commitMsg["remarks"] = "Successfully applied the config patch"
	}
	return commitMsg
}

func HandleTpaasConfig(w http.ResponseWriter, r *http.Request) {
	reqBody, err := ioutil.ReadAll(r.Body)
	if err != nil {
		logging.LogConf.Error("error while reading request body sent for tpaas config update.", err.Error())
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	var data = make(map[string]string)
	err = json.Unmarshal(reqBody, &data)
	if err != nil {
		logging.LogConf.Error("error while unmarshalling request body sent for tpaas config update.", err.Error())
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	logging.LogConf.Info("Event received!", data, "executed on ", data["change-set-key"], "with value", data["config-patch"])
	err = utility.ApplyJsonPatch([]byte(data["config-patch"]))

	commitMsg := prepareCommitDetails(data["change-set-key"], data["revision"], err)
	eventValues, err := json.Marshal(commitMsg)
	if err != nil {
		logging.LogConf.Error(err)
	}
	err = logging.NatsConn.Publish("CONFIG", eventValues)
	if err != nil {
		logging.LogConf.Error("error while publishing message to nats-" + err.Error())
		return
	}
	logging.NatsConn.Flush()

	if strings.Contains(data["config-patch"], "log_level") {
		logging.LogConf.Info("Logging Method switched to ", types.TpaasConfigObj.TpaasConfig.App.LogLevel)
		logging.InitializeLogInfo()
	}
	if strings.Contains(data["config-patch"], "retry_count") {
		logging.LogConf.Info("Retry Count Changed to", types.TpaasConfigObj.TpaasConfig.App.RetryCount)
	}
	if strings.Contains(data["config-patch"], "num_of_partitions") {
		logging.LogConf.Info("Num of Partitions Count for Analytic Kafka Changed to", types.TpaasConfigObj.TpaasConfig.App.NumOfPartitions)
	}
	//if len(types.TpaasConfigObj.TpaasConfig.App.AnalyticKafkaBrokers) != 0 && types.TpaasConfigObj.TpaasConfig.App.SchemaRegistryUrl != "" {
	if strings.Contains(data["config-patch"], "schema_registry_url") {
		//Initialising schema registry client conn.
		processavro.SchemaClient()
		logging.LogConf.Info("Schema Registry Url Changed to", types.TpaasConfigObj.TpaasConfig.App.SchemaRegistryUrl)

	}

	if strings.Contains(data["config-patch"], "analytic_kafka_brokers") {
		logging.LogConf.Info("Analytic Kafka Brokers Changed to", types.TpaasConfigObj.TpaasConfig.App.AnalyticKafkaBrokers)
	}

	go func() {
		if len(types.TpaasConfigObj.TpaasConfig.App.AnalyticKafkaBrokers) != 0 && types.TpaasConfigObj.TpaasConfig.App.SchemaRegistryUrl != "" {
			if len(types.TpaasConfigObj.TpaasConfig.App.AnalyticKafkaBrokers) == 1 && types.TpaasConfigObj.TpaasConfig.App.AnalyticKafkaBrokers[0] == "" {
				logging.LogConf.Error("Analytic Kafka Brokers IP is Missing")
			} else {
				logging.LogConf.Info("Trying to establish connection with analytic kafka brokers")
				<-time.After(5 * time.Second)
				err := kafka.InitializeConfluentConfigs()
				if err != nil {
					logging.LogConf.Error("Could not establish any kafka connection at all, hence will NOT start consuming %s", err)
					eventErr := events.RaiseEvent("TpaasNbiFailure", []string{"Description", "UpdateType"}, []string{"Analytic Kafka Connection", "Dynamic"}, []string{"Error", "STATUS"}, []string{err.Error(), "Failure"})
					if eventErr != nil {
						logging.LogConf.Error(" error while notifying Analytic Kafka Connection failure to FMaaS From Dynamic Update", "Error", err)
					}
					failureEventRaised = true
				} else {
					processavro.XAKafkaUrlStatus = true
					if failureEventRaised {
						eventErr := events.RaiseEvent("TpaasNbiSuccess", []string{"Description", "UpdateType"}, []string{"Analytic Kafka Connection", "Dynamic"}, []string{"STATUS"}, []string{"Success"})
						if eventErr != nil {
							logging.LogConf.Error(" error while notifying Analytic Kafka Connection success to FMaaS Dynamic Update", "Error", err)
						}
						failureEventRaised = false
					}
					if !types.MtcilKafkaConsumerStarted {
						logging.LogConf.Info("Starting Consumer for TRL and SLT topic in TPaaS service")
						go processavro.StartConsumer("TRL")
						go processavro.StartConsumer("SLT")
						go kafka.FlushMsgs()
					}
				}
			}
		} else {
			processavro.XAKafkaUrlStatus = false
			kafka.Close()
		}
	}()

	if strings.Contains(data["config-patch"], "replication_factor") {
		logging.LogConf.Info("Replication Factor for Analytic Kafka Changed to", types.TpaasConfigObj.TpaasConfig.App.ReplicationFactor)
	} //

	if strings.Contains(data["config-patch"], "maxActivePerNf") {
		logging.LogConf.Info("MaxActivePerNf Changed to", types.TpaasConfigObj.TpaasConfig.Slt.MaxActivePerNf)
	}
	return

}
