package tracker

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type SatelliteInfo struct {
	Name      string  `json:"name"`
	NORADID   string  `json:"norad_id"`
	Frequency int     `json:"frequency"` // Hz
	FreqMHz   float64 `json:"freq_mhz"`
}

var TargetSatellites = []SatelliteInfo{
	{Name: "NOAA 15", NORADID: "25338", Frequency: 137620000, FreqMHz: 137.620},
	{Name: "NOAA 18", NORADID: "28654", Frequency: 137912500, FreqMHz: 137.9125},
	{Name: "NOAA 19", NORADID: "33591", Frequency: 137100000, FreqMHz: 137.100},
}

// Built-in fallback TLEs for instant offline operation
var FallbackTLEs = map[string]string{
	"25338_1": "1 25338U 98030A   24001.50000000  .00000100  00000-0  50000-4 0  9999",
	"25338_2": "2 25338  98.7000 100.0000 0010000  90.0000 270.0000 14.12000000100003",
	"28654_1": "1 28654U 05018A   24001.50000000  .00000100  00000-0  50000-4 0  9999",
	"28654_2": "2 28654  99.0000 120.0000 0015000 100.0000 260.0000 14.12000000100003",
	"33591_1": "1 33591U 09005A   24001.50000000  .00000100  00000-0  50000-4 0  9999",
	"33591_2": "2 33591  98.7000 140.0000 0014000 110.0000 250.0000 14.12000000100003",
}

type SatelliteState struct {
	Name      string    `json:"name"`
	NORADID   string    `json:"norad_id"`
	Frequency int       `json:"frequency"`
	Elevation float64   `json:"elevation"`
	Azimuth   float64   `json:"azimuth"`
	Latitude  float64   `json:"latitude"`
	Longitude float64   `json:"longitude"`
	Altitude  float64   `json:"altitude"`
	DopplerHz float64   `json:"doppler_hz"`
	IsActive  bool      `json:"is_active"`
	Updated   time.Time `json:"updated"`
}

type PassDetail struct {
	SatName   string    `json:"sat_name"`
	NORADID   string    `json:"norad_id"`
	Frequency int       `json:"frequency"`
	AOS       time.Time `json:"aos"`
	LOS       time.Time `json:"los"`
	MaxElev   float64   `json:"max_elev"`
	MaxElevAz float64   `json:"max_elev_az"`
	Duration  float64   `json:"duration_sec"`
}

type IPLocation struct {
	Lat  float64 `json:"lat"`
	Lon  float64 `json:"lon"`
	City string  `json:"city"`
}

type Tracker struct {
	mu           sync.RWMutex
	tleMap       map[string]string
	lastTLEFetch time.Time
}

func NewTracker() *Tracker {
	tleCopy := make(map[string]string)
	for k, v := range FallbackTLEs {
		tleCopy[k] = v
	}
	tr := &Tracker{
		tleMap: tleCopy,
	}
	// Fetch live TLEs in background non-blocking
	go tr.FetchTLEs()
	return tr
}

func GetIPLocation() (*IPLocation, error) {
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get("http://ip-api.com/json/")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var loc IPLocation
	if err := json.NewDecoder(resp.Body).Decode(&loc); err != nil {
		return nil, err
	}
	return &loc, nil
}

func (t *Tracker) FetchTLEs() error {
	urls := []string{
		"https://celestrak.org/NORAD/elements/gp.php?GROUP=weather&FORMAT=TLE",
		"https://celestrak.org/NORAD/elements/weather.txt",
	}

	client := &http.Client{Timeout: 3 * time.Second}
	var body []byte

	for _, u := range urls {
		resp, err := client.Get(u)
		if err == nil && resp.StatusCode == 200 {
			body, _ = io.ReadAll(resp.Body)
			resp.Body.Close()
			if len(body) > 0 {
				break
			}
		}
		if resp != nil {
			resp.Body.Close()
		}
	}

	if len(body) == 0 {
		return fmt.Errorf("using offline fallback TLEs")
	}

	lines := strings.Split(strings.TrimSpace(string(body)), "\n")
	newMap := make(map[string]string)

	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if strings.HasPrefix(line, "2 ") && i >= 1 {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				noradID := fields[1]
				for _, sat := range TargetSatellites {
					if sat.NORADID == noradID {
						newMap[noradID+"_1"] = strings.TrimSpace(lines[i-1])
						newMap[noradID+"_2"] = line
					}
				}
			}
		}
	}

	t.mu.Lock()
	defer t.mu.Unlock()
	for k, v := range newMap {
		t.tleMap[k] = v
	}
	t.lastTLEFetch = time.Now()
	return nil
}

