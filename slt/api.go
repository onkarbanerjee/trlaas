package slt

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
	"trlaas/etcd"
	"trlaas/logging"
	"trlaas/promutil"
	"trlaas/types"

	"github.com/coreos/etcd/clientv3"
	"github.com/gorilla/mux"
)

// start handles start
func start(r *http.Request) {
	nfId := "NA"
	statusCode := http.StatusOK

	defer func() {
		// counter increment for tpaas_slt_requests_total
		err := promutil.CounterAdd("tpaas_slt_requests_total", 1, map[string]string{"mNfId": nfId, "statusCode": strconv.Itoa(statusCode), "action": "start"})
		if err != nil {
			logging.LogConf.Error("Error while adding values in  counter  for Total number of operational SLT requests recieved by TPaaS.", err)
		}
	}()

	if strings.Contains(EtcdResEnabled, "true") {
		logging.LogConf.Info("[SLT-Start] - Going to sleep for seconds -", dst.seconds)
		time.Sleep(time.Duration(dst.seconds) * time.Second)
		logging.LogConf.Info("[SLT-Start] - Sleep period over")
	}
	// get the attrs from headers
	params, err := getParams(r)
	if err != nil {
		logging.LogConf.Error("[SLT-Start] Params error", err.Error())
		//Add ops response key using cim api
		err = report(&problemDetails{fmt.Sprintf("Params error: %s", err), http.StatusBadRequest, "", nil}, http.StatusBadRequest, r.Header.Get("X-Operation-TransactionId"))
		if err != nil {
			logging.LogConf.Error(err.Error())
		}
		// counter increment for tpaas_slt_sessions_error_total
		err = promutil.CounterAdd("tpaas_slt_sessions_error_total", 1, map[string]string{"sourceIP": r.RemoteAddr, "nfId": "NA", "traceSessionId": "NA", "action": "start", "responseCode": strconv.Itoa(http.StatusBadRequest)})
		if err != nil {
			logging.LogConf.Error("Error while adding values in  counter  for Total number of SLT session requests received by TPaaS.", err)
		}
		statusCode = http.StatusBadRequest
		return
	}
	logging.LogConf.Debug("Params =", fmt.Sprintf("%+v", params), "trace id from path =", params.traceSessionID)

	// check if svc attr present or not
	if len(params.svcAtrrs) == 0 {
		logging.LogConf.Error("[SLT-Start] svc attr Not found")
		//Add ops response key using cim api
		err = report(&problemDetails{"svc attr Not found", http.StatusBadRequest, "", nil}, http.StatusBadRequest, r.Header.Get("X-Operation-TransactionId"))
		if err != nil {
			logging.LogConf.Error(err.Error())
		}
		// counter increment for tpaas_slt_sessions_error_total
		err = promutil.CounterAdd("tpaas_slt_sessions_error_total", 1, map[string]string{"sourceIP": r.RemoteAddr, "nfId": "NA", "traceSessionId": "NA", "action": "start", "responseCode": strconv.Itoa(http.StatusBadRequest)})
		if err != nil {
			logging.LogConf.Error("Error while adding values in  counter  for Total number of SLT session requests received by TPaaS.", err)
		}
		statusCode = http.StatusBadRequest
		return
	}

	// counter increment for tpaas_slt_sessions_total
	err = promutil.CounterAdd("tpaas_slt_sessions_total", 1, map[string]string{"sourceIP": r.RemoteAddr, "nfId": params.svcAtrrs[0].NfName, "traceSessionId": params.traceSessionID, "action": "start"})
	if err != nil {
		logging.LogConf.Error("Error while adding values in  counter  for Total number of SLT session requests received by TPaaS.", err)
	}

	// get the session data from the request
	sessionData := Session{}
	requestData, err := ioutil.ReadAll(r.Body)
	if err != nil {
		logging.LogConf.Error("[SLT-Start] Error reading request body", err.Error())
		//Add ops response key using cim api
		err = report(&problemDetails{fmt.Sprintf("Get session data from request error:%s", err), http.StatusBadRequest, "", nil}, http.StatusBadRequest, r.Header.Get("X-Operation-TransactionId"))
		if err != nil {
			logging.LogConf.Error(err.Error())
		}
		// counter increment for tpaas_slt_sessions_error_total
		err = promutil.CounterAdd("tpaas_slt_sessions_error_total", 1, map[string]string{"sourceIP": r.RemoteAddr, "nfId": "NA", "traceSessionId": "NA", "action": "start", "responseCode": strconv.Itoa(http.StatusBadRequest)})
		if err != nil {
			logging.LogConf.Error("Error while adding values in  counter  for Total number of SLT session requests received by TPaaS.", err)
		}
		statusCode = http.StatusBadRequest
		return
	}

	defer r.Body.Close()
	logging.LogConf.Debug("Request body =", string(requestData))

	nfId = params.svcAtrrs[0].NfName
	//unmarshall session request body to sessio data
	err = json.Unmarshal(requestData, &sessionData)
	if err != nil {
		logging.LogConf.Debug("[SLT-Start] Unmarshalling request data error", err)

		//Add ops response key using cim api
		err = report(&problemDetails{"Internal server error to get session data from request error", http.StatusBadRequest, "", nil}, http.StatusBadRequest, r.Header.Get("X-Operation-TransactionId"))
		if err != nil {
			logging.LogConf.Error(err.Error())
		}
		// counter increment for tpaas_slt_sessions_error_total
		err = promutil.CounterAdd("tpaas_slt_sessions_error_total", 1, map[string]string{"sourceIP": r.RemoteAddr, "nfId": params.svcAtrrs[0].NfName, "traceSessionId": params.traceSessionID, "action": "start", "responseCode": strconv.Itoa(http.StatusBadRequest)})
		if err != nil {
			logging.LogConf.Error("Error while adding values in  counter  for Total number of SLT session requests received by TPaaS.", err)
		}
		statusCode = http.StatusBadRequest
		return
	}

	// validate input fields
	err = validateSessionData(params.traceSessionID, sessionData)
	if err != nil {
		logging.LogConf.Error("[SLT-Start] Input field validation error", err)

		//Add ops response key using cim api
		err = report(&problemDetails{fmt.Sprint(err.Error()), http.StatusBadRequest, "", nil}, http.StatusBadRequest, r.Header.Get("X-Operation-TransactionId"))
		if err != nil {
			logging.LogConf.Error(err.Error())
		}
		// counter increment for tpaas_slt_sessions_error_total
		err = promutil.CounterAdd("tpaas_slt_sessions_error_total", 1, map[string]string{"sourceIP": r.RemoteAddr, "nfId": params.svcAtrrs[0].NfName, "traceSessionId": params.traceSessionID, "action": "start", "responseCode": strconv.Itoa(http.StatusBadRequest)})
		if err != nil {
			logging.LogConf.Error("Error while adding values in  counter  for Total number of SLT session requests received by TPaaS.", err)
		}
		statusCode = http.StatusBadRequest
		return
	}
	// get all sessions data  for the given nf
	ctx := context.Background()
	wtctx, cancel := context.WithTimeout(ctx, time.Second*time.Duration(EtcdTimeOutSec))
	defer cancel()

	t := time.Now()
	sessionsKey, result, err := getSessionsData(wtctx, params.svcAtrrs[0].NfName, "")
	logging.LogConf.Debug("[SLT-Start] Time taken to get session data for trace session ID", params.traceSessionID, "Is", time.Since(t).Seconds())
	if err != nil {
		logging.LogConf.Error("[SLT-Start] Get sessions data error:", err)

		//Add ops response key using cim api
		err = report(&problemDetails{"Get sessions data error", http.StatusInternalServerError, "", nil}, http.StatusInternalServerError, r.Header.Get("X-Operation-TransactionId"))
		if err != nil {
			logging.LogConf.Error(err.Error())
		}
		// counter increment for tpaas_slt_sessions_error_total
		err = promutil.CounterAdd("tpaas_slt_sessions_error_total", 1, map[string]string{"sourceIP": r.RemoteAddr, "nfId": params.svcAtrrs[0].NfName, "traceSessionId": params.traceSessionID, "action": "start", "responseCode": strconv.Itoa(http.StatusInternalServerError)})
		if err != nil {
			logging.LogConf.Error("Error while adding values in  counter  for Total number of SLT session requests received by TPaaS.", err)
		}
		statusCode = http.StatusInternalServerError
		return
	}

	logging.LogConf.Debug("[SLT-Start] configured MaxActivePerNf", types.TpaasConfigObj.TpaasConfig.Slt.MaxActivePerNf)
	if result != nil && result.Count >= int64(types.TpaasConfigObj.TpaasConfig.Slt.MaxActivePerNf) {
		logging.LogConf.Debug("[SLT-Start] etcd get len results", result.Count)

		//Add ops response key using cim api
		err = report(&problemDetails{fmt.Sprintf("Max %d sessions existing already", types.TpaasConfigObj.TpaasConfig.Slt.MaxActivePerNf), http.StatusBadRequest, "", nil}, http.StatusBadRequest, r.Header.Get("X-Operation-TransactionId"))
		if err != nil {
			logging.LogConf.Error(err.Error())
		}
		// counter increment for tpaas_slt_sessions_error_total
		err = promutil.CounterAdd("tpaas_slt_sessions_error_total", 1, map[string]string{"sourceIP": r.RemoteAddr, "nfId": params.svcAtrrs[0].NfName, "traceSessionId": params.traceSessionID, "action": "start", "responseCode": strconv.Itoa(http.StatusBadRequest)})
		if err != nil {
			logging.LogConf.Error("Error while adding values in  counter  for Total number of SLT session requests received by TPaaS.", err)
		}
		statusCode = http.StatusBadRequest
		return
	}
	logging.LogConf.Debug("[SLT-Start] count:", result.Count)

	sessionsKey = fmt.Sprintf("%s/%s", sessionsKey, params.traceSessionID)
	for _, kv := range result.Kvs {
		logging.LogConf.Debug("[SLT-Start] string(kv.Key)", string(kv.Key))
		if string(kv.Key) == sessionsKey {
			logging.LogConf.Debug("[SLT-Start] etcd get results", string(kv.Key))

			//Add ops response key using cim api
			err = report(&problemDetails{"Session already exist", http.StatusBadRequest, "", nil}, http.StatusBadRequest, r.Header.Get("X-Operation-TransactionId"))
			if err != nil {
				logging.LogConf.Error(err.Error())
			}
			// counter increment for tpaas_slt_sessions_error_total
			err = promutil.CounterAdd("tpaas_slt_sessions_error_total", 1, map[string]string{"sourceIP": r.RemoteAddr, "nfId": params.svcAtrrs[0].NfName, "traceSessionId": params.traceSessionID, "action": "start", "responseCode": strconv.Itoa(http.StatusBadRequest)})
			if err != nil {
				logging.LogConf.Error("Error while adding values in  counter  for Total number of SLT session requests received by TPaaS.", err)
			}
			statusCode = http.StatusBadRequest
			return
		}
	}

	ops := []clientv3.Op{}
	var leaseIds []clientv3.LeaseID
	nfsNames := []string{}
	for _, attr := range params.svcAtrrs {
		//create lease
		t = time.Now()
		lease, err := etcd.Client.Grant(wtctx, int64(attr.TTL))
		logging.LogConf.Debug("[SLT-Start] Time taken to grant lease for trace session ID", params.traceSessionID, "Is", time.Since(t).Seconds())
		if err != nil {
			logging.LogConf.Error("[SLT-Start] Error while creating lease", err)

			//Add ops response key using cim api
			err = report(&problemDetails{"Put operations request error", http.StatusInternalServerError, "", nil}, http.StatusInternalServerError, r.Header.Get("X-Operation-TransactionId"))
			if err != nil {
				logging.LogConf.Error(err.Error())
			}
			rvFailedIds := revokeLeases(wtctx, leaseIds)
			if len(rvFailedIds) > 0 {
				logging.LogConf.Debug("[SLT-Start] Lease revoke failed for Id's", rvFailedIds)
			}
			// counter increment for tpaas_slt_sessions_error_total
			err = promutil.CounterAdd("tpaas_slt_sessions_error_total", 1, map[string]string{"sourceIP": r.RemoteAddr, "nfId": params.svcAtrrs[0].NfName, "traceSessionId": params.traceSessionID, "action": "start", "responseCode": strconv.Itoa(http.StatusInternalServerError)})
			if err != nil {
				logging.LogConf.Error("Error while adding values in  counter  for Total number of SLT session requests received by TPaaS.", err)
			}
			statusCode = http.StatusInternalServerError
			return
		}
		logging.LogConf.Debug("[SLT-Start] clientv3.WithLease(resp.ID)", lease.ID)
		leaseIds = append(leaseIds, lease.ID)
		// get ClientV3 Ops
		attr.Method = http.MethodPost
		op, err := getClientV3Ops(&attr, lease.ID, params.transactionID, params.traceSessionID, requestData)
		if err != nil {
			logging.LogConf.Debug("[SLT-Start] Put operations request error:", err)

			//Add ops response key using cim api
			err = report(&problemDetails{"Put operations request error", http.StatusInternalServerError, "", nil}, http.StatusInternalServerError, r.Header.Get("X-Operation-TransactionId"))
			if err != nil {
				logging.LogConf.Error(err.Error())
			}
			rvFailedIds := revokeLeases(wtctx, leaseIds)
			if len(rvFailedIds) > 0 {
				logging.LogConf.Debug("[SLT-Start] Lease revoke failed for Id's", rvFailedIds)
			}
			// counter increment for tpaas_slt_sessions_error_total
			err = promutil.CounterAdd("tpaas_slt_sessions_error_total", 1, map[string]string{"sourceIP": r.RemoteAddr, "nfId": params.svcAtrrs[0].NfName, "traceSessionId": params.traceSessionID, "action": "start", "responseCode": strconv.Itoa(http.StatusInternalServerError)})
			if err != nil {
				logging.LogConf.Error("Error while adding values in  counter  for Total number of SLT session requests received by TPaaS.", err)
			}
			statusCode = http.StatusInternalServerError
			return
		}
		ops = append(ops, op)
		nfsNames = append(nfsNames, attr.NfsName)
	}

	//Update a resp key in map, etcd watcher can ignore if unwanted keys occured.
	nfResponseKeys.Store(params.svcAtrrs[0].NfName, true)

	t = time.Now()
	_, err = etcd.Client.Txn(wtctx).Then(ops...).Commit()
	logging.LogConf.Debug("[SLT-Start] Time taken to commit transaction for trace session ID", params.traceSessionID, "Is", time.Since(t).Seconds())
	if err != nil {
		logging.LogConf.Debug("[SLT-Start] Commit Operations Request error", err)

		//Add ops response key using cim api
		err = report(&problemDetails{"Commit Operations Request error", http.StatusInternalServerError, "", nil}, http.StatusInternalServerError, r.Header.Get("X-Operation-TransactionId"))
		if err != nil {
			logging.LogConf.Error(err.Error())
		}
		// counter increment for tpaas_slt_sessions_error_total
		err = promutil.CounterAdd("tpaas_slt_sessions_error_total", 1, map[string]string{"sourceIP": r.RemoteAddr, "nfId": params.svcAtrrs[0].NfName, "traceSessionId": params.traceSessionID, "action": "start", "responseCode": strconv.Itoa(http.StatusInternalServerError)})
		if err != nil {
			logging.LogConf.Error("Error while adding values in  counter  for Total number of SLT session requests received by TPaaS.", err)
		}
		statusCode = http.StatusInternalServerError
		return
	}
	logging.LogConf.Info("[SLT-Start] Operations Request Commit Success")

	t = time.Now()
	nfResponses, err := getNFResponses(params.svcAtrrs)
	logging.LogConf.Debug("[SLT-Start] Time taken to get nf response for trace session ID", params.traceSessionID, "Is", time.Since(t).Seconds())
	if err != nil {
		logging.LogConf.Debug("[SLT-Start] Error getting response from NF", err)

		//Add ops response key using cim api
		err = report(&problemDetails{fmt.Sprint("Error getting response from NF. ", err.Error()), http.StatusInternalServerError, "", nil}, http.StatusInternalServerError, r.Header.Get("X-Operation-TransactionId"))
		if err != nil {
			logging.LogConf.Error(err.Error())
		}
		// counter increment for tpaas_slt_sessions_error_total
		err = promutil.CounterAdd("tpaas_slt_sessions_error_total", 1, map[string]string{"sourceIP": r.RemoteAddr, "nfId": params.svcAtrrs[0].NfName, "traceSessionId": params.traceSessionID, "action": "start", "responseCode": strconv.Itoa(http.StatusInternalServerError)})
		if err != nil {
			logging.LogConf.Error("Error while adding values in counter for Total number of SLT session requests received by TPaaS.", err)
		}
		statusCode = http.StatusInternalServerError
		return
	}
	logging.LogConf.Info("[SLT-Start] Received and collated response from NF")

	// Check if any nf responded with success status code.
	// Only in success scenario we should touch the tpaas session key in etcd.
	if isNFResponseSuccess(nfResponses) {
		ctxPut := context.Background()
		wtCtxPut, cancelPutCtx := context.WithTimeout(ctxPut, time.Second*time.Duration(EtcdTimeOutSec))
		defer cancelPutCtx()

		// put the sessions data
		t = time.Now()
		serverUrl := make(map[string]string)
		for _, val := range params.svcAtrrs {
			serverUrl[val.NfsName] = strings.TrimRight(val.ServerURL, "/")
		}
		_, err = putSessionsData(wtCtxPut, sessionsKey, &traceSession{
			NfName:    params.svcAtrrs[0].NfName,
			NfsName:   nfsNames,
			State:     running,
			Headers:   map[string]string{"X-Operation-TransactionId": params.transactionID},
			URL:       strings.TrimRight(params.svcAtrrs[0].ServerURL, "/") + "/slt/sessions/" + params.traceSessionID,
			Method:    http.MethodPost,
			Session:   sessionData,
			OpURL:     "/slt/sessions/" + params.traceSessionID,
			ServerURL: serverUrl,
		})
		logging.LogConf.Debug("[SLT-Start] Time taken to put session data for trace session ID", params.traceSessionID, "Is", time.Since(t).Seconds())
		if err != nil {
			logging.LogConf.Debug("[SLT-Start] Put sessions data error", err)

			//Add ops response key using cim api
			err = report(&problemDetails{"Put sessions data error", http.StatusInternalServerError, "", nil}, http.StatusInternalServerError, r.Header.Get("X-Operation-TransactionId"))
			if err != nil {
				logging.LogConf.Error(err.Error())
			}
			// counter increment for tpaas_slt_sessions_error_total
			err = promutil.CounterAdd("tpaas_slt_sessions_error_total", 1, map[string]string{"sourceIP": r.RemoteAddr, "nfId": params.svcAtrrs[0].NfName, "traceSessionId": params.traceSessionID, "action": "start", "responseCode": strconv.Itoa(http.StatusInternalServerError)})
			if err != nil {
				logging.LogConf.Error("Error while adding values in  counter  for Total number of SLT session requests received by TPaaS.", err)
			}
			statusCode = http.StatusInternalServerError
			return
		}
	}

	// Add the collated response from nf to TPaaS response.
	// Call CIMs report api to provide feedback to it so that it can add operations/reponse key for tpaas in etcd.
	err = report(nfResponses, http.StatusOK, r.Header.Get("X-Operation-TransactionId"))
	if err != nil {
		logging.LogConf.Error(err.Error())
	}
}

