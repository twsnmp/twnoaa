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
	Name        string  `json:"name"`
	NORADID     string  `json:"norad_id"`
	Frequency   int     `json:"frequency"` // Hz
	FreqMHz     float64 `json:"freq_mhz"`
	Mode        string  `json:"mode"`         // "LRPT" or "APT"
	Operational bool    `json:"operational"`  // true if satellite transmitter is active
}

var TargetSatellites = []SatelliteInfo{
	{Name: "METEOR-M2 3", NORADID: "57166", Frequency: 137900000, FreqMHz: 137.900, Mode: "LRPT", Operational: true},
	{Name: "METEOR-M2 4", NORADID: "59051", Frequency: 137900000, FreqMHz: 137.900, Mode: "LRPT", Operational: true},
	{Name: "ISS (SSTV)", NORADID: "25544", Frequency: 145800000, FreqMHz: 145.800, Mode: "SSTV", Operational: true},
	{Name: "SONATE-2 (SSTV)", NORADID: "59112", Frequency: 145880000, FreqMHz: 145.880, Mode: "SSTV", Operational: true},
	{Name: "NOAA 15", NORADID: "25338", Frequency: 137620000, FreqMHz: 137.620, Mode: "APT", Operational: false},
	{Name: "NOAA 18", NORADID: "28654", Frequency: 137912500, FreqMHz: 137.9125, Mode: "APT", Operational: false},
	{Name: "NOAA 19", NORADID: "33591", Frequency: 137100000, FreqMHz: 137.100, Mode: "APT", Operational: false},
}

// Built-in fallback TLEs
var FallbackTLEs = map[string]string{
	"57166_1": "1 57166U 23091A   26294.28915516 -.00000009  00000+0  14724-4 0  9991",
	"57166_2": "2 57166  98.5937 346.5899 0004466 150.2252 209.9182 14.24054645172465",
	"59051_1": "1 59051U 24039A   26294.30610602  .00000021  00000+0  29259-4 0  9992",
	"59051_2": "2 59051  98.7193 252.1149 0007319 156.7347 203.4163 14.22440844137199",
	"25544_1": "1 25544U 98067A   26294.04791053  .00009575  00000+0  18188-3 0  9996",
	"25544_2": "2 25544  51.6327  37.2623 0006759 289.5321  70.4938 15.48967427591263",
	"59112_1": "1 59112U 24043Q   26293.43366991  .00067999  00000+0  56417-3 0  9999",
	"59112_2": "2 59112  97.5637  92.6151 0004604  94.8533 265.3254 15.69986732147462",
	"25338_1": "1 25338U 98030A   26227.87321262  .00000085  00000+0  65761-4 0  9993",
	"25338_2": "2 25338  98.7183 286.3210 0011540  89.4120 270.8120 14.25983120421832",
	"28654_1": "1 28654U 05018A   26227.85412093  .00000092  00000+0  78210-4 0  9997",
	"28654_2": "2 28654  98.9812  45.1923 0013890 148.2019 212.0192 14.12591029082918",
	"33591_1": "1 33591U 09005A   26227.89102918  .00000105  00000+0  89120-4 0  9991",
	"33591_2": "2 33591  98.7012 312.8912 0014200  78.9102 281.2019 14.11920198890123",
}

type SatelliteState struct {
	Name        string    `json:"name"`
	NORADID     string    `json:"norad_id"`
	Frequency   int       `json:"frequency"`
	Mode        string    `json:"mode"`
	Operational bool      `json:"operational"`
	Elevation   float64   `json:"elevation"`
	Azimuth     float64   `json:"azimuth"`
	Latitude    float64   `json:"latitude"`
	Longitude   float64   `json:"longitude"`
	Altitude    float64   `json:"altitude"`
	DopplerHz   float64   `json:"doppler_hz"`
	IsActive    bool      `json:"is_active"`
	Updated     time.Time `json:"updated"`
}

type PassDetail struct {
	SatName     string    `json:"sat_name"`
	NORADID     string    `json:"norad_id"`
	Frequency   int       `json:"frequency"`
	Mode        string    `json:"mode"`
	Operational bool      `json:"operational"`
	AOS         time.Time `json:"aos"`
	LOS         time.Time `json:"los"`
	MaxElev     float64   `json:"max_elev"`
	MaxElevAz   float64   `json:"max_elev_az"`
	Duration    float64   `json:"duration_sec"`
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
		"https://celestrak.org/NORAD/elements/gp.php?GROUP=amateur&FORMAT=TLE",
		"https://celestrak.org/NORAD/elements/gp.php?GROUP=stations&FORMAT=TLE",
	}

	client := &http.Client{Timeout: 5 * time.Second}
	var allLines []string

	for _, u := range urls {
		resp, err := client.Get(u)
		if err == nil && resp.StatusCode == 200 {
			b, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			if len(b) > 0 {
				allLines = append(allLines, strings.Split(strings.TrimSpace(string(b)), "\n")...)
			}
		} else if resp != nil {
			resp.Body.Close()
		}
	}

	if len(allLines) == 0 {
		return fmt.Errorf("using offline fallback TLEs")
	}

	lines := allLines
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

	if len(newMap) > 0 {
		t.mu.Lock()
		for k, v := range newMap {
			t.tleMap[k] = v
		}
		t.lastTLEFetch = time.Now()
		t.mu.Unlock()
	}
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
			Name:        satInfo.Name,
			NORADID:     satInfo.NORADID,
			Frequency:   satInfo.Frequency,
			Mode:        satInfo.Mode,
			Operational: satInfo.Operational,
			Elevation:   math.Round(look.Elevation*10) / 10,
			Azimuth:     math.Round(look.Azimuth*10) / 10,
			Latitude:    math.Round(lat*100) / 100,
			Longitude:   math.Round(lon*100) / 100,
			Altitude:    math.Round(alt*10) / 10,
			DopplerHz:   math.Round(doppler),
			IsActive:    look.Elevation >= 5.0 && satInfo.Operational,
			Updated:     time.Now(),
		})
	}

	return states, nil
}

