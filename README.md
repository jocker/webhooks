**Review**
Please review the code from the master branch 

**Project packages**
- [common](https://github.com/jocker/webhooks/tree/master/common) contains all the code which is common to both slave and master
- [app](https://github.com/jocker/webhooks/tree/master/common/app) contains the initialization code common to both slave and master
- [storage](https://github.com/jocker/webhooks/tree/master/common/storage) defines the common [Store](https://github.com/jocker/webhooks/blob/master/common/storage/store.go) interface  and 2 implementations for it([dynamodb](https://github.com/jocker/webhooks/blob/master/common/storage/dynamo_store.go) and [s3](https://github.com/jocker/webhooks/blob/master/common/storage/s3_storage.go)) for storing/retrieving data received via webhooks
- [data](https://github.com/jocker/webhooks/tree/master/common/data) object mapping

**ObjectId**
- a common identifier for webhooks payloads received by both master and slave
- inspired by monogodb's [ObjectId](https://github.com/mongodb/mongo-go-driver/blob/master/bson/primitive/objectid.go)
- it contains 8 bytes - first 4 contain the unix timestamp for the record, last 4 contain a crc32c hash of the wekbook payload 
- the crc32c hash is obtained by 
    - getting a string representation of the object values - [code](https://github.com/jocker/webhooks/blob/master/common/json_reader.go#L123)
    - sorting the keys of the stringified version - [code](https://github.com/jocker/webhooks/blob/master/common/json_reader.go#L226)
    - calculate the hash of the result - [code](https://github.com/jocker/webhooks/blob/master/common/json_reader.go#L231-L238)
    
   
**How does it work**
- both slave and master can receive webhooks by posting a json to /webhook
- slave/master save the webhook data + their generated ObjectId in their corresponding Store. In this example, the slaves are saving the json in s3 and the master in dynamodb - please not that this is a demo where I wanted to show how would I use multiple store backends and also to get familiar with the aws stack. S3 would normally not be a good candidate for handling 100 reqs/second
- master periodically queries the slaves about missing records by posting a json in [this](https://github.com/jocker/webhooks/blob/master/common/things.go#L10) format. Basically, the master asks the slave to give it all the records which are between SlaveRangeStart and SlaveRangeEnd and whose ObjectIds are not included in MasterIds and which satisfy the +-1 minute condition. The code that does this is [here](https://github.com/jocker/webhooks/blob/master/slave/slave_server.go#L47)
- the slave replies back with a json array containing only the records which were not found in MasterIds

**Running the code**
- please note that >90% of the code is not tested, thus bugs are expected
- you'd have to start both [master](https://github.com/jocker/webhooks/blob/master/master/master_server.go) and [slave](https://github.com/jocker/webhooks/blob/master/slave/slave_server.go) servers
- make sure you have AWS_CREDENTIALS, REGION, DYNAMO_TABLE, S3_BUCKET env variables defined - AWS_CREDENTIALS needs to point to your local aws config file 


**Improvements**
- my approach for performing any sort of master-slave replication would rely grpc. 
    - The master would always have a direct connection with all the slaves and it would receive the slave data in realtime - thus we can have [ACID](https://en.wikipedia.org/wiki/ACID) transactions
    - as a fallback, in case the master is down I'd use pub/sub queues  (either sqs from aws, or pub/sub from google, or apache thrift, or kafka, etc)
- in case the crc32c hash is not enough, we can move to a 64bit one like [xxhash](https://github.com/cespare/xxhash)
- any processing should be done using streams of data and it shouldn't process all data at once

