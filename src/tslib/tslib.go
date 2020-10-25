package tslib

import (
	"fmt"
	"math"
)

const noReading = math.MinInt16

// Metric json
type Metric struct {
	Addr      string `json:"addr"`
	RSSI      int    `json:"rssi"`
	Timestamp int64  `json:"timestamp"`
	Version   uint8  `json:"version"`
	Counter   uint8  `json:"counter"`
	Type      string `json:"type"`
	Value     int16  `json:"value"`
}

// ParseValue Converts the value
func (m *Metric) ParseValue() (float64, error) {
	if m.Value == noReading {
		return 0, fmt.Errorf("No reading: %#x", m.Value)
	}
	if m.Type == "temperature" {
		if m.Version == 1 {
			return float64(m.Value) / 100.0, nil
		}
		return float64(m.Value) / 1000.0, nil
	}
	return float64(m.Value), nil
}
