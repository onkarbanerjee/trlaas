package events

import (
	"errors"
	"time"
	fbEvent "trlaas/flatbuf/EventInterface"
	"trlaas/logging"

	fb "github.com/google/flatbuffers/go"
	//"golang.org/x/net/http2"
	//"crypto/tls"
	//"runtime"
)

func buildKeyValueObject(builder *fb.Builder, input_object_key []string, input_object_val []string, dataType string) fb.UOffsetT {
	var values []fb.UOffsetT
	//values := make([]interface{}, 0)

	for i, _ := range input_object_key {

		//key_val_obj = append(key_val_obj, obj)

		moKey := builder.CreateString(input_object_key[i])
		moVal := builder.CreateString(input_object_val[i])
		fbEvent.KeyValueStart(builder)
		fbEvent.KeyValueAddKey(builder, moKey)
		fbEvent.KeyValueAddValue(builder, moVal)
		mo := fbEvent.KeyValueEnd(builder)
		values = append(values, mo)
	}
	switch dataType {

	case "managed":
		fbEvent.EventStartManagedObjectVector(builder, len(values))
		for _, v := range values {
			builder.PrependUOffsetT(v)
		}
	case "additional":
		fbEvent.EventStartAdditionalInfoVector(builder, len(values))
		for _, v := range values {
			builder.PrependUOffsetT(v)
		}

	case "statechangedevent":
		fbEvent.EventStartStateChangeDefinitionVector(builder, len(values))
		for _, v := range values {
			builder.PrependUOffsetT(v)
		}
	case "threshold":

		fbEvent.EventStartThresholdInfoVector(builder, len(values))
		for _, v := range values {
			builder.PrependUOffsetT(v)
		}
	case "monitoredattributes":

		fbEvent.EventStartMonitoredAttributesVector(builder, len(values))
		for _, v := range values {
			builder.PrependUOffsetT(v)

		}
	}
	dataTypeObject := builder.EndVector(len(values))
	return dataTypeObject
}

func createEventMsg(event_name string, manage_details_keys []string, manage_details_val []string, additional_info_key []string, additional_info_val []string) []byte {

	builder := fb.NewBuilder(0)
	eventName := builder.CreateString(event_name)
	containerID := logging.ContainerID

	cntID := builder.CreateString(containerID)
	//cntName := builder.CreateString("tpaas")

	//fbPayload := builder.CreateString(logs)
	managedObj := buildKeyValueObject(builder, manage_details_keys, manage_details_val, "managed")

	additionalInfo := buildKeyValueObject(builder, additional_info_key, additional_info_val, "additional")
	etime := time.Now().UnixNano() / int64(time.Millisecond)
	fbEvent.EventStart(builder)
	fbEvent.EventAddEventName(builder, eventName)
	fbEvent.EventAddEventTime(builder, etime)
	fbEvent.EventAddContainerId(builder, cntID)
	fbEvent.EventAddManagedObject(builder, managedObj)
	fbEvent.EventAddAdditionalInfo(builder, additionalInfo)

	eventMsg := fbEvent.EventEnd(builder)
	builder.Finish(eventMsg)
	return builder.FinishedBytes()
}
func RaiseEvent(event_name string, manage_details_keys []string, manage_details_val []string, additional_info_key []string, additional_info_val []string) error {
	if len(event_name) < 1 {
		logging.LogConf.Error("Event name is mandatory")
		err := errors.New("Mandatory parameter event name missing.")
		return err
	}
	logging.LogConf.Debug("Raising Event for", event_name)
	eventByteData := createEventMsg(event_name, manage_details_keys, manage_details_val, additional_info_key, additional_info_val)

	err := logging.NatsConn.Publish("EVENT", eventByteData)
	if err != nil {
		logging.LogConf.Error("error while publishing message to nats-" + err.Error())
		return err
	}
	logging.NatsConn.Flush()
	return nil
}
