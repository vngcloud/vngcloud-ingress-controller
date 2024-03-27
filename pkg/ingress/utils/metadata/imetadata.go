package metadata

import (
	"time"
)

type IMetadata interface {
	GetInstanceID() (string, error)
	GetAvailabilityZone() (string, error)
	GetProjectID() (string, error)
}

type MyDuration struct {
	time.Duration
}

type (
	Opts struct {
		SearchOrder    string     `gcfg:"search-order"`    // will be configDriver
		RequestTimeout MyDuration `gcfg:"request-timeout"` // will be 0
		ChartVersion   string     `gcfg:"chartVersion"`    // will be 0
	}

	Metadata struct {
		UUID             string           `json:"uuid"`
		Name             string           `json:"name"`
		AvailabilityZone string           `json:"availability_zone"`
		ProjectID        string           `json:"project_id"`
		Devices          []DeviceMetadata `json:"devices,omitempty"`
		// ... and other fields we don't care about.  Expand as necessary.
	}

	DeviceMetadata struct {
		Type    string `json:"type"`
		Bus     string `json:"bus,omitempty"`
		Serial  string `json:"serial,omitempty"`
		Address string `json:"address,omitempty"`
		// ... and other fields.
	}
)
