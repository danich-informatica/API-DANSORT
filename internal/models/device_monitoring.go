package models

import "time"

// DeviceType representa el tipo de dispositivo
type DeviceType string

const (
	DeviceTypePLC    DeviceType = "PLC"
	DeviceTypeCognex DeviceType = "Cognex"
)

// DeviceStatus representa el estado de un dispositivo
type DeviceStatus struct {
	ID                int        `json:"id"`
	DeviceName        string     `json:"device_name"`
	DeviceType        DeviceType `json:"device_type"`
	IP                string     `json:"ip"`
	Port              int        `json:"port"`
	IsDisconnected    bool       `json:"is_disconnected"`
	LastDisconnection *time.Time `json:"last_disconnection"`
	LastCheck         time.Time  `json:"last_check"`
	SectionID         int        `json:"section_id"`
	ResponseTimeMs    int64      `json:"response_time_ms"`
}

// SectionStatus representa el estado de una sección (sorter)
type SectionStatus struct {
	SectionID         int    `json:"section_id"`
	SectionName       string `json:"section_name"`
	HasConnection     bool   `json:"has_connection"` // false si algún dispositivo está desconectado
	DeviceCount       int    `json:"device_count"`
	DisconnectedCount int    `json:"disconnected_count"`
}
