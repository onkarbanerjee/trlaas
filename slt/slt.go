package slt

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
	"trlaas/etcd"
	"trlaas/logging"

	"github.com/coreos/etcd/clientv3"
	"github.com/gorilla/mux"
)

const (
	running                 state  = "RUNNING"
	suspended               state  = "SUSPENDED"
	sessionsKeyPrefix       string = "/tpaas/sessions"
	nfOpsRespWatchKeyPrefix string = "/operations/response"
)

type param struct {
	transactionID,
	traceSessionID string
	svcAtrrs []attr
}
type attr struct {
	NfName         string `json:"nfName"`
	NfsName        string `json:"nfsName"`
	SvcVersion     string `json:"svcVersion"`
	InvocationMode string `json:"invocationMode"`
	TTL            int    `json:"ttl"`
	ServerURL      string `json:"serverUrl"`
	Method         string `json:"method"`
	PodCount       int    `json:"podCount"`
}

type state string

type operationReq struct {
	NfName        string            `json:"nfName"`
	NfsName       string            `json:"nfsName"`
	TransactionID string            `json:"transactionID"`
	Request       string            `json:"reqBody"`
	Headers       map[string]string `json:"headers"`
	URL           string            `json:"url"`
	Method        string            `json:"method"`
}

// Session is session
type Session struct {
	TraceSessionID   string   `json:"traceSessionId"`
	TraceObjectType  string   `json:"traceObjectType"`
	TraceObjectValue string   `json:"traceObjectValue"`
	TraceInterfaces  []string `json:"traceInterfaces"`
	TraceDepth       string   `json:"traceDepth,omitempty"`
}

type traceSession struct {
	NfName  string            `json:"nfId"`
	NfsName []string          `json:"nfsId,omitempty"`
	State   state             `json:"state"`
	Headers map[string]string `json:"headers,omitempty"`
	URL     string            `json:"url,omitempty"`
	Method  string            `json:"method,omitempty"`
	Session
	OpURL     string            `json:"opURL,omitempty"`
	ServerURL map[string]string `json:"serverURL,omitempty"`
}

type problemDetails struct {
	Cause         string `json:"cause"`
	Status        int    `json:"status"`
	Detail        string `json:"detail"`
	InvalidParams []struct {
		Param string `json:"param"`
	} `json:"invalidParams,omitempty"`
}

type NFFullResponse struct {
	TxnID    string `json:"txnId"`
	Response string `json:"response"`
}

//OpResponse operation response
type NFResponse struct {
	NFServiceInstance     string `json:"NFServiceInstance"`
	OperationResponsebody string `json:"Operation-ResponseBody"`
	OperationStatusCode   string `json:"Operation-StatusCode"`
}
type delaySlt struct {
	*sync.RWMutex
	seconds int
}

var (
	dst                   = delaySlt{&sync.RWMutex{}, 0}
	EtcdResEnabled string = "false"
	EtcdTimeOutSec        = 15
	nfResponses           = sync.Map{}
	nfResponseKeys        = sync.Map{}
)

func getParams(r *http.Request) (*param, error) {
	params := param{
		transactionID:  r.Header.Get("X-Operation-TransactionId"),
		traceSessionID: mux.Vars(r)["traceSessionID"],
	}

	var attrs []attr
	err := json.Unmarshal([]byte(r.Header.Get("X-Operation-Service-Attributes")), &attrs)
	if err != nil {
		return nil, err
	}

	params.svcAtrrs = attrs
	return &params, nil
}

func getSessionsData(ctx context.Context, nfName, traceSessionID string) (string, *clientv3.GetResponse, error) {
	// make the sessionsKey
	sessionsKey := fmt.Sprintf("%s/%s", sessionsKeyPrefix, nfName)

	opts := []clientv3.OpOption{
		clientv3.WithPrefix(),
		clientv3.WithSort(clientv3.SortByCreateRevision, clientv3.SortDescend),
	}

	if traceSessionID != "" {
		sessionsKey = fmt.Sprintf("%s/%s", sessionsKey, traceSessionID)
		opts = []clientv3.OpOption{
			clientv3.WithLimit(1),
		}
		logging.LogConf.Debug("updated opts")
	}

	logging.LogConf.Debug("sessionsKey is", sessionsKey)
	result, err := etcd.Client.Get(ctx, sessionsKey, opts...)
	return sessionsKey, result, err
}