// stop handles stop
func stop(r *http.Request) {
	nfId := "NA"
	statusCode := http.StatusOK

	defer func() {
		// counter increment for tpaas_slt_requests_total
		err := promutil.CounterAdd("tpaas_slt_requests_total", 1, map[string]string{"mNfId": nfId, "statusCode": strconv.Itoa(statusCode), "action": "stop"})
		if err != nil {
			logging.LogConf.Error("Error while adding values in  counter  for Total number of operational SLT requests recieved by TPaaS.", err)
		}
	}()

	if strings.Contains(EtcdResEnabled, "true") {
		logging.LogConf.Info("[SLT-Stop] - Going to sleep for seconds -", dst.seconds)
		time.Sleep(time.Duration(dst.seconds) * time.Second)
		logging.LogConf.Info("[SLT-Stop] - Sleep period over")
	}

	// get the attrs from the request headers
	params, err := getParams(r)
	if err != nil {
		logging.LogConf.Error("[SLT-Stop] Params error", err.Error())
		//Add ops response key using cim api
		err = report(&problemDetails{fmt.Sprintf("Params error: %s", err), http.StatusBadRequest, "", nil}, http.StatusBadRequest, r.Header.Get("X-Operation-TransactionId"))
		if err != nil {
			logging.LogConf.Error(err.Error())
		}
		// counter increment for tpaas_slt_sessions_error_total
		err = promutil.CounterAdd("tpaas_slt_sessions_error_total", 1, map[string]string{"sourceIP": r.RemoteAddr, "nfId": "NA", "traceSessionId": "NA", "action": "stop", "responseCode": strconv.Itoa(http.StatusBadRequest)})
		if err != nil {
			logging.LogConf.Error("Error while adding values in  counter  for Total number of SLT session requests received by TPaaS.", err)
		}
		statusCode = http.StatusBadRequest
		return
	}

	// check if svc attr present or not
	if len(params.svcAtrrs) == 0 {
		logging.LogConf.Error("[SLT-Stop] svc attr Not found")
		//Add ops response key using cim api
		err = report(&problemDetails{"svc attr Not found", http.StatusBadRequest, "", nil}, http.StatusBadRequest, r.Header.Get("X-Operation-TransactionId"))
		if err != nil {
			logging.LogConf.Error(err.Error())
		}
		// counter increment for tpaas_slt_sessions_error_total
		err = promutil.CounterAdd("tpaas_slt_sessions_error_total", 1, map[string]string{"sourceIP": r.RemoteAddr, "nfId": "NA", "traceSessionId": "NA", "action": "stop", "responseCode": strconv.Itoa(http.StatusBadRequest)})
		if err != nil {
			logging.LogConf.Error("Error while adding values in  counter  for Total number of SLT session requests received by TPaaS.", err)
		}
		statusCode = http.StatusBadRequest
		return
	}

	nfId = params.svcAtrrs[0].NfName
	// counter increment for tpaas_slt_sessions_total
	err = promutil.CounterAdd("tpaas_slt_sessions_total", 1, map[string]string{"sourceIP": r.RemoteAddr, "nfId": params.svcAtrrs[0].NfName, "traceSessionId": params.traceSessionID, "action": "stop"})
	if err != nil {
		logging.LogConf.Error("Error while adding values in  counter  for Total number of SLT session requests received by TPaaS.", err)
	}

	// get sessions data
	ctx := context.Background()
	wtctx, cancel := context.WithTimeout(ctx, time.Second*time.Duration(EtcdTimeOutSec))
	defer cancel()

	t := time.Now()
	sessionsKey, result, err := getSessionsData(wtctx, params.svcAtrrs[0].NfName, params.traceSessionID)
	logging.LogConf.Debug("[SLT-Stop] Get is done", err, result, "time taken to get session data for trace session ID", params.traceSessionID, "Is", time.Since(t).Seconds())
	if err != nil {
		logging.LogConf.Error("[SLT-Stop] Get sessions data error:", err)

		//Add ops response key using cim api
		err = report(&problemDetails{"Get sessions data error", http.StatusInternalServerError, "", nil}, http.StatusInternalServerError, r.Header.Get("X-Operation-TransactionId"))
		if err != nil {
			logging.LogConf.Error(err.Error())
		}
		// counter increment for tpaas_slt_sessions_error_total
		err = promutil.CounterAdd("tpaas_slt_sessions_error_total", 1, map[string]string{"sourceIP": r.RemoteAddr, "nfId": params.svcAtrrs[0].NfName, "traceSessionId": params.traceSessionID, "action": "stop", "responseCode": strconv.Itoa(http.StatusInternalServerError)})
		if err != nil {
			logging.LogConf.Error("Error while adding values in  counter  for Total number of SLT session requests received by TPaaS.", err)
		}
		statusCode = http.StatusInternalServerError
		return
	}

	// check if session not found
	if result == nil || len(result.Kvs) == 0 {
		logging.LogConf.Debug("[SLT-Stop] Get sessions data error: result.Kvs", result.Kvs)
		//Add ops response key using cim api
		err = report(&problemDetails{"Session not found", http.StatusBadRequest, "", nil}, http.StatusBadRequest, r.Header.Get("X-Operation-TransactionId"))
		if err != nil {
			logging.LogConf.Error(err.Error())
		}
		err = promutil.CounterAdd("tpaas_slt_sessions_error_total", 1, map[string]string{"sourceIP": r.RemoteAddr, "nfId": params.svcAtrrs[0].NfName, "traceSessionId": params.traceSessionID, "action": "stop", "responseCode": strconv.Itoa(http.StatusBadRequest)})
		if err != nil {
			logging.LogConf.Error("Error while adding values in  counter  for Total number of SLT session requests received by TPaaS.", err)
		}
		statusCode = http.StatusBadRequest
		return
	}

	// start-MTCILMWP-3980: fix stop not working after pause
	// Get the session record to find the session status
	traceSessionsRecord := traceSession{}
	err = json.Unmarshal(result.Kvs[0].Value, &traceSessionsRecord)
	if err != nil {
		logging.LogConf.Debug("[SLT-Stop] Session data error", err)
		//Add ops response key using cim api
		err = report(&problemDetails{"Session data error", http.StatusInternalServerError, "", nil}, http.StatusInternalServerError, r.Header.Get("X-Operation-TransactionId"))
		if err != nil {
			logging.LogConf.Error(err.Error())
		}
		// counter increment for tpaas_slt_sessions_error_total
		err = promutil.CounterAdd("tpaas_slt_sessions_error_total", 1, map[string]string{"sourceIP": r.RemoteAddr, "nfId": params.svcAtrrs[0].NfName, "traceSessionId": params.traceSessionID, "action": "stop", "responseCode": strconv.Itoa(http.StatusInternalServerError)})
		if err != nil {
			logging.LogConf.Error("Error while adding values in  counter  for Total number of SLT session requests received by TPaaS.", err)
		}
		statusCode = http.StatusInternalServerError
		return
	}
	var nfResponses []NFResponse
	// Update the regularkey (/operations/request/<NfName>/<NfsName>/<SvcVersion>/<InvocationMode>/<transactionID>) only if the session is running.
	// Updating the regular key of an already suspended session causes NFs to error out with 404 session not found code.
	// For suspended sessions the key is already updated and we don't want to do it again.
	if traceSessionsRecord.State == running {
		ops := []clientv3.Op{}
		var leaseIds []clientv3.LeaseID
		for _, attr := range params.svcAtrrs {
			//create lease
			t = time.Now()
			lease, err := etcd.Client.Grant(wtctx, int64(attr.TTL))
			logging.LogConf.Debug("[SLT-Stop] Time taken to grant lease for trace session ID", params.traceSessionID, "Is", time.Since(t).Seconds())
			if err != nil {
				logging.LogConf.Error("[SLT-Stop] Error while creating lease", err)

				//Add ops response key using cim api
				err = report(&problemDetails{"Put operations request error", http.StatusInternalServerError, "", nil}, http.StatusInternalServerError, r.Header.Get("X-Operation-TransactionId"))
				if err != nil {
					logging.LogConf.Error(err.Error())
				}
				rvFailedIds := revokeLeases(wtctx, leaseIds)
				if len(rvFailedIds) > 0 {
					logging.LogConf.Debug("[SLT-Stop] Lease revoke failed for Id's", rvFailedIds)
				}
				// counter increment for tpaas_slt_sessions_error_total
				err = promutil.CounterAdd("tpaas_slt_sessions_error_total", 1, map[string]string{"sourceIP": r.RemoteAddr, "nfId": params.svcAtrrs[0].NfName, "traceSessionId": params.traceSessionID, "action": "stop", "responseCode": strconv.Itoa(http.StatusInternalServerError)})
				if err != nil {
					logging.LogConf.Error("Error while adding values in  counter  for Total number of SLT session requests received by TPaaS.", err)
				}
				statusCode = http.StatusInternalServerError
				return
			}
			logging.LogConf.Debug("clientv3.WithLease(resp.ID)", lease.ID)
			leaseIds = append(leaseIds, lease.ID)
			// get ClientV3 Ops
			attr.Method = http.MethodDelete
			op, err := getClientV3Ops(&attr, lease.ID, params.transactionID, params.traceSessionID, nil)
			if err != nil {
				logging.LogConf.Debug("[SLT-Stop] Put operations request error:", err)

				//Add ops response key using cim api
				err = report(&problemDetails{"Put operations request error", http.StatusInternalServerError, "", nil}, http.StatusInternalServerError, r.Header.Get("X-Operation-TransactionId"))
				if err != nil {
					logging.LogConf.Error(err.Error())
				}
				rvFailedIds := revokeLeases(wtctx, leaseIds)
				if len(rvFailedIds) > 0 {
					logging.LogConf.Debug("[SLT-Stop] Lease revoke failed for Id's", rvFailedIds)
				}
				// counter increment for tpaas_slt_sessions_error_total
				err = promutil.CounterAdd("tpaas_slt_sessions_error_total", 1, map[string]string{"sourceIP": r.RemoteAddr, "nfId": params.svcAtrrs[0].NfName, "traceSessionId": params.traceSessionID, "action": "stop", "responseCode": strconv.Itoa(http.StatusInternalServerError)})
				if err != nil {
					logging.LogConf.Error("Error while adding values in  counter  for Total number of SLT session requests received by TPaaS.", err)
				}
				statusCode = http.StatusInternalServerError
				return
			}
			ops = append(ops, op)
		}

		//Update a resp key in map, etcd watcher can ignore if unwanted keys occured.
		nfResponseKeys.Store(params.svcAtrrs[0].NfName, true)

		t = time.Now()
		_, err = etcd.Client.Txn(wtctx).Then(ops...).Commit()
		logging.LogConf.Debug("[SLT-Stop] Time taken to commit transaction for trace session ID", params.traceSessionID, "Is", time.Since(t).Seconds())
		if err != nil {
			logging.LogConf.Debug("[SLT-Stop] Commit Operations Request error", err)

			//Add ops response key using cim api
			err = report(&problemDetails{"Commit Operations Request error", http.StatusInternalServerError, "", nil}, http.StatusInternalServerError, r.Header.Get("X-Operation-TransactionId"))
			if err != nil {
				logging.LogConf.Error(err.Error())
			}
			// counter increment for tpaas_slt_sessions_error_total
			err = promutil.CounterAdd("tpaas_slt_sessions_error_total", 1, map[string]string{"sourceIP": r.RemoteAddr, "nfId": params.svcAtrrs[0].NfName, "traceSessionId": params.traceSessionID, "action": "stop", "responseCode": strconv.Itoa(http.StatusInternalServerError)})
			if err != nil {
				logging.LogConf.Error("Error while adding values in  counter  for Total number of SLT session requests received by TPaaS.", err)
			}
			statusCode = http.StatusInternalServerError
			return
		}
		logging.LogConf.Info("[SLT-Stop] Operations Request Commit Success")

		t = time.Now()
		nfResponses, err = getNFResponses(params.svcAtrrs)
		logging.LogConf.Debug("[SLT-Stop] Time taken to get nf responses for trace session ID", params.traceSessionID, "Is", time.Since(t).Seconds())
		if err != nil {
			logging.LogConf.Debug("[SLT-Stop] Error getting response from NF", err)

			//Add ops response key using cim api
			err = report(&problemDetails{fmt.Sprint("Error getting response from NF. ", err.Error()), http.StatusInternalServerError, "", nil}, http.StatusInternalServerError, r.Header.Get("X-Operation-TransactionId"))
			if err != nil {
				logging.LogConf.Error(err.Error())
			}
			// counter increment for tpaas_slt_sessions_error_total
			err = promutil.CounterAdd("tpaas_slt_sessions_error_total", 1, map[string]string{"sourceIP": r.RemoteAddr, "nfId": params.svcAtrrs[0].NfName, "traceSessionId": params.traceSessionID, "action": "stop", "responseCode": strconv.Itoa(http.StatusInternalServerError)})
			if err != nil {
				logging.LogConf.Error("Error while adding values in counter for Total number of SLT session requests received by TPaaS.", err)
			}
			statusCode = http.StatusInternalServerError
			return
		}
		logging.LogConf.Info("[SLT-Stop] Received and collated response from NF")
	} // end-MTCILMWP-3980: fix stop not working after pause

	// Check if any nf responded with success status code.
	// Only in success scenario or if the session is already suspended (stop after pause scenario) we should delete the tpaas session key in etcd.
	if isNFResponseSuccess(nfResponses) || traceSessionsRecord.State == suspended {
		ctxDel := context.Background()
		wtCtxDel, cancelDelCtx := context.WithTimeout(ctxDel, time.Second*time.Duration(EtcdTimeOutSec))
		defer cancelDelCtx()

		// delete the sessions data
		t = time.Now()
		_, err = etcd.Client.Delete(wtCtxDel, sessionsKey)
		logging.LogConf.Debug("[SLT-Stop] Time taken to delete trace session ID", params.traceSessionID, "Is", time.Since(t).Seconds())
		if err != nil {
			logging.LogConf.Debug("[SLT-Stop] Delete sessions data error:", err)

			//Add ops response key using cim api
			err = report(&problemDetails{"Delete sessions data error", http.StatusInternalServerError, "", nil}, http.StatusInternalServerError, r.Header.Get("X-Operation-TransactionId"))
			if err != nil {
				logging.LogConf.Error(err.Error())
			}
			// counter increment for tpaas_slt_sessions_error_total
			err = promutil.CounterAdd("tpaas_slt_sessions_error_total", 1, map[string]string{"sourceIP": r.RemoteAddr, "nfId": params.svcAtrrs[0].NfName, "traceSessionId": params.traceSessionID, "action": "stop", "responseCode": strconv.Itoa(http.StatusInternalServerError)})
			if err != nil {
				logging.LogConf.Error("Error while adding values in  counter  for Total number of SLT session requests received by TPaaS.", err)
			}
			statusCode = http.StatusInternalServerError
			return
		}
	}

	// We will get NF responses only for running sessions which are being stopped. For already Paused sessions we will not get NF response for Stopping those sessions.
	if traceSessionsRecord.State == running {
		// Add the collated response from nf to TPaaS response.
		err = report(nfResponses, http.StatusOK, r.Header.Get("X-Operation-TransactionId"))
		if err != nil {
			logging.LogConf.Error(err.Error())
		}
		return
	}

	// This line indicates that a stop request for paused session was received and that the paused session was stopped.
	// In this case TOPO should consider a 200 Ok status with empty ("") nf response in the body as an indicator that the session was already stopped.
	logging.LogConf.Info("[SLT-Stop] Stopped a paused session")
	// Call CIMs report api to provide feedback to it so that it can add operations/reponse key for tpaas in etcd.
	err = report("", http.StatusOK, r.Header.Get("X-Operation-TransactionId"))
	if err != nil {
		logging.LogConf.Error(err.Error())
	}
}

