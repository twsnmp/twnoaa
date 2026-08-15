package tracker

import (
	"testing"
	"time"
)

func TestParseTLE(t *testing.T) {
	line1 := "1 25338U 98030A   26227.87321262  .00000085  00000+0  65761-4 0  9993"
	line2 := "2 25338  98.7183 286.3210 0011540  89.4120 270.8120 14.25983120421832"

	sat, err := parseTLE(line1, line2)
	if err != nil {
		t.Fatalf("Failed to parse TLE: %v", err)
	}

	if sat.EpochYear != 2026 {
		t.Errorf("Expected year 2026, got %d", sat.EpochYear)
	}

	ecef := sat.propagateECEF(time.Now())
	if ecef.X == 0 && ecef.Y == 0 && ecef.Z == 0 {
		t.Errorf("Propagated position is zero vector")
	}

	obs := NewObserver(35.6895, 139.6917, 0.05) // Tokyo
	look := obs.LookAngle(ecef)
	if look.Azimuth < 0 || look.Azimuth > 360 {
		t.Errorf("Azimuth out of bounds: %f", look.Azimuth)
	}
}

func TestTargetSatellitesList(t *testing.T) {
	if len(TargetSatellites) != 3 {
		t.Fatalf("Expected 3 target satellites, got %d", len(TargetSatellites))
	}
	if TargetSatellites[0].Name != "NOAA 15" {
		t.Errorf("Expected NOAA 15 first, got %s", TargetSatellites[0].Name)
	}
}

func TestFetchAndCalculateUpcomingPasses(t *testing.T) {
	start := time.Now()
	tr := NewTracker()

	// Observer: Tokyo (35.6895° N, 139.6917° E)
	passes, err := tr.GetUpcomingPasses(35.6895, 139.6917, 0.05, 24)
	if err != nil {
		t.Fatalf("GetUpcomingPasses returned error: %v", err)
	}

	elapsed := time.Since(start)
	t.Logf("Calculated %d passes in %v", len(passes), elapsed)
	for i, p := range passes {
		t.Logf("Pass #%d: %s | AOS: %s | LOS: %s | MaxElev: %.1f° | Azimuth: %.1f°",
			i+1, p.SatName, p.AOS.Format("2006-01-02 15:04:05"), p.LOS.Format("15:04:05"), p.MaxElev, p.MaxElevAz)
	}
}
