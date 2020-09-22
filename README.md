
## Overview

When benchmarking a Pub/Sub Systems, we specifically require two distinct roles ( publishers and subscribers ) as benchmark participants - this repo contains code to mimic the subscriber workload on Redis Pub/Sub.

Several aspects can dictate the overall system performance, like the:
- Payload size (controlled on publisher)
- Number of Pub/Sub channels (controlled on publisher)
- Total message traffic per channel (controlled on publisher)
- Number of subscribers per channel (controlled on subscriber)
- Subscriber distribution per shard and channel (controlled on subscriber)

## Installation

The easiest way to get and install the Subscriber Go program is to use
`go get` and then `go install`:
```bash
# Fetch this repo
go get github.com/filipecosta90/pubsub-sub-bench
cd $GOPATH/src/github.com/filipecosta90/pubsub-sub-bench
make
```

## Usage of pubsub-sub-bench

```
Usage of pubsub-sub-bench:
  -channel-maximum int
        channel ID maximum value ( each channel has a dedicated thread ). (default 100)
  -channel-minimum int
        channel ID minimum value ( each channel has a dedicated thread ). (default 1)
  -client-output-buffer-limit-pubsub string
        Specify client output buffer limits for clients subscribed to at least one pubsub channel or pattern. If the value specified is different that the one present on the DB, this setting will apply.
  -client-update-tick int
        client update tick. (default 1)
  -host string
        redis host. (default "127.0.0.1")
  -json-out-file string
        Name of json output file, if not set, will not print to json.
  -messages int
        Number of total messages per subscriber per channel.
  -oss-cluster-api-distribute-subscribers
        read cluster slots and distribute subscribers among them.
  -port string
        redis port. (default "6379")
  -print-messages
        print messages.
  -subscriber-prefix string
        prefix for subscribing to channel, used in conjunction with key-minimum and key-maximum. (default "channel-")
  -subscribers-per-channel int
        number of subscribers per channel. (default 1)
  -subscribers-placement-per-channel string
        (dense,sparse) dense - Place all subscribers to channel in a specific shard. sparse- spread the subscribers across as many shards possible, in a round-robin manner. (default "dense")
  -test-time int
        Number of seconds to run the test, after receiving the first message.
```