// suspend handles suspend
func suspend(r *http.Request) {
	nfId := "NA"
	statusCode := http.StatusOK

	defer func() {
		// counter increment for tpaas_slt_requests_total
		err := promutil.CounterAdd("tpaas_slt_requests_total", 1, map[string]string{"mNfId": nfId, "statusCode": strconv.Itoa(statusCode), "action": "suspend"})
		if err != nil {
			logging.LogConf.Error("Error while adding values in  counter  for Total number of operational SLT requests recieved by TPaaS.", err)
		}
	}()

	if strings.Contains(EtcdResEnabled, "true") {
		logging.LogConf.Info("[SLT-Suspend] - Going to sleep for seconds -", dst.seconds)
		time.Sleep(time.Duration(dst.seconds) * time.Second)
		logging.LogConf.Info("[SLT-Suspend] - Sleep period over")
	}

	// get the attrs
	params, err := getParams(r)
	if err != nil {
		logging.LogConf.Error("[SLT-Suspend] Params error", err.Error())
		//Add ops response key using cim api
		err = report(&problemDetails{fmt.Sprintf("Params error: %s", err), http.StatusBadRequest, "", nil}, http.StatusBadRequest, r.Header.Get("X-Operation-TransactionId"))
		if err != nil {
			logging.LogConf.Error(err.Error())
		}
		// counter increment for tpaas_slt_sessions_error_total
		err = promutil.CounterAdd("tpaas_slt_sessions_error_total", 1, map[string]string{"sourceIP": r.RemoteAddr, "nfId": "NA", "traceSessionId": "NA", "action": "suspend", "responseCode": strconv.Itoa(http.StatusBadRequest)})
		if err != nil {
			logging.LogConf.Error("Error while adding values in  counter  for Total number of SLT session requests received by TPaaS.", err)
		}
		statusCode = http.StatusBadRequest
		return
	}

	// check if svc attr present or not
	if len(params.svcAtrrs) == 0 {
		logging.LogConf.Error("[SLT-Suspend] svc attr Not found")
		//Add ops response key using cim api
		err = report(&problemDetails{"svc attr Not found", http.StatusBadRequest, "", nil}, http.StatusBadRequest, r.Header.Get("X-Operation-TransactionId"))
		if err != nil {
			logging.LogConf.Error(err.Error())
		}
		// counter increment for tpaas_slt_sessions_error_total
		err = promutil.CounterAdd("tpaas_slt_sessions_error_total", 1, map[string]string{"sourceIP": r.RemoteAddr, "nfId": "NA", "traceSessionId": "NA", "action": "suspend", "responseCode": strconv.Itoa(http.StatusBadRequest)})
		if err != nil {
			logging.LogConf.Error("Error while adding values in  counter  for Total number of SLT session requests received by TPaaS.", err)
		}
		statusCode = http.StatusBadRequest
		return
	}

	nfId = params.svcAtrrs[0].NfName
	// counter increment for tpaas_slt_sessions_total
	err = promutil.CounterAdd("tpaas_slt_sessions_total", 1, map[string]string{"sourceIP": r.RemoteAddr, "nfId": params.svcAtrrs[0].NfName, "traceSessionId": params.traceSessionID, "action": "suspend"})
	if err != nil {
		logging.LogConf.Error("Error while adding values in  counter  for Total number of SLT session requests received by TPaaS.", err)
	}

	// get sessions data
	ctx := context.Background()
	wtctx, cancel := context.WithTimeout(ctx, time.Second*time.Duration(EtcdTimeOutSec))
	defer cancel()

	t := time.Now()
	sessionsKey, result, err := getSessionsData(wtctx, params.svcAtrrs[0].NfName, params.traceSessionID)
	logging.LogConf.Debug("[SLT-Suspend] Get is done", err, result, "time taken to get session data for trace session ID", params.traceSessionID, "Is", time.Since(t).Seconds())
	if err != nil {
		logging.LogConf.Debug("[SLT-Suspend] Get sessions data error:", err)

		//Add ops response key using cim api
		err = report(&problemDetails{"Get sessions data error", http.StatusInternalServerError, "", nil}, http.StatusInternalServerError, r.Header.Get("X-Operation-TransactionId"))
		if err != nil {
			logging.LogConf.Error(err.Error())
		}
		// counter increment for tpaas_slt_sessions_error_total
		err = promutil.CounterAdd("tpaas_slt_sessions_error_total", 1, map[string]string{"sourceIP": r.RemoteAddr, "nfId": params.svcAtrrs[0].NfName, "traceSessionId": params.traceSessionID, "action": "suspend", "responseCode": strconv.Itoa(http.StatusInternalServerError)})
		if err != nil {
			logging.LogConf.Error("Error while adding values in  counter  for Total number of SLT session requests received by TPaaS.", err)
		}
		statusCode = http.StatusInternalServerError
		return
	}

	if len(result.Kvs) == 0 || result.Kvs[0] == nil {
		logging.LogConf.Debug("[SLT-Suspend] Get sessions data error: result.Kvs", result.Kvs)

		//Add ops response key using cim api
		err = report(&problemDetails{"Session not found", http.StatusBadRequest, "", nil}, http.StatusBadRequest, r.Header.Get("X-Operation-TransactionId"))
		if err != nil {
			logging.LogConf.Error(err.Error())
		}
		// counter increment for tpaas_slt_sessions_error_total
		err = promutil.CounterAdd("tpaas_slt_sessions_error_total", 1, map[string]string{"sourceIP": r.RemoteAddr, "nfId": params.svcAtrrs[0].NfName, "traceSessionId": params.traceSessionID, "action": "suspend", "responseCode": strconv.Itoa(http.StatusBadRequest)})
		if err != nil {
			logging.LogConf.Error("Error while adding values in  counter  for Total number of SLT session requests received by TPaaS.", err)
		}
		statusCode = http.StatusBadRequest
		return
	}

	traceSessionsRecord := traceSession{}
	err = json.Unmarshal(result.Kvs[0].Value, &traceSessionsRecord)
	if err != nil {
		logging.LogConf.Debug("[SLT-Suspend] Session data error", err)
		//Add ops response key using cim api
		err = report(&problemDetails{"Session data error", http.StatusInternalServerError, "", nil}, http.StatusInternalServerError, r.Header.Get("X-Operation-TransactionId"))
		if err != nil {
			logging.LogConf.Error(err.Error())
		}
		// counter increment for tpaas_slt_sessions_error_total
		err = promutil.CounterAdd("tpaas_slt_sessions_error_total", 1, map[string]string{"sourceIP": r.RemoteAddr, "nfId": params.svcAtrrs[0].NfName, "traceSessionId": params.traceSessionID, "action": "suspend", "responseCode": strconv.Itoa(http.StatusInternalServerError)})
		if err != nil {
			logging.LogConf.Error("Error while adding values in  counter  for Total number of SLT session requests received by TPaaS.", err)
		}
		statusCode = http.StatusInternalServerError
		return
	}

	// check if session already suspended
	if traceSessionsRecord.State == suspended {
		//Add ops response key using cim api
		err = report(&problemDetails{"Session already suspended", http.StatusBadRequest, "", nil}, http.StatusBadRequest, r.Header.Get("X-Operation-TransactionId"))
		if err != nil {
			logging.LogConf.Error(err.Error())
		}
		// counter increment for tpaas_slt_sessions_error_total
		err = promutil.CounterAdd("tpaas_slt_sessions_error_total", 1, map[string]string{"sourceIP": r.RemoteAddr, "nfId": params.svcAtrrs[0].NfName, "traceSessionId": params.traceSessionID, "action": "suspend", "responseCode": strconv.Itoa(http.StatusBadRequest)})
		if err != nil {
			logging.LogConf.Error("Error while adding values in  counter  for Total number of SLT session requests received by TPaaS.", err)
		}
		statusCode = http.StatusBadRequest
		return
	}
	ops := []clientv3.Op{}
	var leaseIds []clientv3.LeaseID
	for _, attr := range params.svcAtrrs {
		//create lease
		t = time.Now()
		lease, err := etcd.Client.Grant(wtctx, int64(attr.TTL))
		logging.LogConf.Debug("[SLT-Suspend] Time taken to grant lease for trace session ID", params.traceSessionID, "Is", time.Since(t).Seconds())
		if err != nil {
			logging.LogConf.Error("[SLT-Suspend] Error while creating lease", err)
			//Add ops response key using cim api
			err = report(&problemDetails{"Put operations request error", http.StatusInternalServerError, "", nil}, http.StatusInternalServerError, r.Header.Get("X-Operation-TransactionId"))
			if err != nil {
				logging.LogConf.Error(err.Error())
			}
			rvFailedIds := revokeLeases(wtctx, leaseIds)
			if len(rvFailedIds) > 0 {
				logging.LogConf.Debug("[SLT-Suspend] Lease revoke failed for Id's", rvFailedIds)
			}
			// counter increment for tpaas_slt_sessions_error_total
			err = promutil.CounterAdd("tpaas_slt_sessions_error_total", 1, map[string]string{"sourceIP": r.RemoteAddr, "nfId": params.svcAtrrs[0].NfName, "traceSessionId": params.traceSessionID, "action": "suspend", "responseCode": strconv.Itoa(http.StatusInternalServerError)})
			if err != nil {
				logging.LogConf.Error("Error while adding values in  counter  for Total number of SLT session requests received by TPaaS.", err)
			}
			statusCode = http.StatusInternalServerError
			return
		}
		logging.LogConf.Debug("[SLT-Suspend] clientv3.WithLease(resp.ID)", lease.ID)
		leaseIds = append(leaseIds, lease.ID)
		// get ClientV3 Ops
		attr.Method = http.MethodDelete
		op, err := getClientV3Ops(&attr, lease.ID, params.transactionID, params.traceSessionID, nil)
		if err != nil {
			logging.LogConf.Debug("[SLT-Suspend] Put operations request error", err)

			//Add ops response key using cim api
			err = report(&problemDetails{"Put operations request error", http.StatusInternalServerError, "", nil}, http.StatusInternalServerError, r.Header.Get("X-Operation-TransactionId"))
			if err != nil {
				logging.LogConf.Error(err.Error())
			}
			rvFailedIds := revokeLeases(wtctx, leaseIds)
			if len(rvFailedIds) > 0 {
				logging.LogConf.Debug("[SLT-Suspend] Lease revoke failed for Id's", rvFailedIds)
			}
			// counter increment for tpaas_slt_sessions_error_total
			err = promutil.CounterAdd("tpaas_slt_sessions_error_total", 1, map[string]string{"sourceIP": r.RemoteAddr, "nfId": params.svcAtrrs[0].NfName, "traceSessionId": params.traceSessionID, "action": "suspend", "responseCode": strconv.Itoa(http.StatusInternalServerError)})
			if err != nil {
				logging.LogConf.Error("Error while adding values in  counter  for Total number of SLT session requests received by TPaaS.", err)
			}
			statusCode = http.StatusInternalServerError
			return
		}
		ops = append(ops, op)
	}

	//Update a resp key in map, etcd watcher can ignore if unwanted keys occured.
	nfResponseKeys.Store(params.svcAtrrs[0].NfName, true)

	t = time.Now()
	_, err = etcd.Client.Txn(wtctx).Then(ops...).Commit()
	logging.LogConf.Debug("[SLT-Suspend] Time taken to commit transaction for trace session ID", params.traceSessionID, "Is", time.Since(t).Seconds())
	if err != nil {
		logging.LogConf.Debug("[SLT-Suspend] Commit Operations Request error", err)

		//Add ops response key using cim api
		err = report(&problemDetails{"Commit Operations Request error", http.StatusInternalServerError, "", nil}, http.StatusInternalServerError, r.Header.Get("X-Operation-TransactionId"))
		if err != nil {
			logging.LogConf.Error(err.Error())
		}
		// counter increment for tpaas_slt_sessions_error_total
		err = promutil.CounterAdd("tpaas_slt_sessions_error_total", 1, map[string]string{"sourceIP": r.RemoteAddr, "nfId": params.svcAtrrs[0].NfName, "traceSessionId": params.traceSessionID, "action": "suspend", "responseCode": strconv.Itoa(http.StatusInternalServerError)})
		if err != nil {
			logging.LogConf.Error("Error while adding values in  counter  for Total number of SLT session requests received by TPaaS.", err)
		}
		statusCode = http.StatusInternalServerError
		return
	}
	logging.LogConf.Info("[SLT-Suspend] Operations Request Commit Success")

	t = time.Now()
	nfResponses, err := getNFResponses(params.svcAtrrs)
	logging.LogConf.Debug("[SLT-Suspend] Time taken to get nf response for trace session ID", params.traceSessionID, "Is", time.Since(t).Seconds())
	if err != nil {
		logging.LogConf.Debug("[SLT-Suspend] Error getting response from NF", err)
		//Add ops response key using cim api
		err = report(&problemDetails{fmt.Sprint("Error getting response from NF. ", err.Error()), http.StatusInternalServerError, "", nil}, http.StatusInternalServerError, r.Header.Get("X-Operation-TransactionId"))
		if err != nil {
			logging.LogConf.Error(err.Error())
		}
		// counter increment for tpaas_slt_sessions_error_total
		err = promutil.CounterAdd("tpaas_slt_sessions_error_total", 1, map[string]string{"sourceIP": r.RemoteAddr, "nfId": params.svcAtrrs[0].NfName, "traceSessionId": params.traceSessionID, "action": "suspend", "responseCode": strconv.Itoa(http.StatusInternalServerError)})
		if err != nil {
			logging.LogConf.Error("Error while adding values in counter for Total number of SLT session requests received by TPaaS.", err)
		}
		statusCode = http.StatusInternalServerError
		return
	}
	logging.LogConf.Info("[SLT-Suspend] Received and collated response from NF")

	// Check if any nf responded with success status code.
	// Only in success scenario we should touch the tpaas session key in etcd.
	if isNFResponseSuccess(nfResponses) {
		traceSessionsRecord.State = suspended
		traceSessionsRecord.Method = http.MethodDelete

		ctxPut := context.Background()
		wtCtxPut, cancelPutCtx := context.WithTimeout(ctxPut, time.Second*time.Duration(EtcdTimeOutSec))
		defer cancelPutCtx()

		t = time.Now()
		_, err = putSessionsData(wtCtxPut, sessionsKey, &traceSessionsRecord)
		logging.LogConf.Debug("[SLT-Suspend] Time taken to put session data for trace session ID", params.traceSessionID, "Is", time.Since(t).Seconds())
		if err != nil {
			logging.LogConf.Debug("[SLT-Suspend] Put sessions data error:", err)

			//Add ops response key using cim api
			err = report(&problemDetails{"Put sessions data error", http.StatusInternalServerError, "", nil}, http.StatusInternalServerError, r.Header.Get("X-Operation-TransactionId"))
			if err != nil {
				logging.LogConf.Error(err.Error())
			}
			// counter increment for tpaas_slt_sessions_error_total
			err = promutil.CounterAdd("tpaas_slt_sessions_error_total", 1, map[string]string{"sourceIP": r.RemoteAddr, "nfId": params.svcAtrrs[0].NfName, "traceSessionId": params.traceSessionID, "action": "suspend", "responseCode": strconv.Itoa(http.StatusInternalServerError)})
			if err != nil {
				logging.LogConf.Error("Error while adding values in  counter  for Total number of SLT session requests received by TPaaS.", err)
			}
			statusCode = http.StatusInternalServerError
			return
		}
	}

	// Add the collated response from nf to TPaaS response.
	// Call CIMs report api to provide feedback to it so that it can add operations/reponse key for tpaas in etcd.
	err = report(nfResponses, http.StatusOK, r.Header.Get("X-Operation-TransactionId"))
	if err != nil {
		logging.LogConf.Error(err.Error())
	}
}

