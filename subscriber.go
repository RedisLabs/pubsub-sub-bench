package main

import (
	"flag"
	"fmt"
	"github.com/mediocregopher/radix"
	"log"
	"math/rand"
	"os"
	"os/signal"
	"strings"
	"sync"
	"text/tabwriter"
	"time"
)

func subscriberRoutine(addr string, subscriberName string, channel string, printMessages bool, stop chan struct{}, wg *sync.WaitGroup) {
	// tell the caller we've stopped
	defer wg.Done()

	conn, err, ps, msgCh, tick := bootstrapPubSub(addr, subscriberName, channel)
	defer conn.Close()
	defer tick.Stop()

	for {
		select {
		case <-tick.C:
			err = ps.Ping()
			if err != nil {
				//try to bootstrap again
				conn, err, ps, msgCh, tick = bootstrapPubSub(addr, subscriberName, channel)
				defer conn.Close()
				defer tick.Stop()
				if err != nil {
					panic(err)
				}
			}
			// loop

		case msg := <-msgCh:
			if printMessages {
				fmt.Println(fmt.Sprintf("received message in channel %s. Message: %s", msg.Channel, msg.Message))
			}

			break
		case <-stop:
			return
		}
	}
}

func bootstrapPubSub(addr string, subscriberName string, channel string) (radix.Conn, error, radix.PubSubConn, chan radix.PubSubMessage, *time.Ticker) {
	// Create a normal redis connection
	conn, err := radix.Dial("tcp", addr)

	err = conn.Do(radix.FlatCmd(nil, "CLIENT", "SETNAME", subscriberName))
	if err != nil {
		log.Fatal(err)
	}

	if err != nil {
		panic(err)
	}
	// Pass that connection into PubSub, conn should never get used after this
	ps := radix.PubSub(conn)

	msgCh := make(chan radix.PubSubMessage)
	err = ps.Subscribe(msgCh, channel)
	if err != nil {
		panic(err)
	}
	tickTime := 10 + rand.Intn(10)
	tick := time.NewTicker(time.Duration(tickTime) * time.Second)

	return conn, err, ps, msgCh, tick
}

func main() {
	host := flag.String("host", "127.0.0.1", "redis host.")
	port := flag.Int("port", 6379, "redis port.")
	channel_minimum := flag.Int("channel-minimum", 1, "channel ID minimum value ( each channel has a dedicated thread ).")
	channel_maximum := flag.Int("channel-maximum", 100, "channel ID maximum value ( each channel has a dedicated thread ).")
	subscribers_per_channel := flag.Int("subscribers-per-channel", 1, "number of subscribers per channel.")
	client_update_tick := flag.Int("client-update-tick", 1, "client update tick.")
	subscribe_prefix := flag.String("subscriber-prefix", "channel-", "prefix for subscribing to channel, used in conjunction with key-minimum and key-maximum.")
	client_output_buffer_limit_pubsub := flag.String("client-output-buffer-limit-pubsub", "", "Specify client output buffer limits for clients subscribed to at least one pubsub channel or pattern. If the value specified is different that the one present on the DB, this setting will apply.")
	distributeSubscribers := flag.Bool("oss-cluster-api-distribute-subscribers", false, "read cluster slots and distribute subscribers among them.")
	printMessages := flag.Bool("print-messages", false, "print messages.")
	flag.Parse()

	// Create a normal redis connection
	conn, err := radix.Dial("tcp", fmt.Sprintf("%s:%d", *host, *port))
	if err != nil {
		panic(err)
	}

	var nodes []radix.ClusterNode
	if *distributeSubscribers {
		var topology radix.ClusterTopo
		err := conn.Do(radix.FlatCmd(&topology, "CLUSTER", "SLOTS"))
		if err != nil {
			log.Fatal(err)
		}

		for _, slot := range topology.Map() {
			slot_host := strings.Split(slot.Addr, ":")[0]
			slot_port := strings.Split(slot.Addr, ":")[1]
			if strings.Compare(slot_host, "127.0.0.1") == 0 {
				slot.Addr = fmt.Sprintf("%s:%s", *host, slot_port)
			}
			nodes = append(nodes, slot)
		}
	} else {
		nodes = []radix.ClusterNode{{
			Addr:            fmt.Sprintf("%s:%d", *host, *port),
			ID:              "",
			Slots:           nil,
			SecondaryOfAddr: "",
			SecondaryOfID:   "",
		}}
	}
	conn.Close()
	if strings.Compare(*client_output_buffer_limit_pubsub, "") != 0 {
		checkClientOutputBufferLimitPubSub(nodes, client_output_buffer_limit_pubsub)
	}

	// a channel to tell `tick()` and `tock()` to stop
	stopChan := make(chan struct{})

	// a WaitGroup for the goroutines to tell us they've stopped
	wg := sync.WaitGroup{}

	for channel_id := *channel_minimum; channel_id <= *channel_maximum; channel_id++ {
		for channel_subscriber_number := 1; channel_subscriber_number <= *subscribers_per_channel; channel_subscriber_number++ {
			nodes_pos := (channel_id * channel_subscriber_number) % len(nodes)
			addr := nodes[nodes_pos]
			channel := fmt.Sprintf("%s%d", *subscribe_prefix, channel_id)
			subscriberName := fmt.Sprintf("subscriber#%d-%s%d", channel_subscriber_number, *subscribe_prefix, channel_id)
			wg.Add(1)
			go subscriberRoutine(addr.Addr, subscriberName, channel, *printMessages, stopChan, &wg)
		}

	}

	// listen for C-c
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)

	tick := time.NewTicker(time.Duration(*client_update_tick) * time.Second)
	var connections []radix.Conn
	if updateCLI(nodes, connections, tick, c) {
		return
	}

	// tell the goroutine to stop
	close(stopChan)
	// and wait for them both to reply back
	wg.Wait()

}