// GetRemainingPassDuration calculates how many seconds remain before the satellite drops below minElevation
func (t *Tracker) GetRemainingPassDuration(satName string, obsLat, obsLon, obsAlt float64, minElevation float64) float64 {
	t.mu.RLock()
	defer t.mu.RUnlock()

	obs := NewObserver(obsLat, obsLon, obsAlt)
	now := time.Now().UTC()

	var targetSat *SatelliteInfo
	for _, sat := range TargetSatellites {
		if sat.Name == satName {
			targetSat = &sat
			break
		}
	}
	if targetSat == nil {
		return 0
	}

	tle1, ok1 := t.tleMap[targetSat.NORADID+"_1"]
	tle2, ok2 := t.tleMap[targetSat.NORADID+"_2"]
	if !ok1 || !ok2 {
		tle1 = FallbackTLEs[targetSat.NORADID+"_1"]
		tle2 = FallbackTLEs[targetSat.NORADID+"_2"]
	}

	sat, err := parseTLE(tle1, tle2)
	if err != nil {
		return 0
	}

	// Check current elevation first
	currEcef := sat.propagateECEF(now)
	currLook := obs.LookAngle(currEcef)
	if currLook.Elevation < minElevation {
		return 0
	}

	// Step forward by 10s up to 25 minutes (1500s) to find when it drops below minElevation
	step := 10 * time.Second
	for offset := step; offset <= 25*time.Minute; offset += step {
		tm := now.Add(offset)
		ecef := sat.propagateECEF(tm)
		look := obs.LookAngle(ecef)
		if look.Elevation < minElevation {
			return offset.Seconds()
		}
	}
	return 25 * 60
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
	step := 15 * time.Second

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
					if maxEl >= 5.0 && satInfo.Operational {
						passes = append(passes, &PassDetail{
							SatName:     satInfo.Name,
							NORADID:     satInfo.NORADID,
							Frequency:   satInfo.Frequency,
							Mode:        satInfo.Mode,
							Operational: satInfo.Operational,
							AOS:         passStart.Local(),
							LOS:         tm.Local(),
							MaxElev:     math.Round(maxEl*10) / 10,
							MaxElevAz:   math.Round(maxElAz*10) / 10,
							Duration:    tm.Sub(passStart).Seconds(),
						})
					}
				}
			}
		}

		if inPass && maxEl >= 5.0 && satInfo.Operational {
			passes = append(passes, &PassDetail{
				SatName:     satInfo.Name,
				NORADID:     satInfo.NORADID,
				Frequency:   satInfo.Frequency,
				Mode:        satInfo.Mode,
				Operational: satInfo.Operational,
				AOS:         passStart.Local(),
				LOS:         endTime.Local(),
				MaxElev:     math.Round(maxEl*10) / 10,
				MaxElevAz:   math.Round(maxElAz*10) / 10,
				Duration:    endTime.Sub(passStart).Seconds(),
			})
		}
	}

	sort.Slice(passes, func(i, j int) bool {
		return passes[i].AOS.Before(passes[j].AOS)
	})

	return passes, nil
}

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

	altKm := alt
	if alt > 100.0 {
		altKm = alt / 1000.0
	}

	N := a / math.Sqrt(1.0-e2*sinLat*sinLat)
	obsX := (N + altKm) * cosLat * cosLon
	obsY := (N + altKm) * cosLat * sinLon
	obsZ := (N*(1.0-e2) + altKm) * sinLat

	return &Observer{
		Lat: lat, Lon: lon, Alt: altKm,
		SinLat: sinLat, CosLat: cosLat,
		SinLon: sinLon, CosLon: cosLon,
		ObsX: obsX, ObsY: obsY, ObsZ: obsZ,
	}
}

type LookAngle struct {
	Azimuth   float64
	Elevation float64
	Range     float64
}