// resume handles resume
func resume(r *http.Request) {
	nfId := "NA"
	statusCode := http.StatusOK

	defer func() {
		// counter increment for tpaas_slt_requests_total
		err := promutil.CounterAdd("tpaas_slt_requests_total", 1, map[string]string{"mNfId": nfId, "statusCode": strconv.Itoa(statusCode), "action": "resume"})
		if err != nil {
			logging.LogConf.Error("Error while adding values in  counter  for Total number of operational SLT requests recieved by TPaaS.", err)
		}
	}()

	if strings.Contains(EtcdResEnabled, "true") {
		logging.LogConf.Info("[SLT-Resume] - Going to sleep for seconds -", dst.seconds)
		time.Sleep(time.Duration(dst.seconds) * time.Second)
		logging.LogConf.Info("[SLT-Resume] - Sleep period over")
	}

	// get the attrs from the request headers
	params, err := getParams(r)
	if err != nil {
		logging.LogConf.Error("[SLT-Resume] Params error", err.Error())
		//Add ops response key using cim api
		err = report(&problemDetails{fmt.Sprintf("Params error: %s", err), http.StatusBadRequest, "", nil}, http.StatusBadRequest, r.Header.Get("X-Operation-TransactionId"))
		if err != nil {
			logging.LogConf.Error(err.Error())
		}
		// counter increment for tpaas_slt_sessions_error_total
		err = promutil.CounterAdd("tpaas_slt_sessions_error_total", 1, map[string]string{"sourceIP": r.RemoteAddr, "nfId": "NA", "traceSessionId": "NA", "action": "resume", "responseCode": strconv.Itoa(http.StatusBadRequest)})
		if err != nil {
			logging.LogConf.Error("Error while adding values in  counter  for Total number of SLT session requests received by TPaaS.", err)
		}
		statusCode = http.StatusBadRequest
		return
	}

	// check if svc attr present or not
	if len(params.svcAtrrs) == 0 {
		logging.LogConf.Error("[SLT-Resume] svc attr Not found")
		//Add ops response key using cim api
		err = report(&problemDetails{"svc attr Not found", http.StatusBadRequest, "", nil}, http.StatusBadRequest, r.Header.Get("X-Operation-TransactionId"))
		if err != nil {
			logging.LogConf.Error(err.Error())
		}
		// counter increment for tpaas_slt_sessions_error_total
		err = promutil.CounterAdd("tpaas_slt_sessions_error_total", 1, map[string]string{"sourceIP": r.RemoteAddr, "nfId": "NA", "traceSessionId": "NA", "action": "resume", "responseCode": strconv.Itoa(http.StatusBadRequest)})
		if err != nil {
			logging.LogConf.Error("Error while adding values in  counter  for Total number of SLT session requests received by TPaaS.", err)
		}
		statusCode = http.StatusBadRequest
		return
	}

	nfId = params.svcAtrrs[0].NfName
	// counter increment for tpaas_slt_sessions_total
	err = promutil.CounterAdd("tpaas_slt_sessions_total", 1, map[string]string{"sourceIP": r.RemoteAddr, "nfId": params.svcAtrrs[0].NfName, "traceSessionId": params.traceSessionID, "action": "resume"})
	if err != nil {
		logging.LogConf.Error("Error while adding values in  counter  for Total number of SLT session requests received by TPaaS.", err)
	}

	// get sessions data
	ctx := context.Background()
	wtctx, cancel := context.WithTimeout(ctx, time.Second*time.Duration(EtcdTimeOutSec))
	defer cancel()

	t := time.Now()
	sessionsKey, result, err := getSessionsData(wtctx, params.svcAtrrs[0].NfName, params.traceSessionID)
	logging.LogConf.Debug("[SLT-Resume] Get is done", err, result, "Time taken to get session data for trace session ID", params.traceSessionID, "Is", time.Since(t).Seconds())
	if err != nil {
		logging.LogConf.Debug("[SLT-Resume] Get sessions data error:", err)
		//Add ops response key using cim api
		err = report(&problemDetails{"Get sessions data error", http.StatusInternalServerError, "", nil}, http.StatusInternalServerError, r.Header.Get("X-Operation-TransactionId"))
		if err != nil {
			logging.LogConf.Error(err.Error())
		}

		// counter increment for tpaas_slt_sessions_error_total
		err = promutil.CounterAdd("tpaas_slt_sessions_error_total", 1, map[string]string{"sourceIP": r.RemoteAddr, "nfId": params.svcAtrrs[0].NfName, "traceSessionId": params.traceSessionID, "action": "resume", "responseCode": strconv.Itoa(http.StatusInternalServerError)})
		if err != nil {
			logging.LogConf.Error("Error while adding values in  counter  for Total number of SLT session requests received by TPaaS.", err)
		}
		statusCode = http.StatusInternalServerError
		return
	}

	// check if session not found
	if result == nil || len(result.Kvs) == 0 {
		//Add ops response key using cim api
		err = report(&problemDetails{"Session not found", http.StatusBadRequest, "", nil}, http.StatusBadRequest, r.Header.Get("X-Operation-TransactionId"))
		if err != nil {
			logging.LogConf.Error(err.Error())
		}
		// counter increment for tpaas_slt_sessions_error_total
		err = promutil.CounterAdd("tpaas_slt_sessions_error_total", 1, map[string]string{"sourceIP": r.RemoteAddr, "nfId": params.svcAtrrs[0].NfName, "traceSessionId": params.traceSessionID, "action": "resume", "responseCode": strconv.Itoa(http.StatusBadRequest)})
		if err != nil {
			logging.LogConf.Error("Error while adding values in  counter  for Total number of SLT session requests received by TPaaS.", err)
		}
		statusCode = http.StatusBadRequest
		return
	}
	// check if the session already running
	traceSessionsRecord := traceSession{}
	err = json.Unmarshal(result.Kvs[0].Value, &traceSessionsRecord)
	if err != nil {
		logging.LogConf.Debug("[SLT-Resume] Session data error", err)
		//Add ops response key using cim api
		err = report(&problemDetails{"Session data error", http.StatusInternalServerError, "", nil}, http.StatusInternalServerError, r.Header.Get("X-Operation-TransactionId"))
		if err != nil {
			logging.LogConf.Error(err.Error())
		}
		// counter increment for tpaas_slt_sessions_error_total
		err = promutil.CounterAdd("tpaas_slt_sessions_error_total", 1, map[string]string{"sourceIP": r.RemoteAddr, "nfId": params.svcAtrrs[0].NfName, "traceSessionId": params.traceSessionID, "action": "resume", "responseCode": strconv.Itoa(http.StatusInternalServerError)})
		if err != nil {
			logging.LogConf.Error("Error while adding values in  counter  for Total number of SLT session requests received by TPaaS.", err)
		}
		statusCode = http.StatusInternalServerError
		return
	}
	if traceSessionsRecord.State == running {
		//Add ops response key using cim api
		err = report(&problemDetails{"Session already running", http.StatusBadRequest, "", nil}, http.StatusBadRequest, r.Header.Get("X-Operation-TransactionId"))
		if err != nil {
			logging.LogConf.Error(err.Error())
		}
		// counter increment for tpaas_slt_sessions_error_total
		err = promutil.CounterAdd("tpaas_slt_sessions_error_total", 1, map[string]string{"sourceIP": r.RemoteAddr, "nfId": params.svcAtrrs[0].NfName, "traceSessionId": params.traceSessionID, "action": "resume", "responseCode": strconv.Itoa(http.StatusBadRequest)})
		if err != nil {
			logging.LogConf.Error("Error while adding values in  counter  for Total number of SLT session requests received by TPaaS.", err)
		}
		statusCode = http.StatusBadRequest
		return
	}

	ops := []clientv3.Op{}
	var leaseIds []clientv3.LeaseID
	sessionData, err := json.Marshal(traceSessionsRecord.Session)
	if err != nil {
		logging.LogConf.Debug("[SLT-Resume] Session data marshall error", err)
		//Add ops response key using cim api
		err = report(&problemDetails{"Put operations request error", http.StatusInternalServerError, "", nil}, http.StatusInternalServerError, r.Header.Get("X-Operation-TransactionId"))
		if err != nil {
			logging.LogConf.Error(err.Error())
		}
		// counter increment for tpaas_slt_sessions_error_total
		err = promutil.CounterAdd("tpaas_slt_sessions_error_total", 1, map[string]string{"sourceIP": r.RemoteAddr, "nfId": params.svcAtrrs[0].NfName, "traceSessionId": params.traceSessionID, "action": "resume", "responseCode": strconv.Itoa(http.StatusInternalServerError)})
		if err != nil {
			logging.LogConf.Error("Error while adding values in  counter  for Total number of SLT session requests received by TPaaS.", err)
		}
		statusCode = http.StatusInternalServerError
		return
	}
	for _, attr := range params.svcAtrrs {
		// put the operation request
		//create lease
		t = time.Now()
		lease, err := etcd.Client.Grant(wtctx, int64(attr.TTL))
		logging.LogConf.Debug("[SLT-Resume] Time taken to grant lease for trace session ID", params.traceSessionID, "Is", time.Since(t).Seconds())
		if err != nil {
			logging.LogConf.Error("[SLT-Resume] Error while creating lease", err)

			//Add ops response key using cim api
			err = report(&problemDetails{"Put operations request error", http.StatusInternalServerError, "", nil}, http.StatusInternalServerError, r.Header.Get("X-Operation-TransactionId"))
			if err != nil {
				logging.LogConf.Error(err.Error())
			}
			rvFailedIds := revokeLeases(wtctx, leaseIds)
			if len(rvFailedIds) > 0 {
				logging.LogConf.Debug("[SLT-Resume] Lease revoke failed for Id's", rvFailedIds)
			}
			// counter increment for tpaas_slt_sessions_error_total
			err = promutil.CounterAdd("tpaas_slt_sessions_error_total", 1, map[string]string{"sourceIP": r.RemoteAddr, "nfId": params.svcAtrrs[0].NfName, "traceSessionId": params.traceSessionID, "action": "resume", "responseCode": strconv.Itoa(http.StatusInternalServerError)})
			if err != nil {
				logging.LogConf.Error("Error while adding values in  counter  for Total number of SLT session requests received by TPaaS.", err)
			}
			statusCode = http.StatusInternalServerError
			return
		}
		logging.LogConf.Debug("[SLT-Resume] clientv3.WithLease(resp.ID)", lease.ID)
		leaseIds = append(leaseIds, lease.ID)
		// get ClientV3 Ops
		attr.Method = http.MethodPost
		op, err := getClientV3Ops(&attr, lease.ID, params.transactionID, params.traceSessionID, sessionData)
		if err != nil {
			logging.LogConf.Debug("[SLT-Resume] Put operations request error", err)
			//Add ops response key using cim api
			err = report(&problemDetails{"Put operations request error", http.StatusInternalServerError, "", nil}, http.StatusInternalServerError, r.Header.Get("X-Operation-TransactionId"))
			if err != nil {
				logging.LogConf.Error(err.Error())
			}
			rvFailedIds := revokeLeases(wtctx, leaseIds)
			if len(rvFailedIds) > 0 {
				logging.LogConf.Debug("[SLT-Resume] Lease revoke failed for Id's", rvFailedIds)
			}
			// counter increment for tpaas_slt_sessions_error_total
			err = promutil.CounterAdd("tpaas_slt_sessions_error_total", 1, map[string]string{"sourceIP": r.RemoteAddr, "nfId": params.svcAtrrs[0].NfName, "traceSessionId": params.traceSessionID, "action": "resume", "responseCode": strconv.Itoa(http.StatusInternalServerError)})
			if err != nil {
				logging.LogConf.Error("Error while adding values in  counter  for Total number of SLT session requests received by TPaaS.", err)
			}
			statusCode = http.StatusInternalServerError
			return
		}
		ops = append(ops, op)
	}

	//Update a resp key in map, etcd watcher can ignore if unwanted keys occured.
	nfResponseKeys.Store(params.svcAtrrs[0].NfName, true)

	t = time.Now()
	_, err = etcd.Client.Txn(wtctx).Then(ops...).Commit()
	logging.LogConf.Debug("[SLT-Resume] Time taken to commit transaction for trace session ID", params.traceSessionID, "Is", time.Since(t).Seconds())
	if err != nil {
		logging.LogConf.Debug("[SLT-Resume] Commit Operations Request error", err)
		//Add ops response key using cim api
		err = report(&problemDetails{"Commit Operations Request error", http.StatusInternalServerError, "", nil}, http.StatusInternalServerError, r.Header.Get("X-Operation-TransactionId"))
		if err != nil {
			logging.LogConf.Error(err.Error())
		}

		// counter increment for tpaas_slt_sessions_error_total
		err = promutil.CounterAdd("tpaas_slt_sessions_error_total", 1, map[string]string{"sourceIP": r.RemoteAddr, "nfId": params.svcAtrrs[0].NfName, "traceSessionId": params.traceSessionID, "action": "resume", "responseCode": strconv.Itoa(http.StatusInternalServerError)})
		if err != nil {
			logging.LogConf.Error("Error while adding values in  counter  for Total number of SLT session requests received by TPaaS.", err)
		}
		statusCode = http.StatusInternalServerError
		return
	}
	logging.LogConf.Info("[SLT-Resume] Operations Request Commit Success")

	t = time.Now()
	nfResponses, err := getNFResponses(params.svcAtrrs)
	logging.LogConf.Debug("[SLT-Resume] Time taken to get nf response for trace session ID", params.traceSessionID, "Is", time.Since(t).Seconds())
	if err != nil {
		logging.LogConf.Debug("[SLT-Resume] Error getting response from NF", err)
		//Add ops response key using cim api
		err = report(&problemDetails{fmt.Sprint("Error getting response from NF. ", err.Error()), http.StatusInternalServerError, "", nil}, http.StatusInternalServerError, r.Header.Get("X-Operation-TransactionId"))
		if err != nil {
			logging.LogConf.Error(err.Error())
		}

		// counter increment for tpaas_slt_sessions_error_total
		err = promutil.CounterAdd("tpaas_slt_sessions_error_total", 1, map[string]string{"sourceIP": r.RemoteAddr, "nfId": params.svcAtrrs[0].NfName, "traceSessionId": params.traceSessionID, "action": "resume", "responseCode": strconv.Itoa(http.StatusInternalServerError)})
		if err != nil {
			logging.LogConf.Error("Error while adding values in counter for Total number of SLT session requests received by TPaaS.", err)
		}
		statusCode = http.StatusInternalServerError
		return
	}
	logging.LogConf.Info("[SLT-Resume] Received and collated response from NF")

	// Check if any nf responded with success status code.
	// Only in success scenario we should touch the tpaas session key in etcd.
	if isNFResponseSuccess(nfResponses) {
		// update the sessions data
		traceSessionsRecord.State = running
		traceSessionsRecord.Method = http.MethodPost

		ctxPut := context.Background()
		wtCtxPut, cancelPutCtx := context.WithTimeout(ctxPut, time.Second*time.Duration(EtcdTimeOutSec))
		defer cancelPutCtx()

		t = time.Now()
		_, err = putSessionsData(wtCtxPut, sessionsKey, &traceSessionsRecord)
		logging.LogConf.Debug("[SLT-Resume] Time taken to put session data for trace session ID", params.traceSessionID, "Is", time.Since(t).Seconds())
		if err != nil {
			logging.LogConf.Debug("[SLT-Resume] Put sessions data error:", err)
			//Add ops response key using cim api
			err = report(&problemDetails{"Put sessions data error", http.StatusInternalServerError, "", nil}, http.StatusInternalServerError, r.Header.Get("X-Operation-TransactionId"))
			if err != nil {
				logging.LogConf.Error(err.Error())
			}
			// counter increment for tpaas_slt_sessions_error_total
			err = promutil.CounterAdd("tpaas_slt_sessions_error_total", 1, map[string]string{"sourceIP": r.RemoteAddr, "nfId": params.svcAtrrs[0].NfName, "traceSessionId": params.traceSessionID, "action": "resume", "responseCode": strconv.Itoa(http.StatusInternalServerError)})
			if err != nil {
				logging.LogConf.Error("Error while adding values in  counter  for Total number of SLT session requests received by TPaaS.", err)
			}
			statusCode = http.StatusInternalServerError
			return
		}
	}

	// Add the collated response from nf to TPaaS response.
	// Call CIMs report api to provide feedback to it so that it can add operations/reponse key for tpaas in etcd.
	err = report(nfResponses, http.StatusOK, r.Header.Get("X-Operation-TransactionId"))
	if err != nil {
		logging.LogConf.Error(err.Error())
	}
}

