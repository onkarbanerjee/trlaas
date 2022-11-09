package api

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"
	"trlaas/logging"
	"trlaas/types"
)

var (
	successCodes = make(map[int]string) //Expected Success codes from TMaaS
	failureCodes = make(map[int]string) //Expected Failure codes from TMaaS
)

func init() {
	successCodes[200] = "Specification registration successfull"
	successCodes[304] = "No Change in registration specification"
	failureCodes[400] = "Bad request"
}

// sendRegistrationReqToCim sends registration request to cim and gets the response.
func sendRegistrationReqToCim(reqBody []byte) error {
	cimport := "6060"
	var response *http.Response
	var err error
	for i := 0; i < 300; i++ {
		response, err = http.Post("http://localhost:"+cimport+"/api/v1/_operations/specification/register", "application/json", bytes.NewBuffer(reqBody))
		if err != nil {
			logging.LogConf.Error("The HTTP request failed with error ", err.Error())
			return err
		}
		if successMsg, ok := successCodes[response.StatusCode]; ok {
			logging.LogConf.Info(successMsg, "StatusCode :", response.StatusCode, "Status :", response.Status)
			body, _ := ioutil.ReadAll(response.Body)
			defer response.Body.Close()
			logging.LogConf.Info("Response Header :", response.Header)
			logging.LogConf.Info("Response Body :", string(body))
			return nil
		} else if failureMsg, ok := failureCodes[response.StatusCode]; ok {
			logging.LogConf.Error(failureMsg, "StatusCode :", response.StatusCode, "Status :", response.Status)
			return fmt.Errorf("Specification registration failure. StatusCode : %d Response Message : %s", response.StatusCode, failureMsg)
		} else {
			logging.LogConf.Error("specification registration failure", "StatusCode :", response.StatusCode, "Status :", response.Status)
		}
		time.Sleep(time.Second * 5)
		logging.LogConf.Info("Going for retry", i)
	}
	logging.LogConf.Error("specification registration failure even after", types.TpaasConfigObj.TpaasConfig.App.RetryCount, "Retries")
	return fmt.Errorf("Specification registration failure Even after 60 Retries(300 secs)")
}

// RegisterAPI Register's SLT API's
func RegisterAPI() (err error) {
	specData, err := ioutil.ReadFile("/opt/conf/static/specification-registration.json")
	if err != nil {
		logging.LogConf.Error("Error in reading", "specification-registration.json file", err.Error())
	}
	logging.LogConf.Info("specification-registration.json value", string(specData[:]))
	err = sendRegistrationReqToCim(specData)
	if err != nil {
		logging.LogConf.Error("Specification registration failure", err)
		return
	}
	logging.LogConf.Info("Specification registration OK")
	return
}
