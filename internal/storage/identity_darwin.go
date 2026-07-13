//go:build darwin

package storage

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"os/exec"
	"strings"
	"time"
)

func Identify(path string) (Identity, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	data, err := exec.CommandContext(ctx, "/usr/sbin/diskutil", "info", "-plist", path).Output()
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
	identity := Identity{UUID: values["VolumeUUID"], Label: values["VolumeName"], FSType: values["FilesystemType"], Model: values["MediaName"], DeviceType: deviceType}
	device := values["ParentWholeDisk"]
	if device == "" {
		device = values["DeviceIdentifier"]
	}
	if device != "" {
		identity.enrichDarwinHardware(device)
	}
	return identity, nil
}

func (identity *Identity) enrichDarwinHardware(device string) {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	data, err := exec.CommandContext(ctx, "/usr/sbin/system_profiler", "SPUSBDataType", "-json", "-detailLevel", "mini").Output()
	if err != nil {
		return
	}
	var root any
	if json.Unmarshal(data, &root) != nil {
		return
	}
	if details, ok := findDarwinDevice(root, device); ok {
		identity.Serial = firstString(details, "serial_num", "serial_number")
		identity.Vendor = firstString(details, "manufacturer", "vendor_id")
		if model := firstString(details, "_name", "model"); model != "" {
			identity.Model = model
		}
	}
}

func findDarwinDevice(value any, device string) (map[string]any, bool) {
	switch item := value.(type) {
	case map[string]any:
		matched := firstString(item, "bsd_name", "device_identifier") == device
		for _, child := range item {
			if details, ok := findDarwinDevice(child, device); ok {
				for key, candidate := range item {
					if _, exists := details[key]; !exists {
						details[key] = candidate
					}
				}
				return details, true
			}
		}
		return item, matched
	case []any:
		for _, child := range item {
			if details, ok := findDarwinDevice(child, device); ok {
				return details, true
			}
		}
	}
	return nil, false
}

func firstString(values map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := values[key].(string); ok && strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
