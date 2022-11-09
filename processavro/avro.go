package processavro

/*import (
	"context"
	"sync"

	//"fmt"
	"os"

	"gopkg.in/avro.v0"

	"io/ioutil"
	"log"
	"strconv"
	"strings"
	"time"
	trlcim "trlaas/flatbuf/TrlCimInterface"
	trl "trlaas/flatbuf/TrlInterface"
	conn "trlaas/kafka"
	"trlaas/logging"
)

var (
	PodID            string
	namespace        string
	microserviceName string
	fileMap          = make(map[string]*sync.RWMutex)
)

type TransactionDetails struct {
	//	ContainerID string
	RecordName string
	Details    []KeyValueObjects
}

type KeyValueObjects struct {
	Name  string      `json:"name"`
	Value interface{} `json:"value"`
	Type  string      `json:"type"`
}

//ProcessTRLMessages process trl messages from kafka
func ProcessTRLMessages() {
	// connect to kafka
	reader := conn.GetKafkaReader()

	logging.LogForwarder("start consuming ... !!")
	for {
		m, err := reader.ReadMessage(context.Background())
		if err != nil {
			log.Fatalln(err)
		}
		//fmt.Printf("message at topic:%v partition:%v offset:%v	key:%v value:%v \n", m.Topic, m.Partition, m.Offset, m.Key, m.Value)
		logging.LogForwarder("message at topic:" + m.Topic + " partition:" + string(m.Partition) + " offset:" + string(m.Offset) + "\n")

		go decodeTRLMessages(string(m.Key), m.Value)
	}
}

//decodeTRLMessages decode the messages
func decodeTRLMessages(key string, data []byte) {

	trlMsg := trlcim.GetRootAsTRLCim(data, 0)

	var (
		mtcilHeaders TransactionDetails

		transactionDetails []TransactionDetails

		mtcilData []KeyValueObjects
	)
	mtcilHeaders.RecordName = "mtcilHeaders"

	for i := 0; i < trlMsg.MTCILHeaderLength(); i++ {
		trlObj := new(trlcim.TrlData)

		trlMsg.MTCILHeader(trlObj, i)

		var kv KeyValueObjects
		if string(trlObj.Key()) == "podID" {
			PodID = string(trlObj.Value())
		}
		if string(trlObj.Key()) == "namespace" {
			namespace = string(trlObj.Value())
		}
		if string(trlObj.Key()) == "microserviceName" {
			microserviceName = string(trlObj.Value())
		}

		kv.Name = string(trlObj.Key())
		kv.Value = string(trlObj.Value())
		kv.Type = string(trlObj.Type())
		mtcilData = append(mtcilData, kv)
	}

	mtcilHeaders.Details = mtcilData
	transactionDetails = append(transactionDetails, mtcilHeaders)

	//get the actual trl sent by app
	trlDetails := trlMsg.TrlBytes()
	trlData := trl.GetRootAsTrl(trlDetails, 0)

	//get transaction  details
	for i := 0; i < trlData.TrlDetailsLength(); i++ {
		trlObj := new(trl.TrlDetails)
		var m TransactionDetails
		trlData.TrlDetails(trlObj, i)
		m.RecordName = string(trlObj.RecordName())
		var d []KeyValueObjects

		for j := 0; j < trlObj.TrlSpecLength(); j++ {

			trlData := new(trl.TrlSpec)
			trlObj.TrlSpec(trlData, j)
			var kv KeyValueObjects

			dType, value := convertToPrimitiveTypes(string(trlData.Type()), string(trlData.Value()))
			if dType == "" {
				//log.Println("not a valid type", string(trlData.Type()), " for kry ", string(trlData.Key()))
				logging.LogForwarder("not a valid type " + string(trlData.Type()) + " for key " + string(trlData.Key()) + ". Ignoring this TRL message.")
				return
			}

			kv.Name = string(trlData.Key())
			kv.Value = value
			kv.Type = dType
			d = append(d, kv)
		}
		m.Details = d
		transactionDetails = append(transactionDetails, m)

	}

	//generate avro schema
	schema := generateAvroSchema(transactionDetails)
	logging.LogForwarder("generated schema is : " + schema)

	processAvroSchema(schema, key, transactionDetails)

}

func generateAvroSchema(trlInfo []TransactionDetails) string {
	var schema string
	logging.LogForwarder("generation avro schema started at: " + time.Now().String())
	schema = `{
		"type" : "record",
		"name" : "` + microserviceName + `",
		"namespace" : "` + namespace + "." + PodID + `",
		"fields" : [`
	for _, data := range trlInfo {
		schema = schema + `{
		"name": "` + strings.ToLower(data.RecordName) + `",
		"type" : {
			"type" : "record",
			"name" : "` + strings.ToLower(data.RecordName) + `",
		"fields": [`
		for _, tdata := range data.Details {
			schema = schema + `
				{"name":"` + tdata.Name + `", "type":"` + tdata.Type + `"},`
		}

		schema = strings.TrimRight(schema, ",")
		schema = schema + `
		]}},`
	}

	schema = strings.TrimRight(schema, ",")
	schema = schema + `]}`

	logging.LogForwarder("generation avro schema fninshed at: " + time.Now().String())

	return schema

}

func processAvroSchema(rawSchema, key string, trlInfo []TransactionDetails) {

	// Parse the schema first
	schema := avro.MustParseSchema(rawSchema)

	// Create a record for a given schema
	record := avro.NewGenericRecord(schema)
	logging.LogForwarder("data encoding started at :" + time.Now().String())
	for _, t := range trlInfo {
		subRecord := avro.NewGenericRecord(schema)
		for _, tdata := range t.Details {
			subRecord.Set(tdata.Name, tdata.Value)
		}
		record.Set(strings.ToLower(t.RecordName), subRecord)
	}

	err := recordWriter(schema, record, key)
	if err != nil {
		logging.LogForwarder("error while writing thr avro record :" + err.Error())
		return
	}

	logging.LogForwarder("data encoding finished at :" + time.Now().String())

	err = sendAvroMsgsToKafka(key)
	if err != nil {
		logging.LogForwarder("error while sending  avro record to kafka :" + err.Error())
		return
	}

}

func recordWriter(schema avro.Schema, record *avro.GenericRecord, key string) error {
	_, ok := fileMap[key]
	if !ok {
		fileMap[key] = new(sync.RWMutex)
	}

	fileMap[key].Lock()
	defer fileMap[key].Unlock()
	f, err := os.Create("./" + key + ".avro")
	if err != nil {
		return err
	}
	defer f.Close()

	writer := avro.NewGenericDatumWriter()

	fileWriter, err := avro.NewDataFileWriter(f, schema, writer)
	if err != nil {
		return err
	}

	// Write the record
	err = fileWriter.Write(record)
	if err != nil {
		return err
	}

	fileWriter.Flush()

	return nil

}

//sendAvroMsgsToKafka send trls to kafka
func sendAvroMsgsToKafka(key string) error {
	fileMap[key].RLock()
	defer fileMap[key].RUnlock()
	enData, err := ioutil.ReadFile("./" + key + ".avro")
	if err != nil {
		logging.LogForwarder("File reading error " + err.Error())
		return err
	}

	// connect to kafka
	kafkaProducer := conn.GetKafkaWriter("trl")
	if err != nil {
		logging.LogForwarder("unable to configure kafka " + err.Error())
		return err
	}

	//newKafkaPublisher("chatMessage", kafkaProducer)
	err = conn.Push(enData, kafkaProducer)
	if err != nil {
		logging.LogForwarder("error while pushing data into kafka" + err.Error())
		return err
	}
	logging.LogForwarder("kafka write is successful")
	return nil

}

//convertToPrimitiveTypes type conversion handling
func convertToPrimitiveTypes(dataType, value string) (string, interface{}) {
	var (
		err error
		s   interface{}
	)

	switch dataType {
	case "bool":
		if s, err = strconv.ParseBool(value); err == nil {
			return "boolean", s
		}
		logging.LogForwarder("not an bool type " + err.Error())
	case "float32":
		if s, err = strconv.ParseFloat(value, 32); err == nil {
			return "float", s
		}
		logging.LogForwarder("not a float type " + err.Error())
	case "float64":
		if s, err = strconv.ParseFloat(value, 64); err == nil {
			return "double", s
		}
		logging.LogForwarder("not a double type " + err.Error())
	case "int64":
		if s, err = strconv.ParseInt(value, 10, 64); err == nil {
			return "long", s
		}
		logging.LogForwarder("not a long type " + err.Error())
	case "int32":
		if s, err = strconv.ParseInt(value, 10, 32); err == nil {
			return "int32", s
		}
		logging.LogForwarder("not a int32 type " + err.Error())
	case "int":
		if s, err = strconv.Atoi(value); err == nil {
			return "int", s
		}
		logging.LogForwarder("not a int type " + err.Error())
	case "string":
		return "string", value
	case "[]byte":
		return "bytes", []byte(value)
	default:
		log.Println("not a valid type " + dataType)
	}

	return "", nil
}*/