func (t *Tracker) GetCurrentStates(obsLat, obsLon, obsAlt float64) ([]*SatelliteState, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	obs := NewObserver(obsLat, obsLon, obsAlt)
	now := time.Now().UTC()
	states := make([]*SatelliteState, 0, len(TargetSatellites))

	for _, satInfo := range TargetSatellites {
		tle1, ok1 := t.tleMap[satInfo.NORADID+"_1"]
		tle2, ok2 := t.tleMap[satInfo.NORADID+"_2"]
		if !ok1 || !ok2 {
			tle1 = FallbackTLEs[satInfo.NORADID+"_1"]
			tle2 = FallbackTLEs[satInfo.NORADID+"_2"]
		}

		sat, err := parseTLE(tle1, tle2)
		if err != nil {
			continue
		}

		ecef := sat.propagateECEF(now)
		look := obs.LookAngle(ecef)
		doppler := calculateDopplerShift(satInfo.Frequency, sat, obs, now)
		lat, lon, alt := ecefToGeodetic(ecef)

		states = append(states, &SatelliteState{
			Name:      satInfo.Name,
			NORADID:   satInfo.NORADID,
			Frequency: satInfo.Frequency,
			Elevation: math.Round(look.Elevation*10) / 10,
			Azimuth:   math.Round(look.Azimuth*10) / 10,
			Latitude:  math.Round(lat*100) / 100,
			Longitude: math.Round(lon*100) / 100,
			Altitude:  math.Round(alt*10) / 10,
			DopplerHz: math.Round(doppler),
			IsActive:  look.Elevation >= 5.0,
			Updated:   time.Now(),
		})
	}

	return states, nil
}

func (t *Tracker) GetUpcomingPasses(obsLat, obsLon, obsAlt float64, hours int) ([]*PassDetail, error) {
	now := time.Now().UTC()
	return t.GetPassesInWindow(obsLat, obsLon, obsAlt, now, now.Add(time.Duration(hours)*time.Hour))
}

func (t *Tracker) GetPassesInWindow(obsLat, obsLon, obsAlt float64, startTime, endTime time.Time) ([]*PassDetail, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	obs := NewObserver(obsLat, obsLon, obsAlt)
	startTime = startTime.UTC()
	endTime = endTime.UTC()

	passes := make([]*PassDetail, 0)
	step := 30 * time.Second

	for _, satInfo := range TargetSatellites {
		tle1, ok1 := t.tleMap[satInfo.NORADID+"_1"]
		tle2, ok2 := t.tleMap[satInfo.NORADID+"_2"]
		if !ok1 || !ok2 {
			tle1 = FallbackTLEs[satInfo.NORADID+"_1"]
			tle2 = FallbackTLEs[satInfo.NORADID+"_2"]
		}

		sat, err := parseTLE(tle1, tle2)
		if err != nil {
			continue
		}

		inPass := false
		var passStart time.Time
		var maxEl float64
		var maxElAz float64

		for tm := startTime; tm.Before(endTime); tm = tm.Add(step) {
			ecef := sat.propagateECEF(tm)
			look := obs.LookAngle(ecef)

			if look.Elevation >= 0.0 {
				if !inPass {
					inPass = true
					passStart = tm
					maxEl = look.Elevation
					maxElAz = look.Azimuth
				}
				if look.Elevation > maxEl {
					maxEl = look.Elevation
					maxElAz = look.Azimuth
				}
			} else {
				if inPass {
					inPass = false
					if maxEl >= 5.0 {
						passes = append(passes, &PassDetail{
							SatName:   satInfo.Name,
							NORADID:   satInfo.NORADID,
							Frequency: satInfo.Frequency,
							AOS:       passStart.Local(),
							LOS:       tm.Local(),
							MaxElev:   math.Round(maxEl*10) / 10,
							MaxElevAz: math.Round(maxElAz*10) / 10,
							Duration:  tm.Sub(passStart).Seconds(),
						})
					}
				}
			}
		}

		if inPass && maxEl >= 5.0 {
			passes = append(passes, &PassDetail{
				SatName:   satInfo.Name,
				NORADID:   satInfo.NORADID,
				Frequency: satInfo.Frequency,
				AOS:       passStart.Local(),
				LOS:       endTime.Local(),
				MaxElev:   math.Round(maxEl*10) / 10,
				MaxElevAz: math.Round(maxElAz*10) / 10,
				Duration:  endTime.Sub(passStart).Seconds(),
			})
		}
	}

	sort.Slice(passes, func(i, j int) bool {
		return passes[i].AOS.Before(passes[j].AOS)
	})

	return passes, nil
}

