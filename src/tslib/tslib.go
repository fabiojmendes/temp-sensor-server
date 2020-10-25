package tslib

// Metric json
type Metric struct {
	Addr        string   `json:"addr"`
	RSSI        int      `json:"rssi"`
	Timestamp   int64    `json:"timestamp"`
	Counter     uint8    `json:"counter"`
	Temperature *float64 `json:"temperature"`
	Voltage     *float64 `json:"voltage"`
}
