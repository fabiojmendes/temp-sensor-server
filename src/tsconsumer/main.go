package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"gopkg.in/yaml.v2"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	influxdb2Api "github.com/influxdata/influxdb-client-go/v2/api"
	"github.com/influxdata/influxdb-client-go/v2/api/write"

	"github.com/fabiojmendes/temp-sensor-scanner/src/tslib"
)

var influxAPI influxdb2Api.WriteAPI

var tagData = make(map[string]map[string]string)

func loadTagData(filename string) error {
	bytes, err := ioutil.ReadFile(filename)
	if err != nil {
		return err
	}
	err = yaml.Unmarshal(bytes, &tagData)
	if err != nil {
		return err
	}
	return nil
}

func lookupTags(sender string) map[string]string {
	tags := make(map[string]string)
	for k, v := range tagData[sender] {
		tags[k] = v
	}
	return tags
}

func createPoint(measurement string, value interface{}, ts int64, tags map[string]string) *write.Point {

	p := write.NewPointWithMeasurement(measurement).
		AddField("value", value).
		SetTime(time.Unix(ts, 0))

	for k, v := range tags {
		p.AddTag(k, v)
	}
	return p
}

func handleMessage(client mqtt.Client, msg mqtt.Message) {
	log.Println("Message:", msg.Topic(), string(msg.Payload()))
	var metric tslib.Metric
	if err := json.Unmarshal(msg.Payload(), &metric); err != nil {
		log.Println("Error parsing json", err)
		return
	}
	tags := lookupTags(metric.Addr)
	tags["sender"] = metric.Addr
	tags["boot"] = fmt.Sprintf("%d", metric.Counter)

	if metric.Temperature != nil {
		p := createPoint("temperature", *metric.Temperature, metric.Timestamp, tags)
		influxAPI.WritePoint(p)
	}

	if metric.Voltage != nil {
		p := createPoint("voltage", *metric.Voltage, metric.Timestamp, tags)
		influxAPI.WritePoint(p)
	}

	p := createPoint("rssi", float64(metric.RSSI), metric.Timestamp, tags)
	influxAPI.WritePoint(p)
}

func main() {
	waitSignal := make(chan os.Signal, 1)
	signal.Notify(waitSignal, os.Interrupt, syscall.SIGTERM)

	mqttServer := flag.String("mqtt", "tcp://127.0.0.1:1883",
		"The full URL of the MQTT server to connect to")
	mqttTopic := flag.String("topic", "",
		"MQTT Topic to subscribe")
	mqttCleanSession := flag.Bool("mqtt-clean", false,
		"Use a clean session for this consumer")
	influxServer := flag.String("influx", "http://127.0.0.1:8086",
		"The full URL of the InfluxDB server to connect to")
	tagFile := flag.String("tags", "",
		"The full URL of the InfluxDB server to connect to")
	flag.Parse()

	mqttUser := os.Getenv("MQTT_USERNAME")
	mqttPass := os.Getenv("MQTT_PASSWORD")

	if *mqttTopic == "" {
		fmt.Fprintln(os.Stderr, "Error: A topic to subscribe is required")
		flag.PrintDefaults()
		os.Exit(1)
	}

	log.Println("Loading tag data")
	err := loadTagData(*tagFile)
	if err != nil {
		log.Fatalln("Error loading tag file", err)
	}

	influx := influxdb2.NewClient(*influxServer, "")
	defer influx.Close()
	log.Println("Checking for influxdb availability...")
	influxReady, err := influx.Ping(context.Background())
	if !influxReady || err != nil {
		log.Fatal("InfluxDB is not ready! ", err)
	}
	log.Printf("Influx is ready? %v\n", influxReady)

	influxAPI = influx.WriteAPI("", "sensor_data")

	connOpts := mqtt.NewClientOptions().
		AddBroker(*mqttServer).
		SetCleanSession(*mqttCleanSession).
		SetClientID(mqttUser).
		SetUsername(mqttUser).
		SetPassword(mqttPass)
	client := mqtt.NewClient(connOpts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		log.Fatal(token.Error())
	}
	defer client.Disconnect(1000)
	log.Println("Connected to the mqtt broker")

	if token := client.Subscribe(*mqttTopic, 1, handleMessage); token.Wait() {
		if token.Error() != nil {
			log.Fatal(token.Error())
		}
	}
	<-waitSignal
	log.Println("Exiting...")
}