func putSessionsData(ctx context.Context, sessionsKey string, traceSessionRecord *traceSession) (*clientv3.PutResponse, error) {

	sessionsRecord, err := json.Marshal(traceSessionRecord)
	if err != nil {
		return nil, err
	}

	return etcd.Client.Put(ctx, sessionsKey, string(sessionsRecord))
}

func getClientV3Ops(attrib *attr, leaseID clientv3.LeaseID, transactionID, traceSessionID string, reqBody []byte) (op clientv3.Op, err error) {
	// get the regular key
	regularKey := fmt.Sprintf("/operations/request/%s/%s/%s/%s/%s",
		attrib.NfName, attrib.NfsName, attrib.SvcVersion, attrib.InvocationMode, transactionID)

	// make an operation request
	opReq := &operationReq{
		NfName:        attrib.NfName,
		NfsName:       attrib.NfsName,
		TransactionID: transactionID,
		Headers:       map[string]string{"X-Operation-TransactionId": transactionID},
		URL:           strings.TrimRight(attrib.ServerURL, "/") + "/slt/sessions/" + traceSessionID,
		Method:        attrib.Method,
	}

	if len(reqBody) > 0 {
		opReq.Request = string(reqBody)
	}

	regularValue, err := json.Marshal(opReq)
	if err != nil {
		return
	}
	op = clientv3.OpPut(regularKey, string(regularValue), clientv3.WithLease(leaseID))
	return
}

func writeProblemDetails(w http.ResponseWriter, p *problemDetails) {
	logging.LogConf.Debug("Sending problems details", *p)

	w.Header().Add("Content-Type", "application/problem+json")
	w.WriteHeader(p.Status)
	json.NewEncoder(w).Encode(p)
}

func revokeLeases(ctx context.Context, leaseIds []clientv3.LeaseID) (rvFailedIds []clientv3.LeaseID) {
	for _, val := range leaseIds {
		_, err := etcd.Client.Revoke(ctx, val)
		if err != nil {
			rvFailedIds = append(rvFailedIds, val)
			logging.LogConf.Error("Lease Revoke failure for llease id", val, "Error", err)
		}
	}
	return
}

func validateSessionData(traceSessionID string, sessionData Session) error {
	if sessionData.TraceSessionID == "" {
		return errors.New("TraceSessionID is empty in request body")
	}
	if traceSessionID != sessionData.TraceSessionID {
		logging.LogConf.Error("Trace id in body =", sessionData.TraceSessionID, "Trace id in path =", traceSessionID)
		return errors.New("TraceSessionID expected in request body and to be same as in path param")
	}
	if sessionData.TraceObjectValue == "" {
		return errors.New("TraceObjectValue is empty in request body")
	}
	if sessionData.TraceObjectType == "" {
		return errors.New("TraceObjectType is empty in request body")
	}
	if len(sessionData.TraceInterfaces) <= 0 {
		return errors.New("TraceInterfaces is empty in request body")
	}
	return nil
}

