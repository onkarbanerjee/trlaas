# TPaaS Performance Analysis  
Initially we have been using segmentio library to read from MTCIL kafka as well as same library to write into analytic kafka as well. However, we were observing a throughput of **just 1msg/sec** because of default batch timeout set to 1sec in segmentio writer config.
# Optimization
After making several fine tunings to the existing configurations and using different libraries as well, we could observe optimizations in throughput as detailed below.
## Note
- We used Infra Kafka as analytic kafka for testing purpose to eliminate latency due to external interface.  
- To count number of messages per second we implemented below prometheus metrics for both consuming from MTCIL kafka and publishing to Analytics kafka  
	- tpaas_total_messages_consumed  
	- tpaas_total_messages_processed  
- Required Acks set to 1 and BatchTimeout reduced to 1 ms in writer configs


|S.No|Library|Approach|Details|Output (msgs/sec)|Comments|  
|-----|---------|--------|-------|------|------|  
|1|Segmentio|single threaded with all logs enabled|Inorder processing reader, writer config optimized batch_timeout (used as 1millisecond)| 40 msgs/sec|Sequence is in order but throughput still low|  
|2|Segmentio|info only logs enabled|Too much logging was a suspect for low throughput in 1 above, disabled DEBUG logging  Mode|200 msgs/sec||
|3|Segmentio|error only logs enabled|Same as 2|200 msgs/sec||
|4|Segmentio|Tpaas to get nftype in key itself|some POC has been done to get nfType in kafka message key|185 msgs/sec|No impact on throughput|
|5|Segmentio|async writer|Inorder but async writing|240 msgs/sec|not viable because error handling is tricky and error prone|
|6|Segmentio|async batch writer|same as 5 except using configurable batch sizes(500)|280 msgs/sec|same as 5|
|7|Segmentio|multi consumer (consumer perpartition)|Partition based consumer used|250 msgs/sec|server side rebalance handling is tricky, no impact on performance, Not viable|
|8|Sarama|sync consumer/producer|In order, basic sarama implementation.|50 msgs/sec|oom issue because of sarama default metricregistry in sarama producer, tricky to handle, not viable.|
|9|Sarama|sarama consumer, segment producer|In order, basic sarama consumer(consumer per partition) and segmentio producer|420 msgs/sec|rebalancing issue on server side, tricky to handle, not viable. NOTE: cpu used 70m, memory used 120Mi|
|10|Confluent|sync consumer/producer 100m/100mi|Basic confluent implementation with fine tune configs cpu - 100m memory - 100Mi|280 msgs/sec|less memory footprint|
|11|Confluent|sync consumer/producer 200m/100mi|Basic confluent implementation with fine tune configs cpu - 200m memory - 100Mi|700 msgs/sec|all kafka settings configurable|
|12|Confluent|sync consumer/producer 300m/100mi|Basic confluent implementation with fine tune configs cpu - 300m memory - 100Mi|1000 msgs/sec||
|13|Confluent|sync consumer/producer 500m/100mi|Basic confluent implementation with fine tune configs cpu - 500m memory - 100Mi|1700 msgs/sec||
|14|Confluent|sync consumer/producer 1000m/100mi|Basic confluent implementation with fine tune configs cpu - 1000m memory - 100Mi|2800 msgs/sec|Though the more cpu is given utilization is in between 65-82% i.e 650m-820m cpu is getting utilized|
|15|Confluent|sync consumer/producer 1500m/100mi|Basic confluent implementation with fine tune configs cpu - 1500m memory - 100Mi|2800 msgs/sec|same as above|
