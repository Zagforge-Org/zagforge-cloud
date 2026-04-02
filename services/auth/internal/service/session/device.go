package session

import "strings"

// Device represents a parsed device type from a user-agent string.
type Device string

const (
	DeviceIPhone  Device = "iPhone"
	DeviceIPad    Device = "iPad"
	DeviceAndroid Device = "Android"
	DeviceMac     Device = "Mac"
	DeviceWindows Device = "Windows"
	DeviceLinux   Device = "Linux"
	DeviceUnknown Device = "Unknown"
)

// deviceRules maps a user-agent substring to a Device.
// Order matters — first match wins.
var deviceRules = []struct {
	contains string
	device   Device
}{
	{"iphone", DeviceIPhone},
	{"ipad", DeviceIPad},
	{"android", DeviceAndroid},
	{"macintosh", DeviceMac},
	{"windows", DeviceWindows},
	{"linux", DeviceLinux},
}

// Parse returns the Device for the given user-agent string.
func (Device) Parse(ua string) Device {
	lower := strings.ToLower(ua)
	for _, rule := range deviceRules {
		if strings.Contains(lower, rule.contains) {
			return rule.device
		}
	}
	return DeviceUnknown
}

// String returns the string representation of the Device.
func (d Device) String() string {
	return string(d)
}
