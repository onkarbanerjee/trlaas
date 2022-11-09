package kafka

import (
	"context"
	"fmt"

	"os"

	"strings"
	"sync"
	"time"
	"trlaas/events"
	"trlaas/logging"
	"trlaas/promutil"
	"trlaas/types"

	"github.com/confluentinc/confluent-kafka-go/kafka"
)

// LastOffset is LastOffset
const (
	LastOffset  int64 = -1
	FirstOffset int64 = -2
)

type nbiEvents struct {
	*sync.RWMutex
	// counter for tracking consecutive failures in sending msgs to XA kafka topics. eg. events["TRL-test"] = 10
	// the counter is removed immediately once a msg is sent successfully to that topic.
	// used for tracking and generating TpaasNbiFailure/Success event for a given topic.
	events map[string]int
}

var (
	producer    *kafka.Producer
	TrlConsumer *kafka.Consumer
	SltConsumer *kafka.Consumer
	client      *kafka.AdminClient
	evs         = nbiEvents{&sync.RWMutex{}, make(map[string]int)}
	countv      int
	msgc        uint64
)

func InitializeConfluentConfigs() (err error) {
	var i int

	for ; i <= types.TpaasConfigObj.TpaasConfig.App.RetryCount; i++ {
		TrlConsumer, err = kafka.NewConsumer(
			&kafka.ConfigMap{
				"bootstrap.servers": os.Getenv("MTCIL_KAFKA_BROKERS"),
				"group.id":          "tpaas-TRL",
			},
		)
		if err != nil {
			fmt.Println("Error while creating trl consumer", err)
			continue
		}

		SltConsumer, err = kafka.NewConsumer(
			&kafka.ConfigMap{
				"bootstrap.servers": os.Getenv("MTCIL_KAFKA_BROKERS"),
				"group.id":          "tpaas-SLT",
			},
		)
		if err != nil {
			fmt.Println("Error while creating trl consumer", err)
			continue
		}

		producer, err = kafka.NewProducer(
			&kafka.ConfigMap{
				"bootstrap.servers":       types.TpaasConfigObj.TpaasConfig.App.AnalyticKafkaBrokers[0],
				"go.events.channel.size":  10000,
				"go.produce.channel.size": 10000,
			},
		)
		if err != nil {
			fmt.Println("Error while creating producer", err)
			continue
		}

		client, err = kafka.NewAdminClient(
			&kafka.ConfigMap{
				"bootstrap.servers": types.TpaasConfigObj.TpaasConfig.App.AnalyticKafkaBrokers[0],
			},
		)
		if err != nil {
			fmt.Println("Error while creating Admin client", err)
			continue
		}
		return
	}
	return
}

func processProducerChannelErrors(message []byte, topic string, errorType error) {
	for {
		if strings.Contains(errorType.Error(), kafka.ErrTopicException.String()) || strings.Contains(errorType.Error(), kafka.ErrUnknownTopicOrPart.String()) {
			err := CreateTopic(topic)
			if err != nil {
				logging.LogConf.Error("Error while creating Analytic kafka TOPIC:", topic, " error:", err)
				evs.Lock()
				// check if there is an existing retry counter for the topic. if not create one
				if _, exist := evs.events[topic]; !exist {
					evs.events[topic] = 0
				}
				if evs.events[topic] == types.TpaasConfigObj.TpaasConfig.App.RetryCount {
					logging.LogConf.Error("Error while pushing data into Analytic kafka - RaisingEvent and retrying", err.Error(), " RETRIES=", evs.events[topic])
					eventErr := events.RaiseEvent("TpaasNbiFailure", []string{"Description", "STATUS", "Topic"}, []string{"Analytic Kafka Connection", "Failure", topic}, []string{"Error"}, []string{err.Error()})
					if eventErr != nil {
						logging.LogConf.Error("error while notifying Analytic Kafka Connection failure to FMaaS for topic", topic, "Error", eventErr)
					}
				}
				// increment the counter
				evs.events[topic]++
				evs.Unlock()
				<-time.After(time.Second)
				continue
			}
		}
		Push(message, topic)
		logging.LogConf.Debug("Push to Analytic kafka successful for topic:", topic)
		break
	}
}