// get handles get
func get(w http.ResponseWriter, r *http.Request) {
	nfId := "NA"
	statusCode := http.StatusOK

	defer func() {
		// counter increment for tpaas_slt_requests_total
		err := promutil.CounterAdd("tpaas_slt_requests_total", 1, map[string]string{"mNfId": nfId, "statusCode": strconv.Itoa(statusCode), "action": "get"})
		if err != nil {
			logging.LogConf.Error("Error while adding values in  counter  for Total number of operational SLT requests recieved by TPaaS.", err)
		}
	}()

	logging.LogConf.Debug("[SLT-Get] Recieved request")
	defer logging.LogConf.Debug("[SLT-Get] Exit")

	nfName := r.Header.Get("X-Operation-NFName")
	traceSessionID := mux.Vars(r)["traceSessionID"]
	logging.LogConf.Debug("nfName:", nfName, "traceSessionID:", traceSessionID)

	if nfName == "" {
		logging.LogConf.Debug("[SLT-Get] nfName is empty")
		writeProblemDetails(w, &problemDetails{"nfName required", http.StatusBadRequest, "", nil})
		statusCode = http.StatusBadRequest
		return
	}
	nfId = nfName
	ctx := context.Background()
	wtctx, cancel := context.WithTimeout(ctx, time.Second*time.Duration(EtcdTimeOutSec))
	defer cancel()

	t := time.Now()
	_, result, err := getSessionsData(wtctx, nfName, traceSessionID)
	logging.LogConf.Debug("[SLT-Get] Time taken to get session data for trace session ID(s)", traceSessionID, "Is", time.Since(t).Seconds())
	if err != nil {
		logging.LogConf.Debug("[SLT-Get] Get sessions data error:", err)
		writeProblemDetails(w, &problemDetails{"Get sessions data error", http.StatusInternalServerError, "", nil})
		statusCode = http.StatusInternalServerError
		return
	}

	logging.LogConf.Debug("len(result.Kvs)", len(result.Kvs))

	// Check session data
	if result == nil || len(result.Kvs) == 0 {
		if traceSessionID != "" {
			logging.LogConf.Debug("[SLT-Get] Sessions not found", "Given traceSessionId is empty", "Get All Sessions : Recieved empty response.")
			writeProblemDetails(w, &problemDetails{fmt.Sprint("Session not found for given traceSessionId " + traceSessionID), http.StatusNotFound, "", nil})
		} else {
			logging.LogConf.Debug("[SLT-Get] Sessions not found")
			writeProblemDetails(w, &problemDetails{"Sessions not found", http.StatusNotFound, "", nil})
		}
		statusCode = http.StatusNotFound
		return
	}

	traceSessions := make([]traceSession, len(result.Kvs))
	for i, kv := range result.Kvs {
		err = json.Unmarshal(kv.Value, &traceSessions[i])
		if err != nil {
			logging.LogConf.Debug("[SLT-Get] Unmarshalling traceSession error", err)
			writeProblemDetails(w, &problemDetails{"Getting traceSessionsRecord error", http.StatusInternalServerError, "", nil})
			statusCode = http.StatusInternalServerError
			return
		}
		traceSessions[i].NfsName = nil
		traceSessions[i].Headers = nil
		traceSessions[i].URL = ""
		traceSessions[i].Method = ""
		logging.LogConf.Debug("[SLT-Get] Got traceSession", traceSessions[i].TraceSessionID, traceSessions[i].NfName)
	}

	w.Header().Add("Content-Type", "application/json")
	// TODO: tpaas alone puts the json data in etcd, so there is no error expected here. However, we
	// are still logging it for our own debug purposes during IT. Once tested, this error can be
	// ignored( or just continued to be debug logged maybe...)
	if err = json.NewEncoder(w).Encode(traceSessions); err != nil {
		logging.LogConf.Debug("[SLT-Get] Marshalling traceSession error", err)
	}
}