// Create etcd watcher on /operations/response
func GetEtcdResponses() {

	logging.LogConf.Info("Start watching on NF operations response key", nfOpsRespWatchKeyPrefix)
	ctx := context.Background()
	opts := []clientv3.OpOption{
		clientv3.WithPrefix(),
		// clientv3.WithSort(clientv3.SortByCreateRevision, clientv3.SortDescend),
	}
	logging.LogConf.Debug(fmt.Sprintf("opts is %+v", opts))

	for {
		// A channel to watch over the etcd NF response key. If there are any updates(PUT, DELETE, etc) against this key in etcd then this channel will notify it.
		watchChan := etcd.Client.Watch(ctx, nfOpsRespWatchKeyPrefix, opts...)
		logging.LogConf.Debug("watchChan is:", watchChan)
		if watchChan == nil {
			logging.LogConf.Error("Operations watch channel is nil")
			// return nfResponses, errors.New("operations watch channel is nil")
			continue
		}
		logging.LogConf.Debug("watchChan is not nil")

		for watchResp := range watchChan {
			logging.LogConf.Debug(fmt.Sprintf("watchResp is %+v", watchResp))
			logging.LogConf.Debug(fmt.Sprintf("No of events is - %v, event is %+v", len(watchResp.Events), watchResp.Events))
			for _, event := range watchResp.Events {
				logging.LogConf.Debug("Event type is - ", fmt.Sprintf("%s", event.Type))
				// We are only concerned about PUT event for the key. For other events we ignore.
				eventType := fmt.Sprintf("%s", event.Type)
				if eventType != "PUT" {
					continue
				}
				logging.LogConf.Debug(fmt.Sprintf("PUT event received. %+v", event))

				logging.LogConf.Info("NF response received!", " Key: ", string(event.Kv.Key[:]), " value: ", string(event.Kv.Value[:]))
				//Get the full NF response which contains txnId, response body and status
				// which is like this - {"txnId":"fbe6689f-53f1-40a9-873a-f52f57f66f9a","response":"{\"Operation-ResponseBody\":\"\",\"Operation-StatusCode\":\"204\"}"}
				var opFullRes NFFullResponse
				err := json.Unmarshal(event.Kv.Value, &opFullRes)
				if err != nil {
					logging.LogConf.Error("Error while unmarshalling the full NF response")
				}
				// Get the nf reponse body and status which is present under reponse key. "response":"{\"Operation-ResponseBody\":\"\",\"Operation-StatusCode\":\"204\"}"
				var opRes NFResponse
				err = json.Unmarshal([]byte(opFullRes.Response), &opRes)
				if err != nil {
					logging.LogConf.Error("Error while unmarshalling the NF response body and status code")
				}
				logging.LogConf.Info("Successfully parsed NF response : ", opRes)
				opRes.NFServiceInstance = strings.Split(string(event.Kv.Key[:]), "/")[6]

				nfRespKeyNfName := strings.Split(string(event.Kv.Key[:]), "/")[3]
				if _, ok := nfResponseKeys.Load(nfRespKeyNfName); !ok {
					continue
				}

				var resp []NFResponse
				val, ok := nfResponses.Load(nfRespKeyNfName)
				if !ok {
					logging.LogConf.Debug("Initial nfresponse Recieved for key", nfRespKeyNfName)
				} else {
					resp = val.([]NFResponse)
				}
				resp = append(resp, opRes)
				nfResponses.Store(nfRespKeyNfName, resp)
				logging.LogConf.Debug("Number of nf Responses Recieved for key", nfRespKeyNfName, len(resp))
				// delete the NF response key from etcd
				wtctx, cancel := context.WithTimeout(ctx, time.Second*time.Duration(EtcdTimeOutSec))
				defer cancel()
				_, err = etcd.Client.Delete(wtctx, string(event.Kv.Key[:]))
				if err != nil {
					logging.LogConf.Error("Error deleting NF response key from etcd", err)
				}
			}
		}
	}
}

