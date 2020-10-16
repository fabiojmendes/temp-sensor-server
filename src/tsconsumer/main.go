package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path"
	"strings"
	"syscall"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/go-redis/redis"

	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	influxdb2Api "github.com/influxdata/influxdb-client-go/v2/api"
)

const keyPrefix = "device_name.mac."

var types = map[string]string{
	"0": "voltage",
	"1": "temperature",
	"2": "humidity",
}

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
	topic := msg.Topic()
	sender := path.Base(topic)
	name := lookupName(sender)
	payload := string(msg.Payload())
	p := strings.Split(payload, ",")
	ts, typ, val, count := p[0], p[1], p[2], p[3]

	line := fmt.Sprintf("%s,sender=%s,name=%s,boot=%s value=%s %s",
		types[typ], sender, name, count, val, ts)

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

	if token := client.Subscribe("/sensors/#", 1, handleMessage); token.Wait() {
		if token.Error() != nil {
			log.Fatal(token.Error())
		}
	}

	<-waitSignal
	log.Println("Exiting...")
}
