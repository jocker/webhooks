**Payload hashing**
- checking if 2 payloads are identical shouldn't be done by comparing their actual value
    So we'll need to have a way of hashing the payloads and compare the hashes
- both master and slave need to have the same way of calculating the hash for any given json payload
- needs to be fast, thus a full json deserialization to a known type(like map[string]interface{}) shouldn't be done
- should be numeric - comparing numbers is way faster than comparing strings
- the json object keys of 2 identical payloads may have a different order - {"a":"b","c":"d} == {"c":"d", "a":"b"},
    thus keys need to be sorted before hashing the payload
- the hashing algorithm needs to be fast and needs to offer a good uniqueness factor
- the unique identifier for any given payload will be hash+timestamp
- will use crc32c as it should be supported by most cpu's

**Data persistence**
- only the master writes data to a "master" database, meaning that slaves aren't required to deserialize
    the webhook payload to a known type, however we should check if the payload is a well formatted json object.
    The slaves will save the json in a binary format in some database.
    This will significantly reduce the amount of computing power the slaves need - especially if we're expecting to have a high number of requests
    Using an embedded database would make a lot of sense for saving slave data https://github.com/dgraph-io/badger .
- data that needs to be persisted needs to be buffered - don't save the payload data as soon as we receive it.
    Flush interval on slaves needs to be smaller than the max timeSpan interval value -
            if master requests data between [-4, -2], flush interval needs to be < 2 minutes

**Steps involved in the sync process**
- master sends to slave a json array containing pairs of hash + timestamp - so the request payload will be limited and size predictable
    - The request payload will have this format [hash1, timestamp1,  ...., hashN, timestampN]
    - To make the slave lookup more efficient, the request payload needs to be sorted by timestamp - so any timestamp(N) >= timestamp(N-1)
    - The slave will throw an error unless the data is sorted
- slave will start processing the input immediately - assuming that our server supports http streaming,
        meaning that we might already have some results before the request payload is read completely.
        This will allow us to keep the memory footprint as low as possible
- slave will compare the received data with what is finds in its storage
    - loads all records for the given interval(starTime, endTime)
    - loops though each of them and check which of the records are already in master(based on the payload hash + startTime + endTime + timeSpan=2 minutes)
    - writes out to the output stream each record that does't satisfy the condition above - missing from master
    - the format for each entry in the response will be {hash: hashOfThePayloadJson, timestamp:timestampWhenRecordWasReceived, data:originalPayloadJson}
    - the slave response will be sorted by payload timestamp (master needs to check this)
- master reads whatever the slave sends and saves the data (if any) in it's database
- both slave and master should handle transmission errors gracefully
    - if during the sync the stream is closed, then slave should stop searching for records missing from master
    - master should always process any new chunk of data it received - even if the response ends abruptly
    - master should always start the sync using the timestamp of the last known synced record - sync(N).startTimestamp = sync(N-1).maxTimestamp


**aws lambda/serverless**
- no http2 support
- no grpc support
- no http1 streaming support
- all above probably not supported because calling the lambda function call is based on a
        synchronous rpc call by itself https://github.com/aws/aws-lambda-go/blob/master/lambda/entry.go#L56
- we can't reply to a request with binary data - json will never be more efficient than any sort of binary format
- we're forced to use json encoding and forced to use go's default json marshaller https://github.com/aws/aws-lambda-go/blob/master/lambda/entry.go#L37
- request payload size is limited
- no parallelism - an instance is created for each parallel request even if our code can handle a lot more than that
- nasty way for accessing request headers - https://aws.amazon.com/premiumsupport/knowledge-center/custom-headers-api-gateway-lambda/
- doesn't seem to be a good fit at all for replying to simple POST requests -
        requests are billed in 100ms increments, most probably ours will finish in a couple of milliseconds
- managing database connections will be a nightmare
- no os signalling - so we don't know when our instance is going to be shut down
- platform dependency - what happens if at any point we want to move to a different cloud(google, azure, etc)?
    How much of the code would have to be rewritten?
- can't see any real benefit for using go instead of using any other supported interpreted language
- can't see a any way of controlling the numbers of cpu's which are allocated to each instance - it seems to be dependent on the ram, but it's not linear
- should I go on? this seems to be an endless list of cons.

Based on the information I read so far, aws lambda looks to me like an unfinished product which I personally would not use or recommend.
It seems to be a good fit for occasional heavy duty workloads(file processing, video transcoding, etc), but not for short lived and highly frequent http requests

I'm not really familiar with aws's portfolio of services, but I am with google's, which include
- google's app engine, described as a fully managed serverless platform.
    - i've been using it for a couple of years now, never had any problem with it.
    - the major con is that deployments take a ridiculously amount of time. Longest one took ~20 minutes, shortest one ~5 minutes
    - can scale based on the instance load and not based on the number of requests
    - appengine flex allows running Docker containers
- cloud run - somehow similar to aws lambda - pay per time used, can scale from 0 to N instances
    - supports unary grpc requests - so no asynchronous requests, at least for now
    - is running containers, so we can provision any Docker container we would like instead of always running on amazon's linux distribution
    - we can mount any number of http endpoints - so we can have an entire api, or a microservice running on cloud run
Because it's container based, we can write standard go code(not aws go code) which we can deploy to appengine flex or cloud run.
