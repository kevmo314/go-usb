//go:build darwin

package usb

type devicePathTestCase struct {
	path  string
	valid bool
}

func getDevicePathTestCases() []devicePathTestCase {
	return []devicePathTestCase{
		{"iokit:1a2b3c4d", true},
		{"iokit:DEADBEEF", true},
		{"iokit:0", true},
		{"iokit:", false},
		{"iokit:notahex", false},
		{"/dev/bus/usb/001/001", false},
		{"", false},
	}
}