func (obs *Observer) LookAngle(sat Vector) LookAngle {
	dx := sat.X - obs.ObsX
	dy := sat.Y - obs.ObsY
	dz := sat.Z - obs.ObsZ

	east := -obs.SinLon*dx + obs.CosLon*dy
	north := -obs.SinLat*obs.CosLon*dx - obs.SinLat*obs.SinLon*dy + obs.CosLat*dz
	up := obs.CosLat*obs.CosLon*dx + obs.CosLat*obs.SinLon*dy + obs.SinLat*dz

	r := math.Sqrt(east*east + north*north + up*up)
	el := math.Asin(up/r) * 180.0 / math.Pi
	az := math.Atan2(east, north) * 180.0 / math.Pi
	if az < 0 {
		az += 360.0
	}

	return LookAngle{Azimuth: az, Elevation: el, Range: r}
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
	BStar         float64
	SemiMajorAxis float64
	EpochTime     time.Time
}

func parseTLE(l1, l2 string) (*TLESatellite, error) {
	if len(l1) < 68 || len(l2) < 68 {
		return nil, fmt.Errorf("invalid TLE length")
	}

	year, _ := strconv.Atoi(strings.TrimSpace(l1[18:20]))
	if year < 57 {
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
	mu := 398600.4418
	a := math.Pow(mu/(mmRadMin*mmRadMin/3600.0), 1.0/3.0)

	epochStart := time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC)
	epochTime := epochStart.Add(time.Duration((day - 1.0) * 86400.0 * float64(time.Second)))

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
		EpochTime:     epochTime,
	}, nil
}

type Vector struct{ X, Y, Z float64 }

func (s *TLESatellite) propagateECEF(t time.Time) Vector {
	tUTC := t.UTC()
	dtMinutes := tUTC.Sub(s.EpochTime).Minutes()

	j2 := 0.00108263
	ae := 6378.137
	cosInc := math.Cos(s.Inclin)
	n0 := s.MeanMotion
	a0 := s.SemiMajorAxis
	ecc := s.Eccen
	p0 := a0 * (1.0 - ecc*ecc)

	dNode := -1.5 * j2 * math.Pow(ae/p0, 2) * n0 * cosInc
	dOmega := 0.75 * j2 * math.Pow(ae/p0, 2) * n0 * (5.0*cosInc*cosInc - 1.0)
	dM := n0 + 0.75*j2*math.Pow(ae/p0, 2)*n0*math.Sqrt(1.0-ecc*ecc)*(3.0*cosInc*cosInc-1.0)

	node := math.Mod(s.RAAN+dNode*dtMinutes, 2*math.Pi)
	omega := math.Mod(s.ArgPer+dOmega*dtMinutes, 2*math.Pi)
	M := math.Mod(s.MeanAnom+dM*dtMinutes, 2*math.Pi)
	if M < 0 {
		M += 2 * math.Pi
	}

	E := M
	for i := 0; i < 10; i++ {
		sinE, cosE := math.Sincos(E)
		fVal := E - ecc*sinE - M
		fDeriv := 1.0 - ecc*cosE
		E = E - fVal/fDeriv
	}

	sinE, cosE := math.Sincos(E)
	sqrt1e2 := math.Sqrt(1.0 - ecc*ecc)
	xOrb := a0 * (cosE - ecc)
	yOrb := a0 * sqrt1e2 * sinE

	vTrue := math.Atan2(yOrb, xOrb)
	r := math.Sqrt(xOrb*xOrb + yOrb*yOrb)
	u := vTrue + omega

	sinNode, cosNode := math.Sincos(node)
	sinInc, cosInc := math.Sincos(s.Inclin)
	sinU, cosU := math.Sincos(u)

	xECI := r * (cosNode*cosU - sinNode*sinU*cosInc)
	yECI := r * (sinNode*cosU + cosNode*sinU*cosInc)
	zECI := r * (sinU * sinInc)

	gmstRad := gmstTime(tUTC)

	sinGmst, cosGmst := math.Sincos(gmstRad)
	xECEF := xECI*cosGmst + yECI*sinGmst
	yECEF := -xECI*sinGmst + yECI*cosGmst
	zECEF := zECI

	return Vector{X: xECEF, Y: yECEF, Z: zECEF}
}

func gmstTime(t time.Time) float64 {
	unixSec := float64(t.Unix()) + float64(t.Nanosecond())/1e9
	jd := unixSec/86400.0 + 2440587.5
	d := jd - 2451545.0
	tut1 := d / 36525.0
	gmstDeg := 280.46061837 + 360.98564736629*d + 0.000387933*tut1*tut1 - tut1*tut1*tut1/38710000.0
	gmstDeg = math.Mod(gmstDeg, 360.0)
	if gmstDeg < 0 {
		gmstDeg += 360.0
	}
	return gmstDeg * math.Pi / 180.0
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

	for i := 0; i < 5; i++ {
		radLat := lat * math.Pi / 180.0
		N := a / math.Sqrt(1.0-e2*math.Sin(radLat)*math.Sin(radLat))
		alt = p/math.Cos(radLat) - N
		lat = math.Atan2(v.Z, p*(1.0-e2*(N/(N+alt)))) * 180.0 / math.Pi
	}

	return lat, lon, alt
}
