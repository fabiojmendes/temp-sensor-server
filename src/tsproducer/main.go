package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math"
	"os"
	"time"

	"github.com/go-ble/ble"
	"github.com/go-ble/ble/linux"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/fabiojmendes/temp-sensor-scanner/src/tslib"
)

const noReading = math.MinInt16

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

func (d *manufData) convertTemperature() *float64 {
	if d.Temperature == noReading {
		return nil
	}

	t := float64(d.Temperature)
	if d.Version == 1 {
		t /= 100.0
	} else {
		t /= 1000.0
	}
	return &t
}

func (d *manufData) convertVoltage() *float64 {
	if d.Voltage == noReading {
		return nil
	}
	v := float64(d.Voltage)
	return &v
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

func startPublisher(client mqtt.Client, topic string) {
	for a := range advertisingChannel {

		metric := tslib.Metric{
			Addr:        a.Addr,
			RSSI:        a.RSSI,
			Timestamp:   a.Timestamp,
			Counter:     a.Counter,
			Voltage:     a.convertVoltage(),
			Temperature: a.convertTemperature(),
		}
		payload, err := json.Marshal(metric)
		if err != nil {
			log.Println("Error converting json", err)
			continue
		}
		log.Printf("[%s] Publishing metric %v", a.Addr, string(payload))
		tokenChannel <- client.Publish(topic, 1, false, payload)
	}
}

func advHandler(a ble.Advertisement) {
	var data manufData
	buf := bytes.NewReader(a.ManufacturerData())
	if err := binary.Read(buf, binary.LittleEndian, &data); err != nil {
		log.Println("Error parsing binary data:", err.Error())
		return
	}

	addr := a.Addr().String()
	count, present := lastCount[addr]
	if present && count == data.Counter {
		return
	}
	lastCount[addr] = data.Counter

	log.Printf("[%s] N %3d: Name: %s MD: %#x",
		a.Addr(), a.RSSI(), a.LocalName(), a.ManufacturerData())
	log.Printf("[%s] RAW: Temperature: %d, Voltage: %d, Count: %d\n",
		a.Addr(), data.Temperature, data.Voltage, data.Counter)

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
	topic := flag.String("topic", "", "Name of the topic to send data to")
	username := os.Getenv("MQTT_USERNAME")
	password := os.Getenv("MQTT_PASSWORD")
	flag.Parse()

	if *topic == "" {
		fmt.Fprintln(os.Stderr, "Error: A destination topic is required")
		flag.PrintDefaults()
		os.Exit(1)
	}

	connOpts := mqtt.NewClientOptions().
		AddBroker(*server).
		SetUsername(username).
		SetPassword(password)

	client := mqtt.NewClient(connOpts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		log.Fatalln("Error connecting to the broker:", token.Error())
	}
	log.Println("Connected to broker")

	go tokenHandler()
	go startPublisher(client, *topic)

	d, err := linux.NewDevice()
	if err != nil {
		log.Fatalln("Can't create new device:", err)
	}
	ble.SetDefaultDevice(d)

	log.Println("Scanning...")
	ctx := context.Background()

	err = ble.Scan(ctx, true, advHandler, advFilter)
	if err != nil {
		log.Fatalln("Error during the BLE Scan:", err.Error())
	}
}