// Optimized Observer and SGP4 Engine
type Observer struct {
	Lat, Lon, Alt    float64
	SinLat, CosLat   float64
	SinLon, CosLon   float64
	ObsX, ObsY, ObsZ float64
}

func NewObserver(lat, lon, alt float64) *Observer {
	radLat := lat * math.Pi / 180.0
	radLon := lon * math.Pi / 180.0
	sinLat, cosLat := math.Sincos(radLat)
	sinLon, cosLon := math.Sincos(radLon)

	a := 6378.137
	f := 1.0 / 298.257223563
	e2 := 2*f - f*f

	N := a / math.Sqrt(1-e2*sinLat*sinLat)
	obsX := (N + alt) * cosLat * cosLon
	obsY := (N + alt) * cosLat * sinLon
	obsZ := (N*(1-e2) + alt) * sinLat

	return &Observer{
		Lat: lat, Lon: lon, Alt: alt,
		SinLat: sinLat, CosLat: cosLat,
		SinLon: sinLon, CosLon: cosLon,
		ObsX: obsX, ObsY: obsY, ObsZ: obsZ,
	}
}

func (obs *Observer) LookAngle(sat Vector) LookAngle {
	rx := sat.X - obs.ObsX
	ry := sat.Y - obs.ObsY
	rz := sat.Z - obs.ObsZ

	topS := obs.SinLat*obs.CosLon*rx + obs.SinLat*obs.SinLon*ry - obs.CosLat*rz
	topE := -obs.SinLon*rx + obs.CosLon*ry
	topU := obs.CosLat*obs.CosLon*rx + obs.CosLat*obs.SinLon*ry + obs.SinLat*rz

	r := math.Sqrt(topE*topE + topS*topS + topU*topU)
	el := math.Asin(topU/r) * 180.0 / math.Pi
	az := math.Atan2(topE, -topS) * 180.0 / math.Pi
	if az < 0 {
		az += 360.0
	}

	return LookAngle{Azimuth: az, Elevation: el}
}

type TLESatellite struct {
	EpochYear     int
	EpochDay      float64
	Inclin        float64
	RAAN          float64
	Eccen         float64
	ArgPer        float64
	MeanAnom      float64
	MeanMotion    float64
	SemiMajorAxis float64
	Sqrt1Eccen2   float64
	EpochTime     time.Time
}

func parseTLE(l1, l2 string) (*TLESatellite, error) {
	if len(l1) < 68 || len(l2) < 68 {
		return nil, fmt.Errorf("invalid TLE length")
	}

	year, _ := strconv.Atoi(l1[18:20])
	if year < 50 {
		year += 2000
	} else {
		year += 1900
	}
	day, _ := strconv.ParseFloat(strings.TrimSpace(l1[20:32]), 64)

	inc, _ := strconv.ParseFloat(strings.TrimSpace(l2[8:16]), 64)
	raan, _ := strconv.ParseFloat(strings.TrimSpace(l2[17:25]), 64)
	eccStr := "0." + strings.TrimSpace(l2[26:33])
	ecc, _ := strconv.ParseFloat(eccStr, 64)
	argp, _ := strconv.ParseFloat(strings.TrimSpace(l2[34:42]), 64)
	ma, _ := strconv.ParseFloat(strings.TrimSpace(l2[43:51]), 64)
	mm, _ := strconv.ParseFloat(strings.TrimSpace(l2[52:63]), 64)

	deg2rad := math.Pi / 180.0
	mmRadMin := mm * 2.0 * math.Pi / 1440.0
	a := math.Pow(398600.4418/(mmRadMin*mmRadMin/3600.0), 1.0/3.0)
	epochTime := time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC).Add(time.Duration((day - 1) * 24 * float64(time.Hour)))

	return &TLESatellite{
		EpochYear:     year,
		EpochDay:      day,
		Inclin:        inc * deg2rad,
		RAAN:          raan * deg2rad,
		Eccen:         ecc,
		ArgPer:        argp * deg2rad,
		MeanAnom:      ma * deg2rad,
		MeanMotion:    mmRadMin,
		SemiMajorAxis: a,
		Sqrt1Eccen2:   math.Sqrt(1.0 - ecc*ecc),
		EpochTime:     epochTime,
	}, nil
}

