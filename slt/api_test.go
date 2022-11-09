package slt

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"trlaas/etcd"

	"github.com/coreos/etcd/clientv3"
	"github.com/gorilla/mux"
)

// TODO: Add more error scenarios, check for problemDetails returned
func TestStop(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		label,
		// input
		txnID, svcAttr string
		// mock
		sessionsKey string
		ttl         int64
		opsKeys     []string
		opsValues   []string
		// expected output
		expCode int
	}{
		{"Sunny Day",
			"d1a8d11c-b08b-4a02-ac33-aeddabeeeac0",
			"[{\"nfName\":\"appz\",\"nfsName\":\"test-appz\",\"svcVersion\":\"v0.15\",\"invocationMode\":\"multicast\",\"ttl\":600,\"serverUrl\":\"/smf-mgmtr/v1\",\"method\":\"put\"},{\"nfName\":\"appz\",\"nfsName\":\"test-appy\",\"svcVersion\":\"v0.16\",\"invocationMode\":\"multicast\",\"ttl\":600,\"serverUrl\":\"/nsmf-ipm/v1\",\"method\":\"put\"}]",
			"/tpaas/sessions/appz/3461b6d3-2eab-47d7-af84-6632ae2c95ff",
			int64(600),
			[]string{
				"/operations/request/appz/test-appz/v0.15/multicast/d1a8d11c-b08b-4a02-ac33-aeddabeeeac0",
				"/operations/request/appz/test-appy/v0.16/multicast/d1a8d11c-b08b-4a02-ac33-aeddabeeeac0"},
			[]string{
				`{"nfName":"appz","nfsName":"test-appz","transactionID":"d1a8d11c-b08b-4a02-ac33-aeddabeeeac0","reqBody":"","headers":{"X-Operation-TransactionId":"d1a8d11c-b08b-4a02-ac33-aeddabeeeac0"},"url":"/smf-mgmtr/v1/slt/sessions/3461b6d3-2eab-47d7-af84-6632ae2c95ff","method":"put"}`,
				`{"nfName":"appz","nfsName":"test-appy","transactionID":"d1a8d11c-b08b-4a02-ac33-aeddabeeeac0","reqBody":"","headers":{"X-Operation-TransactionId":"d1a8d11c-b08b-4a02-ac33-aeddabeeeac0"},"url":"/nsmf-ipm/v1/slt/sessions/3461b6d3-2eab-47d7-af84-6632ae2c95ff","method":"put"}`},
			http.StatusOK},
		{"Svc Attr missing in header",
			"d1a8d11c-b08b-4a02-ac33-aeddabeeeac0",
			"",
			"",
			int64(0),
			nil,
			nil,
			http.StatusBadRequest},
	}

	for _, tc := range testCases {
		// mock the etcd client response
		kv := &etcd.MockKV{
			GetKey:      tc.sessionsKey,
			GetResponse: &clientv3.GetResponse{},
			Mt: etcd.MockTxn{
				TxnResponse: clientv3.TxnResponse{},
				OpsKeys:     tc.opsKeys, OpsValues: tc.opsValues}}
		l := &etcd.MockLease{
			Ttl:           tc.ttl,
			LeaseResponse: &clientv3.LeaseGrantResponse{ID: 9999}}
		etcd.Client = &clientv3.Client{KV: kv, Lease: l}

		// make the http request
		r, err := http.NewRequest(http.MethodDelete, "/api/v1/tpaas/slt/sessions/3461b6d3-2eab-47d7-af84-6632ae2c95ff", nil)
		if err != nil {
			t.Fatal(tc.label, "Did not expect error, got", err)
		}
		r = mux.SetURLVars(r, map[string]string{"traceSessionID": "3461b6d3-2eab-47d7-af84-6632ae2c95ff"})
		r.Header.Set("X-Operation-TransactionId", tc.txnID)
		r.Header.Set("X-Operation-Service-Attributes", tc.svcAttr)
		w := httptest.NewRecorder()

		stop(w, r)

		actCode := w.Result().StatusCode
		if actCode != tc.expCode {
			t.Fatal(tc.label, "Expected", tc.expCode, ", got", actCode)
		}

		t.Log(tc.label, "passed")
	}

}
