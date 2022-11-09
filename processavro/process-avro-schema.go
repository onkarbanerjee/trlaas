package processavro

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
	"trlaas/events"
	conn "trlaas/kafka"
	"trlaas/logging"
	"trlaas/promutil"
	"trlaas/types"

	"github.com/confluentinc/confluent-kafka-go/kafka"

	schemaregistry "github.com/landoop/schema-registry"
)

// RegisterSchema is request message recieved from app.
type RegisterSchema struct {
	RecordType   string           `json:"record_type"`
	Schema       string           `json:"schema,omitempty"`
	Version      string           `json:"version,omitempty"`
	TmaasHeaders TmaaSAnnotations `json:"tmaas_annotations"`
	SchemaName   string           `json:"schema_name,omitempty"`
}

// ResponseTaas is response message from TPaaS to App upon Schema Registration
type ResponseTaas struct {
	Status   string `json:"status"`
	SchemaID int    `json:"schema_id,omitempty"`
	ErrMsg   string `json:"err_msg,omitempty"`
}

// TmaaSAnnotations are Tmaas details.
type TmaaSAnnotations struct {
	VendorID      string `json:"vendorId"`
	MtcilID       string `json:"mtcilId"`
	NfClass       string `json:"nfClass"`
	NfType        string `json:"nfType"`
	NfID          string `json:"nfId"`
	NfServiceID   string `json:"nfServiceId"`
	NfServiceType string `json:"nfServiceType"`
}

var (
	client                 *schemaregistry.Client
	err                    error
	stopConsumer           bool
	XAKafkaUrlStatus       = true
	once                   sync.Once
	nonRetriableErrorCodes []string
)

const (
	//TrlPrefix is prefix for TRL
	TrlPrefix = "trl-"
	//SltPrefix is prefix for SLT
	SltPrefix = "slt-"
)

func getNonRetriableErrCodes() {
	inputErrCodes := os.Getenv("SCHEMA_REGISTRY_NON_RETRIABLE_ERROR_CODES")
	if inputErrCodes == "" {
		inputErrCodes = "42201"
	}
	logging.LogConf.Info("inputErrCodes", inputErrCodes)
	nonRetriableErrorCodes = strings.Split(inputErrCodes, "|")
	logging.LogConf.Info("nonRetriableErrorCodes", nonRetriableErrorCodes)
}

//check if schema registration error code is retriable or not
func isErrorRetriable(err error) bool {
	getNonRetriableErrCodes()
	for _, val := range nonRetriableErrorCodes {
		if strings.Contains(err.Error(), val) {
			return false
		}
	}
	return true
}

//SchemaClient iis used to get new schema client.
func SchemaClient() error {

	url := types.TpaasConfigObj.TpaasConfig.App.SchemaRegistryUrl
	if url == "" {
		logging.LogConf.Info("Schema Registry URL is removed by confd. Setting SchemaClent to nil.")
		client = nil
		return errors.New("schema registry URL not configured")
	}
	client, err = schemaregistry.NewClient(url)
	if err != nil {
		client = nil
		logging.LogConf.Error("Error while establishing client connection with schema registry..", err.Error())
		return err
	}
	return nil
}

