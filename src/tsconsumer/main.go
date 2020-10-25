package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/go-redis/redis"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	influxdb2Api "github.com/influxdata/influxdb-client-go/v2/api"

	"github.com/fabiojmendes/temp-sensor-scanner/src/tslib"
)

const keyPrefix = "device_name.mac."

var redisClient *redis.Client

var influxAPI influxdb2Api.WriteAPI

func lookupName(sender string) string {
	name, err := redisClient.Get(keyPrefix + sender).Result()
	if err != nil {
		log.Println("Key not found", err.Error())
		return sender
	}
	return strings.ReplaceAll(name, " ", "\\ ")
}

func handleMessage(client mqtt.Client, msg mqtt.Message) {
	log.Println("Message:", msg.Topic(), string(msg.Payload()))
	var metric tslib.Metric
	json.Unmarshal(msg.Payload(), &metric)
	value, err := metric.ParseValue()
	if err != nil {
		log.Println("Error parsing value:", err)
		return
	}
	name := lookupName(metric.Addr)

	line := fmt.Sprintf("%s,sender=%s,name=%s,boot=%d value=%.2f %d",
		metric.Type, metric.Addr, name, metric.Counter, value, metric.Timestamp)

	influxAPI.WriteRecord(line)
}

func main() {
	waitSignal := make(chan os.Signal, 1)
	signal.Notify(waitSignal, os.Interrupt, syscall.SIGTERM)

	mqttServer := flag.String("mqtt", "tcp://127.0.0.1:1883",
		"The full URL of the MQTT server to connect to")
	mqttCleanSession := flag.Bool("mqtt-clean", false,
		"Use a clean session for this consumer")
	influxServer := flag.String("influx", "http://127.0.0.1:8086",
		"The full URL of the InfluxDB server to connect to")
	redisServer := flag.String("redis", "localhost:6379",
		"The full URL of the InfluxDB server to connect to")
	flag.Parse()

	mqttUser := os.Getenv("MQTT_USERNAME")
	mqttPass := os.Getenv("MQTT_PASSWORD")

	redisClient = redis.NewClient(&redis.Options{
		Addr:     *redisServer,
		Password: "", // no password set
		DB:       0,  // use default DB
	})
	defer redisClient.Close()

	opts := influxdb2.DefaultOptions()
	opts.WriteOptions().
		SetPrecision(time.Second)
	influx := influxdb2.NewClientWithOptions(*influxServer, "", opts)
	defer influx.Close()
	log.Println("Checking for influxdb availability...")
	influxReady, err := influx.Ready(context.Background())
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

	if token := client.Subscribe("/sensor/json", 1, handleMessage); token.Wait() {
		if token.Error() != nil {
			log.Fatal(token.Error())
		}
	}
	<-waitSignal
	log.Println("Exiting...")
}
