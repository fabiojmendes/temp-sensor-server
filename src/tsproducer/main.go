package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"flag"
	"log"
	"os"
	"time"

	"github.com/go-ble/ble"
	"github.com/go-ble/ble/linux"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/fabiojmendes/temp-sensor-scanner/src/tslib"
)

var lastCount = make(map[string]uint8)

var advertisingChannel = make(chan parsedAdv, 1024)

var tokenChannel = make(chan mqtt.Token, 1024)

type manufData struct {
	ID          uint16
	Temperature int16
	Voltage     int16
	Version     uint8
	Counter     uint8
}

type parsedAdv struct {
	Addr      string
	RSSI      int
	Timestamp int64
	manufData
}

func tokenHandler() {
	for tkn := range tokenChannel {
		if tkn.WaitTimeout(15 * time.Second) {
			if tkn.Error() != nil {
				log.Println(tkn.Error())
			}
		} else {
			log.Println("Token timed out")
		}
	}
}

func startPublisher(client mqtt.Client) {
	const topic = "/sensor/json"

	for a := range advertisingChannel {
		log.Println("Publishing adv data", a)

		volt := tslib.Metric{
			Addr:      a.Addr,
			RSSI:      a.RSSI,
			Timestamp: a.Timestamp,
			Version:   a.Version,
			Counter:   a.Counter,
			Type:      "voltage",
			Value:     a.Voltage,
		}
		payload, _ := json.Marshal(volt)
		tokenChannel <- client.Publish(topic, 1, false, payload)

		temp := tslib.Metric{
			Addr:      a.Addr,
			RSSI:      a.RSSI,
			Timestamp: a.Timestamp,
			Version:   a.Version,
			Counter:   a.Counter,
			Type:      "temperature",
			Value:     a.Temperature,
		}
		payload, _ = json.Marshal(temp)
		tokenChannel <- client.Publish(topic, 1, false, payload)
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
	log.Printf("Temperature: %d, Voltage: %d, Count: %d\n",
		data.Temperature, data.Voltage, data.Counter)

	advertisingChannel <- parsedAdv{
		addr,
		a.RSSI(),
		time.Now().Unix(),
		data,
	}
}

func advFilter(a ble.Advertisement) bool {
	return a.LocalName() == "BLETempSensor"
}

func main() {
	server := flag.String("server", "tcp://127.0.0.1:1883",
		"The full URL of the MQTT server to connect to")
	username := os.Getenv("MQTT_USERNAME")
	password := os.Getenv("MQTT_PASSWORD")
	flag.Parse()

	connOpts := mqtt.NewClientOptions().
		AddBroker(*server).
		SetUsername(username).
		SetPassword(password)

	client := mqtt.NewClient(connOpts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		log.Fatal("Error connecting to the broker", token.Error())
	}
	log.Println("Connected to broker")

	go tokenHandler()
	go startPublisher(client)

	d, err := linux.NewDevice()
	if err != nil {
		log.Fatal("Can't create new device: ", err)
	}
	ble.SetDefaultDevice(d)

	log.Println("Scanning...")
	ctx := context.Background()

	err = ble.Scan(ctx, true, advHandler, advFilter)
	if err != nil {
		log.Fatal("Error during the BLE Scan: ", err.Error())
	}
}
