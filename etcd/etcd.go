package etcd

import (
	"context"
	"time"
	"trlaas/logging"
	"trlaas/types"

	"github.com/coreos/etcd/clientv3"
)

// ConnectEtcd is ConnectEtcd
var (
	Client *clientv3.Client
)

// Connect to etcd
func Connect() error {
	var i int
	var err error
	for i = 0; i < 60; i++ {
		Client, err = clientv3.New(clientv3.Config{
			Endpoints:   []string{types.CommonInfraConfigObj.EtcdURL},
			DialTimeout: 5 * time.Second,
		})
		if err != nil {
			logging.LogConf.Error("error while etcd connection.", err, " Retrying....")
			time.Sleep(1 * time.Second)
			continue
		}

		timeoutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		_, err = Client.Status(timeoutCtx, types.CommonInfraConfigObj.EtcdURL)
		if err != nil {
			logging.LogConf.Error("Error while checking etcd status", err.Error())
			continue
		}

		logging.LogConf.Info("Connected to etcd successfully")
		break
	}

	if err != nil {
		logging.LogConf.Error("Unable to connect etcd even after", i, "reiterations", "Terminanting service")
	}

	return err
}

// Close closes the etcd connection
func Close() error {
	return Client.Close()
}
