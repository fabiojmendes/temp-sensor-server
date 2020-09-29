package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"log"

	"github.com/go-ble/ble"
	"github.com/go-ble/ble/linux"
)

type manufData struct {
	ID          uint16
	Temperature uint16
	Voltage     uint16
	Reserved    uint8
	Counter     uint8
}

func advHandler(a ble.Advertisement) {
	fmt.Printf("[%s] N %3d:", a.Addr(), a.RSSI())
	fmt.Printf(" Name: %s", a.LocalName())
	fmt.Printf(" MD: %X ", a.ManufacturerData())
	fmt.Printf("\n")

	var data manufData
	buf := bytes.NewReader(a.ManufacturerData())
	binary.Read(buf, binary.LittleEndian, &data)
	fmt.Printf("Temperature: %.1f, Voltage: %d, Count: %d\n",
		float64(data.Temperature)/1000.0, data.Voltage, data.Counter)
}

func advFilter(a ble.Advertisement) bool {
	return a.LocalName() == "BLETempSensor"
}

func newDevice(opts ...ble.Option) (d ble.Device, err error) {
	return linux.NewDevice(opts...)
}

func main() {
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