func RegisterSchemas(w http.ResponseWriter, r *http.Request) {
	var responseTaas ResponseTaas
	var regSchema RegisterSchema
	nfType := ""
	nfId := ""
	recordType := ""
	statusCode := http.StatusOK

	defer func() {
		// counter increment for tpaas_schema_registration_total
		err = promutil.CounterAdd("tpaas_schema_registration_total", 1, map[string]string{"mNfType": nfType, "mNfId": nfId, "recordType": recordType, "statusCode": strconv.Itoa(statusCode)})
		if err != nil {
			logging.LogConf.Error("Error while adding values in  counter  for Total number of schema registration requests processed by TPaaS.", err)
		}
	}()

	reqBody, err := ioutil.ReadAll(r.Body)
	if err != nil {
		logging.LogConf.Error("error while reading request body sent for schema reg.", err.Error())
		w.WriteHeader(http.StatusBadRequest)
		responseTaas.Status = "failure"
		responseTaas.ErrMsg = "Invalid Request Body"
		json.NewEncoder(w).Encode(responseTaas)
		statusCode = http.StatusBadRequest
		return
	}

	err = json.Unmarshal(reqBody, &regSchema)
	if err != nil {
		logging.LogConf.Error("error while unmarshaling request body sent for schema reg.", err.Error())
		w.WriteHeader(http.StatusBadRequest)
		responseTaas.Status = "failure"
		responseTaas.ErrMsg = "Invalid Request Body"
		json.NewEncoder(w).Encode(responseTaas)
		statusCode = http.StatusBadRequest
		return
	}

	if regSchema.TmaasHeaders.NfType == "" {
		logging.LogConf.Error("NfType is not present in TmaasHeaders of request sent for schema reg.")
		w.WriteHeader(http.StatusBadRequest)
		responseTaas.Status = "failure"
		responseTaas.ErrMsg = "Invalid Request Body"
		json.NewEncoder(w).Encode(responseTaas)
		statusCode = http.StatusBadRequest
		return
	}

	nfType = regSchema.TmaasHeaders.NfType
	nfId = regSchema.TmaasHeaders.NfID

	if regSchema.Schema == "" {
		logging.LogConf.Error("Schema can't be empty")
		w.WriteHeader(http.StatusBadRequest)
		responseTaas.Status = "failure"
		responseTaas.ErrMsg = "Invalid Request Body"
		json.NewEncoder(w).Encode(responseTaas)
		statusCode = http.StatusBadRequest
		return
	}

	subject := ""
	switch regSchema.RecordType {
	case "TRL":
		subject = TrlPrefix + nfType + "-" + "value"
		recordType = "trl"
	case "SLT":
		subject = SltPrefix + nfType + "-" + "value"
		recordType = "slt"
	default:
		logging.LogConf.Error("Invalid recoed type.", regSchema.RecordType)
		w.WriteHeader(http.StatusBadRequest)
		responseTaas.Status = "failure"
		responseTaas.ErrMsg = "Invalid Request Body"
		json.NewEncoder(w).Encode(responseTaas)
		statusCode = http.StatusBadRequest
		return
	}
	logging.LogConf.Info("Request received for registering", regSchema.RecordType, "Schema..!!")

	//Retrying for failed HTTP
	var id, i int
	if client == nil {
		// Trying to get schema client again, If success proceed
		// Else send proper error message in http response
		scErr := SchemaClient()
		if scErr != nil {
			w.WriteHeader(http.StatusInternalServerError)
			responseTaas.Status = "failure"
			responseTaas.ErrMsg = scErr.Error()
			json.NewEncoder(w).Encode(responseTaas)
			statusCode = http.StatusInternalServerError
			return
		}
	}
	for ; i <= types.TpaasConfigObj.TpaasConfig.App.RetryCount; i++ {
		id, err = client.RegisterNewSchema(subject, regSchema.Schema)
		if err != nil {
			logging.LogConf.Error("Retrying schema-reg.", err.Error())
			if !isErrorRetriable(err) {
				break
			}
			time.Sleep(1 * time.Second)
			continue
		}
		break
	}
	if err != nil {
		logging.LogConf.Error("The HTTP-POST req for SCHEMA reg. failed with error", err.Error())
		eventErr := events.RaiseEvent(regSchema.RecordType+"SchemaRegistrationFailure", []string{"Description"}, []string{"Schema Registry Connection"}, []string{"Error", "STATUS"}, []string{err.Error(), "Failure"})
		if eventErr != nil {
			logging.LogConf.Error("error while notifying Schema Registry Connection failure to FMaaS.", err)
		}
		w.WriteHeader(http.StatusInternalServerError)
		responseTaas.Status = "failure"
		responseTaas.ErrMsg = "Schema Registration Failed"
		json.NewEncoder(w).Encode(responseTaas)
		statusCode = http.StatusInternalServerError
		return
	}
	eventErr := events.RaiseEvent(regSchema.RecordType+"SchemaRegistrationSuccess", []string{"Description"}, []string{"Schema Registry Connection"}, []string{"SchemaId", "STATUS"}, []string{strconv.Itoa(id), "Success"})
	if eventErr != nil {
		logging.LogConf.Error("error while notifying Schema Regostration succeess to FMaaS.", err)
	}
	responseTaas.Status = "success"
	responseTaas.SchemaID = id
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(responseTaas)
	logging.LogConf.Info(regSchema.RecordType, " SCHEMA reg. successfully with Id", id)
}

