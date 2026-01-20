//go:build linux

package usb

type devicePathTestCase struct {
	path  string
	valid bool
}

func getDevicePathTestCases() []devicePathTestCase {
	return []devicePathTestCase{
		{"/dev/bus/usb/001/001", true},
		{"/dev/bus/usb/255/255", true},
		{"/dev/bus/usb/001/256", false},
		{"/dev/bus/usb/256/001", false},
		{"/dev/bus/usb/001", false},
		{"/dev/bus/usb/", false},
		{"/dev/bus/001/001", false},
		{"/tmp/001/001", false},
		{"", false},
	}
}
