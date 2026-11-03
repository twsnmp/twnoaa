package main

import "testing"

func TestIsSatMatchISSRecordMode(t *testing.T) {
	tests := []struct {
		name          string
		satName       string
		issRecordMode string
		expected      bool
	}{
		// Non-ISS satellites should always match regardless of mode
		{"Non-ISS METEOR-M2 3", "METEOR-M2 3", "aprs", true},
		{"Non-ISS METEOR-M2 4", "METEOR-M2 4", "off", true},
		{"Non-ISS SONATE-2", "SONATE-2 (SSTV)", "off", true},
		{"Non-ISS NOAA 15", "NOAA 15", "voice", true},

		// Mode: "off" should filter out all ISS modes
		{"ISS APRS UHF when off", "ISS (APRS UHF)", "off", false},
		{"ISS APRS VHF when off", "ISS (APRS VHF)", "off", false},
		{"ISS Voice when off", "ISS (FM Voice)", "off", false},
		{"ISS SSTV VHF when off", "ISS (SSTV VHF)", "off", false},
		{"ISS SSTV UHF when off", "ISS (SSTV UHF)", "off", false},

		// Mode: "aprs" / "aprs_uhf"
		{"ISS APRS UHF when aprs", "ISS (APRS UHF)", "aprs", true},
		{"ISS APRS VHF when aprs", "ISS (APRS VHF)", "aprs", false},
		{"ISS Voice when aprs", "ISS (FM Voice)", "aprs", false},
		{"ISS SSTV VHF when aprs", "ISS (SSTV VHF)", "aprs", false},
		{"ISS SSTV UHF when aprs", "ISS (SSTV UHF)", "aprs", false},

		// Mode: "aprs_vhf"
		{"ISS APRS VHF when aprs_vhf", "ISS (APRS VHF)", "aprs_vhf", true},
		{"ISS APRS UHF when aprs_vhf", "ISS (APRS UHF)", "aprs_vhf", false},

		// Mode: "voice"
		{"ISS Voice when voice", "ISS (FM Voice)", "voice", true},
		{"ISS APRS UHF when voice", "ISS (APRS UHF)", "voice", false},

		// Mode: "sstv_vhf"
		{"ISS SSTV VHF when sstv_vhf", "ISS (SSTV VHF)", "sstv_vhf", true},
		{"ISS SSTV UHF when sstv_vhf", "ISS (SSTV UHF)", "sstv_vhf", false},

		// Mode: "sstv_uhf"
		{"ISS SSTV UHF when sstv_uhf", "ISS (SSTV UHF)", "sstv_uhf", true},
		{"ISS SSTV VHF when sstv_uhf", "ISS (SSTV VHF)", "sstv_uhf", false},

		// Default mode (empty or invalid) should default to APRS UHF
		{"ISS APRS UHF on empty mode", "ISS (APRS UHF)", "", true},
		{"ISS Voice on empty mode", "ISS (FM Voice)", "", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			actual := isSatMatchISSRecordMode(tc.satName, tc.issRecordMode)
			if actual != tc.expected {
				t.Errorf("isSatMatchISSRecordMode(%q, %q) = %v; want %v", tc.satName, tc.issRecordMode, actual, tc.expected)
			}
		})
	}
}