func StartConsumer(topic string) error {
	consumer := conn.TrlConsumer
	if topic == "SLT" {
		consumer = conn.SltConsumer
	}
	consumer.Subscribe(topic, nil)
	for {
		message, err := consumer.ReadMessage(-1)
		if err != nil {
			logging.LogConf.Error("Error while consuming message from", topic, "topic", err)
			continue
		}
		logging.LogConf.Debug("ConsumeMessage: Sending message to topic:", topic)

		err = promutil.CounterAdd("tpaas_total_messages_consumed", 1, map[string]string{"topic": topic})
		if err != nil {
			logging.LogConf.Error("Error while adding values in counter for tpaas_total_messages_consumed by TPaaS.", err)
		}

		err = sendMesssage(message, topic)
		if err == nil {
			_, e := consumer.CommitMessage(message)
			if e != nil {
				logging.LogConf.Error("error while msg commit", e)
				continue
			}
			logging.LogConf.Debug("commit successful", message.TopicPartition.Offset, string(message.Value))

		}
	}
	return nil
}

func getTmaasHeadersFromKafka(messageHeaders []kafka.Header) (*TmaaSAnnotations, error) {
	var tmaas *TmaaSAnnotations
	for _, value := range messageHeaders {
		if value.Key == "TMAAS_HEADERS" {

			err := json.Unmarshal(value.Value, &tmaas)
			if err != nil {
				logging.LogConf.Error("error while unmarshaling kafka response-tmaas headers", err.Error())
				return nil, err
			}
			break

		}
	}
	return tmaas, nil
}

func sendMesssage(message *kafka.Message, topic string) error {
	var ntopic string
	tmaas, err := getTmaasHeadersFromKafka(message.Headers)
	if err != nil {
		logging.LogConf.Error("unable to get Tmaas Headers", err.Error())
		return err
	}
	if tmaas == nil {
		logging.LogConf.Error("Tmaas header missing in kafka header")
		return errors.New("tmaas header missing in kafka header")
	}
	// connect to kafka
	if topic == "TRL" {
		ntopic = TrlPrefix + tmaas.NfType
	} else {
		ntopic = SltPrefix + tmaas.NfType
	}

	logging.LogConf.Debug("Writing to analytic kafka on topic", ntopic)
	conn.Push(message.Value, ntopic)

	counter := "tpaas_trldata_handled_total"
	if topic == "SLT" {
		counter = "tpaas_sltdata_handled_total"

	}
	// counter increment for tpaas_trldata_handled_total/tpaas_sltdata_handled_total
	err = promutil.CounterAdd(counter, 1, map[string]string{"mNfType": tmaas.NfType, "mNfId": tmaas.NfID})
	if err != nil {
		logging.LogConf.Error("Error while adding values in  counter  for Total number of", topic, "messages processed by TPaaS.", err)
	}
	return nil
}

//Remove empty string from array of kafka brokers
func delete_empty(s []string) []string {
	var r []string
	for _, str := range s {
		if str != "" {
			r = append(r, str)
		}
	}
	return r
}