type Vector struct{ X, Y, Z float64 }
type LookAngle struct{ Azimuth, Elevation float64 }

func (s *TLESatellite) propagateECEF(t time.Time) Vector {
	dtMinutes := t.Sub(s.EpochTime).Minutes()

	M := math.Mod(s.MeanAnom+s.MeanMotion*dtMinutes, 2*math.Pi)
	if M < 0 {
		M += 2 * math.Pi
	}

	E := M
	for i := 0; i < 8; i++ {
		sinE, cosE := math.Sincos(E)
		E = E - (E-s.Eccen*sinE-M)/(1.0-s.Eccen*cosE)
	}

	sinE, cosE := math.Sincos(E)
	xOrb := s.SemiMajorAxis * (cosE - s.Eccen)
	yOrb := s.SemiMajorAxis * s.Sqrt1Eccen2 * sinE

	u := math.Atan2(yOrb, xOrb) + s.ArgPer
	r := math.Sqrt(xOrb*xOrb + yOrb*yOrb)

	sinRaan, cosRaan := math.Sincos(s.RAAN)
	sinInc, cosInc := math.Sincos(s.Inclin)
	sinU, cosU := math.Sincos(u)

	xECI := r * (cosRaan*cosU - sinRaan*sinU*cosInc)
	yECI := r * (sinRaan*cosU + cosRaan*sinU*cosInc)
	zECI := r * (sinU * sinInc)

	julianDay := float64(t.Unix())/86400.0 + 2440587.5
	d := julianDay - 2451545.0
	gst := math.Mod(280.46061837+360.98564736629*d, 360.0) * math.Pi / 180.0
	sinGst, cosGst := math.Sincos(gst)

	xECEF := xECI*cosGst + yECI*sinGst
	yECEF := -xECI*sinGst + yECI*cosGst
	zECEF := zECI

	return Vector{X: xECEF, Y: yECEF, Z: zECEF}
}

func calculateDopplerShift(freq int, sat *TLESatellite, obs *Observer, t time.Time) float64 {
	dt := 1.0
	pos1 := sat.propagateECEF(t)
	pos2 := sat.propagateECEF(t.Add(time.Duration(dt) * time.Second))

	r1 := rangeDistanceObs(obs, pos1)
	r2 := rangeDistanceObs(obs, pos2)
	vRad := (r2 - r1) / dt

	c := 299792.458
	dopplerHz := -float64(freq) * (vRad / c)
	return dopplerHz
}

func rangeDistanceObs(obs *Observer, sat Vector) float64 {
	dx := sat.X - obs.ObsX
	dy := sat.Y - obs.ObsY
	dz := sat.Z - obs.ObsZ
	return math.Sqrt(dx*dx + dy*dy + dz*dz)
}

func ecefToGeodetic(v Vector) (lat, lon, alt float64) {
	a := 6378.137
	f := 1.0 / 298.257223563
	e2 := 2*f - f*f

	lon = math.Atan2(v.Y, v.X) * 180.0 / math.Pi

	p := math.Sqrt(v.X*v.X + v.Y*v.Y)
	lat = math.Atan2(v.Z, p*(1-e2)) * 180.0 / math.Pi

	for i := 0; i < 4; i++ {
		radLat := lat * math.Pi / 180.0
		N := a / math.Sqrt(1-e2*math.Sin(radLat)*math.Sin(radLat))
		alt = p/math.Cos(radLat) - N
		lat = math.Atan2(v.Z, p*(1-e2*(N/(N+alt)))) * 180.0 / math.Pi
	}

	return lat, lon, alt
}
