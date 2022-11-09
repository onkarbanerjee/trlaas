package types

var (
	TpaasConfigObj            *ConfigObj
	MtcilKafkaConsumerStarted bool
	CommonInfraConfigObj      *CommonInfraConf
)

type ConfigObj struct {
	TpaasConfig *TpaasConfig `json:"config"`
}
type TpaasConfig struct {
	App *Config    `json:"tpaas_settings,omitempty"`
	Slt *SltConfig `json:"slt,omitempty"`
}

type Config struct {
	LogLevel             string   `json:"log_level,omitempty"`
	AnalyticKafkaBrokers []string `json:"analytic_kafka_brokers,omitempty"`
	SchemaRegistryUrl    string   `json:"schema_registry_url,omitempty"`
	RetryCount           int      `json:"retry_count,omitempty"`
	NumOfPartitions      int      `json:"num_of_partitions,omitempty"`
	ReplicationFactor    int      `json:"replication_factor,omitempty"`
}

type SltConfig struct {
	MaxActivePerNf int `json:"maxActivePerNf,omitempty"`
}
type CommonInfraConf struct {
	EtcdURL string `json:"etcd_url"`
}