func Delay(w http.ResponseWriter, r *http.Request) {
	logging.LogConf.Info("Received delaysltoprequest")
	val := r.URL.Query().Get("seconds")
	if len(val) != 0 {
		iVal, err := strconv.Atoi(val)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		dst.Lock()
		dst.seconds = iVal
		dst.Unlock()
		logging.LogConf.Info("Delaying the slt ops for seconds - ", iVal)
	}
}

// Middleware to handle async response from tpaas
func asyncHandler(f func(*http.Request)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			logging.LogConf.Error(err.Error())
			writeProblemDetails(w, &problemDetails{"Error reading request", http.StatusBadRequest, "", nil})
			return
		}
		logging.LogConf.Debug("Request body before cloning =", string(body))
		r1 := r.Clone(r.Context())
		// clone body
		r1.Body = ioutil.NopCloser(bytes.NewReader(body))
		// send X-Operation-AsyncResponse = true header in the response to CIM to indicate that the response
		// from tpaas will be asynchronus.
		w.Header().Add("X-Operation-AsyncResponse", "true")
		// Make nf request creation and response collation code run in a separate goroutine.
		// Response from TPaaS to CIM would be asynchronous.
		go f(r1)
	}
}

func loggingHandler(next http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logging.LogConf.Info("[SLT] Recieved request", r.URL.EscapedPath(), r.Method)
		defer logging.LogConf.Info("[SLT] Exit", r.URL.EscapedPath(), r.Method)
		next.ServeHTTP(w, r)
	}

}

