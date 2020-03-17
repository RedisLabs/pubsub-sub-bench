package main

import (
	"flag"
	"fmt"
	"github.com/mediocregopher/radix"
	"log"
	"os"
	"os/signal"
	"strings"
	"sync"
)

func subscriberRoutine(addr string, channel string, printMessages bool, stop chan struct{}, wg *sync.WaitGroup) {
	// tell the caller we've stopped
	defer wg.Done()

	// Create a normal redis connection
	conn, err := radix.Dial("tcp", addr)
	defer conn.Close()
	err = conn.Do(radix.FlatCmd(nil, "CLIENT", "SETNAME", channel))
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

	for {
		select {
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

func main() {
	host := flag.String("host", "localhost", "redis host.")
	port := flag.Int("port", 6379, "redis port.")
	channel_minimum := flag.Int("channel-minimum", 1, "channel ID minimum value ( each channel has a dedicated thread ).")
	channel_maximum := flag.Int("channel-maximum", 100, "channel ID maximum value ( each channel has a dedicated thread ).")
	subscribe_prefix := flag.String("subscriber-prefix", "channel-", "prefix for subscribing to channel, used in conjunction with key-minimum and key-maximum.")
	distributeSubscribers := flag.Bool("cluster-api-distribute-subscribers", false, "read cluster slots and distribute subscribers among them.")
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

	for _, slot := range nodes {
		fmt.Println(slot)
	}

	conn.Close()
	// a channel to tell `tick()` and `tock()` to stop
	stopChan := make(chan struct{})

	// a WaitGroup for the goroutines to tell us they've stopped
	wg := sync.WaitGroup{}

	for channel_id := *channel_minimum; channel_id <= *channel_maximum; channel_id++ {
		nodes_pos := channel_id % len(nodes)
		addr := nodes[nodes_pos]
		channel := fmt.Sprintf("%s%d", *subscribe_prefix, channel_id)
		wg.Add(1)
		go subscriberRoutine(addr.Addr, channel, *printMessages, stopChan, &wg)
	}

	// listen for C-c
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	<-c
	fmt.Println("received C-c - shutting down")

	// tell the goroutine to stop
	close(stopChan)
	// and wait for them both to reply back
	wg.Wait()

}
