package config

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"time"
	"trlaas/logging"
	"trlaas/types"
)

func defaultConfig() {
	var content []byte
	var err error
	content, err = ioutil.ReadFile("/opt/config/tpaasConfig.json")
	if err != nil {
		logging.LogConf.Exception("Error in reading Tpaas config", err.Error())
	}
	log.Println("Initial config data", string(content[:]))
	data := types.TpaasConfigObj
	err = json.Unmarshal(content, &data)
	if err != nil {
		panic(err)
	}
	types.TpaasConfigObj = data
	return
}

func commonInfraConfig() {
	commonInfraPath := "/opt/conf/static/common-infra.json"

	_, err := os.Stat(commonInfraPath)
	if os.IsNotExist(err) {
		logging.LogConf.Exception("Error in reading common-infra config, not exist", commonInfraPath)
	}

	content, err := ioutil.ReadFile(commonInfraPath)
	if err != nil {
		logging.LogConf.Exception("Error while reading common infra config from ", commonInfraPath, err, "Terminating Self.")
	}

	fmt.Println("Common infra content", string(content))
	logging.LogConf.Info("Common infra config data", string(content[:]))
	data := types.CommonInfraConfigObj
	err = json.Unmarshal(content, &data)
	if err != nil {
		logging.LogConf.Exception("Error while unmarshalling common-infra configuration data", err, "Terminating Self")
	}
	types.CommonInfraConfigObj = data
	<-time.After(5 * time.Second)
	fmt.Println("etcd url", types.CommonInfraConfigObj.EtcdURL)
	return
}

// Load will load the default config and the common infra config
func Load() {
	fmt.Println("Loading default config")
	defaultConfig()
	fmt.Println("Loading common infra config----1")
	commonInfraConfig()
}