func FlushMsgs() {
	//This goroutine is responsible for checking
	//Errors on producer events channel
	go func() {
		for e := range producer.Events() {
			switch ev := e.(type) {
			case *kafka.Message:
				if ev.TopicPartition.Error != nil {
					logging.LogConf.Error("Error while pushing to Analytic kafka TOPIC:", *ev.TopicPartition.Topic, " error:", ev.TopicPartition.Error, "Event:", ev.TopicPartition)
					processProducerChannelErrors(ev.Value, *ev.TopicPartition.Topic, ev.TopicPartition.Error)
				} else {
					evs.Lock()
					// raise the success event only if the retry counter exist and the no of retries is more than the configured RetryCount.
					if numRetries, exist := evs.events[*ev.TopicPartition.Topic]; exist && numRetries > types.TpaasConfigObj.TpaasConfig.App.RetryCount {
						eventErr := events.RaiseEvent("TpaasNbiSuccess", []string{"Description", "STATUS", "Topic"}, []string{"Analytic Kafka Connection", "Success", *ev.TopicPartition.Topic}, []string{}, []string{})
						if eventErr != nil {
							logging.LogConf.Error("error while notifying Analytic Kafka Connection success to FMaaS topic", *ev.TopicPartition.Topic, "Error", eventErr)
						}
						delete(evs.events, *ev.TopicPartition.Topic)
					}
					evs.Unlock()

					err := promutil.CounterAdd("tpaas_total_messages_processed", 1, map[string]string{"topic": *ev.TopicPartition.Topic})
					if err != nil {
						logging.LogConf.Error("Error while adding values in counter for tpaas_total_messages_processed by TPaaS.", err)
					}

					if countv == 0 {
						logging.LogConf.Info("Analytic kafka write is successful for topic ", *ev.TopicPartition.Topic)
						logging.LogConf.Info("Pushed ", msgc+1, "message to analytic kafka-", string(ev.Value))
					} else if countv == 1000 {
						logging.LogConf.Info("Analytic kafka write is successful for topic ", *ev.TopicPartition.Topic)
						logging.LogConf.Info("Pushed ", msgc+1, "message to analytic kafka-", string(ev.Value))
						countv = 0
					} else {
						logging.LogConf.Debug("Analytic kafka write is successful for topic ", *ev.TopicPartition.Topic)
					}
					countv = countv + 1
					if msgc == 18446744073709551615 {
						msgc = 0
					} else {
						msgc = msgc + 1
					}
				}
			}
		}
	}()
	//Flush the messages in regular intervals
	//TODO:- The interval here 15secs can be configured, Needs discussion
	for {
		producer.Flush(1000 * 15)
	}
}

func Push(val []byte, t string) {
	message := &kafka.Message{
		TopicPartition: kafka.TopicPartition{
			Topic:     &t,
			Partition: kafka.PartitionAny,
		},
		Value:     val,
		Timestamp: time.Now(),
	}
	producer.ProduceChannel() <- message
}

// Close will close all the writers being maintained as well as the connection to kafka brokers
func Close() {
	logging.LogConf.Info("Closing all kafka related connections")
	if SltConsumer != nil {
		SltConsumer.Close()
	}
	if TrlConsumer != nil {
		TrlConsumer.Close()
	}
	if producer != nil {
		producer.Close()
	}
	if client != nil {
		client.Close()
	}
}

// CreateTopic create topic in Kafka
func CreateTopic(topic string) error {
	td := kafka.TopicSpecification{}
	td.NumPartitions = types.TpaasConfigObj.TpaasConfig.App.NumOfPartitions
	td.ReplicationFactor = types.TpaasConfigObj.TpaasConfig.App.ReplicationFactor
	td.Topic = topic
	_, err := client.CreateTopics(context.Background(), []kafka.TopicSpecification{td})
	if err != nil {
		logging.LogConf.Error("CreateTopic: Error while creating topic ", topic, err)
		return fmt.Errorf("CreateTopic: Error while creating topic %s %s", topic, err)
	}
	logging.LogConf.Info("Topic", topic, "Created Successfully")
	return nil
}
