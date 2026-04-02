package session

import "testing"

func TestDevice_Parse(t *testing.T) {
	tests := []struct {
		name     string
		ua       string
		expected Device
	}{
		{"iPhone", "Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X)", DeviceIPhone},
		{"iPad", "Mozilla/5.0 (iPad; CPU OS 17_0 like Mac OS X)", DeviceIPad},
		{"Android", "Mozilla/5.0 (Linux; Android 14; Pixel 8)", DeviceAndroid},
		{"Mac", "Mozilla/5.0 (Macintosh; Intel Mac OS X 14_0)", DeviceMac},
		{"Windows", "Mozilla/5.0 (Windows NT 10.0; Win64; x64)", DeviceWindows},
		{"Linux", "Mozilla/5.0 (X11; Linux x86_64)", DeviceLinux},
		{"unknown", "curl/8.1.2", DeviceUnknown},
		{"empty", "", DeviceUnknown},

		// Case insensitivity.
		{"iPhone lowercase", "iphone", DeviceIPhone},
		{"WINDOWS uppercase", "WINDOWS NT 10.0", DeviceWindows},

		// Priority: iPhone before Linux (iPhone UA contains "like Mac OS X").
		{"iPhone priority over Mac", "Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X)", DeviceIPhone},
		// Android contains "Linux" but should match Android first.
		{"Android priority over Linux", "Mozilla/5.0 (Linux; Android 14)", DeviceAndroid},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Device("").Parse(tt.ua)
			if got != tt.expected {
				t.Errorf("Parse(%q) = %q, want %q", tt.ua, got, tt.expected)
			}
		})
	}
}

func TestDevice_String(t *testing.T) {
	tests := []struct {
		device   Device
		expected string
	}{
		{DeviceIPhone, "iPhone"},
		{DeviceIPad, "iPad"},
		{DeviceAndroid, "Android"},
		{DeviceMac, "Mac"},
		{DeviceWindows, "Windows"},
		{DeviceLinux, "Linux"},
		{DeviceUnknown, "Unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if tt.device.String() != tt.expected {
				t.Errorf("got %q, want %q", tt.device.String(), tt.expected)
			}
		})
	}
}