// AddRoutes will add the routes
func AddRoutes(r *mux.Router) {
	val, err := strconv.Atoi(os.Getenv("ETCDTIMEOUT_SECONDS"))
	if err != nil {
		logging.LogConf.Error("Invalid value for env var ETCDTIMEOUT_SECONDS. Default val will be used - ", EtcdTimeOutSec)
	} else {
		EtcdTimeOutSec = val
		logging.LogConf.Info("Etcd timeout set to seconds - ", val)
	}

	sltRouter := r.PathPrefix("/api/v1/tpaas/slt/sessions").Subrouter()
	sltRouter.HandleFunc("/{traceSessionID}", loggingHandler(asyncHandler(start))).Methods(http.MethodPost)
	sltRouter.HandleFunc("/{traceSessionID}", loggingHandler(asyncHandler(stop))).Methods(http.MethodDelete)
	sltRouter.HandleFunc("/{traceSessionID}/suspend", loggingHandler(asyncHandler(suspend))).Methods(http.MethodPut)
	sltRouter.HandleFunc("/{traceSessionID}/resume", loggingHandler(asyncHandler(resume))).Methods(http.MethodPut)
	sltRouter.HandleFunc("/", get).Methods(http.MethodGet)
	sltRouter.HandleFunc("/{traceSessionID}", get).Methods(http.MethodGet)
}
