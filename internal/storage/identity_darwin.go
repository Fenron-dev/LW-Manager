//go:build darwin

package storage

import (
	"bytes"
	"encoding/xml"
	"os/exec"
	"strings"
)

func Identify(path string) (Identity, error) {
	data, err := exec.Command("/usr/sbin/diskutil", "info", "-plist", path).Output()
	if err != nil {
		return Identity{}, err
	}
	values := map[string]string{}
	decoder := xml.NewDecoder(bytes.NewReader(data))
	current := ""
	for {
		token, err := decoder.Token()
		if err != nil {
			break
		}
		start, ok := token.(xml.StartElement)
		if !ok {
			continue
		}
		switch start.Name.Local {
		case "key":
			var value string
			if decoder.DecodeElement(&value, &start) == nil {
				current = value
			}
		case "string", "integer":
			var value string
			if decoder.DecodeElement(&value, &start) == nil && current != "" {
				values[current] = value
				current = ""
			}
		case "true":
			if current != "" {
				values[current] = "true"
				current = ""
			}
		case "false":
			if current != "" {
				values[current] = "false"
				current = ""
			}
		}
	}
	deviceType := values["BusProtocol"]
	if values["SolidState"] == "true" {
		deviceType = strings.TrimSpace(deviceType + " SSD")
	}
	return Identity{UUID: values["VolumeUUID"], Label: values["VolumeName"], FSType: values["FilesystemType"], Model: values["MediaName"], DeviceType: deviceType}, nil
}