func updateCLI(nodes []radix.ClusterNode, connections []radix.Conn, tick *time.Ticker, c chan os.Signal) bool {
	w := new(tabwriter.Writer)
	w.Init(os.Stdout, 8, 0, 1, ' ', tabwriter.AlignRight)
	for idx, slot := range nodes {
		c, err := radix.Dial("tcp", slot.Addr)
		if err != nil {
			panic(err)
		}
		fmt.Fprint(w, fmt.Sprintf("s%d\t", idx))
		connections = append(connections, c)
		//defer c.Close()
	}
	fmt.Fprint(w, "\n")
	w.Flush()
	for {
		select {
		case <-tick.C:
			{
				for _, c := range connections {
					var infoOutput string
					e := c.Do(radix.FlatCmd(&infoOutput, "INFO", "CLIENTS"))
					if e != nil {
						fmt.Fprint(w, fmt.Sprintf("----\t"))
					} else {
						connected_clients_line := strings.TrimSuffix(strings.Split(infoOutput, "\r\n")[1], "\r\n")
						i := strings.Index(connected_clients_line, ":")
						fmt.Fprint(w, fmt.Sprintf("%s\t", connected_clients_line[i+1:]))
					}
				}
				fmt.Fprint(w, "\r\n")
				w.Flush()

				break
			}

		case <-c:
			fmt.Println("received Ctrl-c - shutting down")
			return true
		}
	}
	return false
}

func checkClientOutputBufferLimitPubSub(nodes []radix.ClusterNode, client_output_buffer_limit_pubsub *string) {
	for _, slot := range nodes {
		//fmt.Println(slot)
		conn, err := radix.Dial("tcp", slot.Addr)
		if err != nil {
			panic(err)
		}
		_, err, pubsubTopology := getPubSubBufferLimit(err, conn)
		if strings.Compare(*client_output_buffer_limit_pubsub, pubsubTopology) != 0 {
			fmt.Println(fmt.Sprintf("\tCHANGING DB pubsub topology for address %s from %s to %s", slot.Addr, pubsubTopology, *client_output_buffer_limit_pubsub))

			err = conn.Do(radix.FlatCmd(nil, "CONFIG", "SET", "client-output-buffer-limit", fmt.Sprintf("pubsub %s", *client_output_buffer_limit_pubsub)))
			if err != nil {
				log.Fatal(err)
			}
			_, err, pubsubTopology = getPubSubBufferLimit(err, conn)
			if err != nil {
				log.Fatal(err)
			}
			fmt.Println(fmt.Sprintf("\tCHANGED DB pubsub topology for address %s: %s", slot.Addr, pubsubTopology))
		} else {
			fmt.Println(fmt.Sprintf("\tNo need to change pubsub topology for address %s: %s", slot.Addr, pubsubTopology))
		}
		conn.Close()
	}
}

func getPubSubBufferLimit(err error, conn radix.Conn) ([]string, error, string) {
	var topologyResponse []string
	err = conn.Do(radix.FlatCmd(&topologyResponse, "CONFIG", "GET", "client-output-buffer-limit"))
	if err != nil {
		log.Fatal(err)
	}
	i := strings.Index(topologyResponse[1], "pubsub ")
	pubsubTopology := topologyResponse[1][i+7:]
	return topologyResponse, err, pubsubTopology
}
