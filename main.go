package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/go-ble/ble"
	"github.com/go-ble/ble/linux"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

var client mqtt.Client

var lastCount = make(map[string]uint8)

const topicPrefix = "/sensors/"

type manufData struct {
	ID          uint16
	Temperature uint16
	Voltage     uint16
	Reserved    uint8
	Counter     uint8
}

func advHandler(a ble.Advertisement) {
	addr := a.Addr().String()
	var data manufData
	buf := bytes.NewReader(a.ManufacturerData())
	binary.Read(buf, binary.LittleEndian, &data)

	count, present := lastCount[addr]
	if present && count == data.Counter {
		return
	}
	lastCount[addr] = data.Counter

	fmt.Printf("[%s] N %3d:", a.Addr(), a.RSSI())
	fmt.Printf(" Name: %s", a.LocalName())
	fmt.Printf(" MD: 0x%X ", a.ManufacturerData())
	fmt.Printf("\n")

	ts := time.Now().Unix()
	temp := float64(data.Temperature) / 1000.0
	fmt.Printf("[%d] Temperature: %.2f, Voltage: %d, Count: %d\n",
		ts, temp, data.Voltage, data.Counter)

	payload := fmt.Sprintf("%d,0,%d,%d", ts, data.Voltage, data.Counter)
	token := client.Publish(topicPrefix+addr, 1, false, payload)
	if token.Wait() && token.Error() != nil {
		fmt.Println(token.Error())
	}

	payload = fmt.Sprintf("%d,1,%.2f,%d", ts, temp, data.Counter)
	token = client.Publish(topicPrefix+addr, 1, false, payload)
	if token.Wait() && token.Error() != nil {
		fmt.Println(token.Error())
	}
}

func advFilter(a ble.Advertisement) bool {
	return a.LocalName() == "BLETempSensor"
}

func newDevice(opts ...ble.Option) (d ble.Device, err error) {
	return linux.NewDevice(opts...)
}

func main() {
	server := flag.String("server", "tcp://127.0.0.1:1883", "The full URL of the MQTT server to connect to")
	flag.Parse()

	connOpts := mqtt.NewClientOptions().AddBroker(*server)
	client = mqtt.NewClient(connOpts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		fmt.Println(token.Error())
		return
	}
	fmt.Println("Connected to broker")

	d, err := newDevice()
	if err != nil {
		log.Fatalf("can't new device : %s", err)
	}
	ble.SetDefaultDevice(d)

	fmt.Println("Scanning...")
	for {
		ctx := context.Background()

		err = ble.Scan(ctx, true, advHandler, advFilter)
		if err != nil {
			log.Fatalf(err.Error())
		}
	}
}