// This function watches and collates the NF responses.
func getNFResponses(attrs []attr) (resp []NFResponse, err error) {
	// var nfResponses = []NFResponse{}
	var podCount = 0
	// NFs can have multiple services (test-appa, test-appb, etc.) and each service can have multiple versions (eg. v0, v1, etc.) which we receive in TPaaS
	// in "X-Operation-Service-Attributes" header which is an array.
	// Each service version has it's own podCount.
	// If there are multiple versions of services then we need to collate responses from all the pods in each version in each service. That's why we need this loop.
	for _, at := range attrs {
		podCount += at.PodCount
	}

	val, ok := nfResponses.Load(attrs[0].NfName)
	logging.LogConf.Debug("In getNfResponse key", attrs[0].NfName, val)
	// We cannot proceed if we received podCount as zero.
	if podCount == 0 {
		logging.LogConf.Error("NF has zero pods!")
		err = errors.New("nF has zero pods")
		return
	}

	// Time-out would be floor of 90% of the TTL value. TTL is number of seconds
	nfRespTimeOutDuration := time.Duration(int(math.Floor(float64(attrs[0].TTL) * 0.9)))

	timeOut := make(chan bool)
	defer close(timeOut)

	go func(timeOut chan bool) {
		select {
		case <-time.After(time.Second * nfRespTimeOutDuration):
			logging.LogConf.Info("Request timed out. Stop watching the NF responses.")
			timeOut <- true
			return
		case <-timeOut:
			logging.LogConf.Info("Recieved nf Responses, Stop timeOutDuration watcher.")
			return
		}
	}(timeOut)

OUTER:

	for {
		logging.LogConf.Debug("In getNfResponse", len(resp))
		select {
		case <-timeOut:
			break OUTER
		default:
			if val, ok = nfResponses.Load(attrs[0].NfName); !ok {
				logging.LogConf.Debug("No response recieved yet from nf's for ", attrs[0].NfName)
				continue
			}
			resp = val.([]NFResponse)
			if len(resp) >= podCount {
				logging.LogConf.Debug("In getNfResponse Inside If", len(resp))
				timeOut <- false
				break OUTER
			}
		}
	}
	// In ideal case we are expecting the number of responses to be equal to the "podCount" value.
	// If we receive at least one response (due to context timeout), we consider it as success.
	// Only if there are zero repsonses we consider as failure.
	val, _ = nfResponses.Load(attrs[0].NfName)
	resp = val.([]NFResponse)
	if len(resp) == 0 {
		err = errors.New("got no response from NF")
	}
	nfResponses.Delete(attrs[0].NfName)
	nfResponseKeys.Delete(attrs[0].NfName)
	return
}

// This funcion checks if any nf responded with success status code. Success status codes are defined in ENV variable - "SLT_NF_SUCCESS_RESPONSE_STATUS_CODES". For eg. "200|204".
// If any nf reponded with success status code then we consider it as succcess scenario.
func isNFResponseSuccess(nfResponses []NFResponse) bool {
	nfSuccessReponseStatusCodes := os.Getenv("SLT_NF_SUCCESS_RESPONSE_STATUS_CODES")
	// If the success status codes are not defined in env variable then we consider only "200" & "204" as success status codes.
	if len(nfSuccessReponseStatusCodes) == 0 {
		nfSuccessReponseStatusCodes = "200|204"
	}
	for _, nfResp := range nfResponses {
		if strings.Contains(nfSuccessReponseStatusCodes, nfResp.OperationStatusCode) {
			return true
		}
	}
	return false
}

// Make a call to cim api to add the operation response key.
func report(postBody interface{}, statusCode int, transactionID string) error {
	cimport := "6060"
	postBodyJSON, err := json.Marshal(postBody)
	if err != nil {
		return errors.New("error posting operation response to CIM - postBody marshalling error -" + err.Error())
	}
	//To handle empty response in case of stop after pause scenario
	if len(fmt.Sprint(postBody)) == 0 {
		postBodyJSON = []byte("")
	}
	request, error := http.NewRequest("POST", "http://localhost:"+cimport+"/api/v1/_operations/report", bytes.NewBuffer(postBodyJSON))
	if error != nil {
		return errors.New("error posting operation response to CIM - error creating post request(http.NewRequest) - " + err.Error())
	}
	request.Header.Set("X-Operation-StatusCode", strconv.Itoa(statusCode))
	request.Header.Set("X-Operation-TransactionId", transactionID)

	response, error := http.DefaultClient.Do(request)
	if error != nil {
		return errors.New("error posting operation response to CIM - error making post request(http.DefaultClient.Do) - " + err.Error())
	}
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("got resp status code %d from CIM", response.StatusCode)
	}
	logging.LogConf.Info("TPaaS operation response key successfully added by CIM")
	return nil
}
