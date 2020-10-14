package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/go-ble/ble"
	"github.com/go-ble/ble/linux"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

var lastCount = make(map[string]uint8)

var advertisingChannel = make(chan parsedAdv, 1024)

type manufData struct {
	ID          uint16
	Temperature int16
	Voltage     uint16
	Reserved    uint8
	Counter     uint8
}

func (d *manufData) tempToFloat() float64 {
	return float64(d.Temperature) / 1000.0
}

type parsedAdv struct {
	addr string
	data manufData
}

func startPublisher(client mqtt.Client) {
	const topicPrefix = "/sensors/"

	for adv := range advertisingChannel {
		data := adv.data
		log.Println("Publishing adv data", adv)
		ts := time.Now().Unix()
		payload := fmt.Sprintf("%d,0,%d,%d", ts, data.Voltage, data.Counter)
		tkn := client.Publish(topicPrefix+adv.addr, 1, false, payload)
		if tkn.WaitTimeout(15*time.Second) && tkn.Error() != nil {
			log.Println(tkn.Error())
		}

		payload = fmt.Sprintf("%d,1,%.2f,%d", ts, data.tempToFloat(), data.Counter)
		tkn = client.Publish(topicPrefix+adv.addr, 1, false, payload)
		if tkn.WaitTimeout(15*time.Second) && tkn.Error() != nil {
			log.Println(tkn.Error())
		}
	}
}

func advHandler(a ble.Advertisement) {
	var data manufData
	buf := bytes.NewReader(a.ManufacturerData())
	if err := binary.Read(buf, binary.LittleEndian, &data); err != nil {
		log.Println("Error parsing data:", err.Error())
		return
	}

	addr := a.Addr().String()
	count, present := lastCount[addr]
	if present && count == data.Counter {
		return
	}
	lastCount[addr] = data.Counter

	log.Printf("[%s] N %3d: Name: %s MD: 0x%X",
		a.Addr(), a.RSSI(), a.LocalName(), a.ManufacturerData())
	log.Printf("Temperature: %.2f, Voltage: %d, Count: %d\n",
		data.tempToFloat(), data.Voltage, data.Counter)

	advertisingChannel <- parsedAdv{addr, data}
}

func advFilter(a ble.Advertisement) bool {
	return a.LocalName() == "BLETempSensor"
}

func main() {
	server := flag.String("server", "tcp://127.0.0.1:1883",
		"The full URL of the MQTT server to connect to")
	username := os.Getenv("USERNAME")
	password := os.Getenv("PASSWORD")
	flag.Parse()

	connOpts := mqtt.NewClientOptions().
		AddBroker(*server).
		SetUsername(username).
		SetPassword(password)

	client := mqtt.NewClient(connOpts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		log.Println(token.Error())
		return
	}
	log.Println("Connected to broker")
	go startPublisher(client)

	d, err := linux.NewDevice()
	if err != nil {
		log.Fatalf("can't new device : %s", err)
	}
	ble.SetDefaultDevice(d)

	log.Println("Scanning...")
	ctx := context.Background()

	err = ble.Scan(ctx, true, advHandler, advFilter)
	if err != nil {
		log.Fatalf(err.Error())
	}
}
