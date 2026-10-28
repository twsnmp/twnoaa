# twnoaa 📡🛰️

**twnoaa** is an all-in-one desktop application for automated weather satellite pass tracking, RTL-SDR radio reception, real-time DSP demodulation, multi-format satellite image decoding, and multimodal AI-powered meteorological analysis. Built with [Wails](https://wails.io/), [Svelte 5](https://svelte.dev/), and Go. Part of the TWSNMP project family.

> 🇯🇵 [日本語版 README はこちら](README_ja.md)

---

## 🗺️ System Workflow & Architecture

![twnoaa System Workflow & Architecture](images/overview.png)

*twnoaa integrates the full satellite reception pipeline: from SGP4 Orbit Tracking to RTL-SDR signal capture, real-time DSP (FFT Spectrum & Waterfall), multi-format image decoding, and automated multimodal AI meteorological analysis.*

---

## ✨ Key Features

- 🛰️ **Automated Orbit Propagation & Pass Prediction** — Built-in pure Go SGP4 engine fetches live TLEs from CelesTrak to calculate precise visible satellite passes and auto-schedule receptions based on observer coordinates.
- 📻 **Direct RTL-SDR Control & Real-Time DSP** — Direct hardware control via `librtlsdr`, including Bias-Tee power (4.5V for active LNAs), dynamic gain tuning, and live high-speed FFT spectrum & waterfall displays.
- 🧪 **Built-in Simulation Mode** — Full workflow verification (synthetic signal generation, DSP demodulation, APT image synthesis, AI analysis) even without physical SDR hardware connected.
- 🖼️ **Multi-Protocol Satellite Decoders**
  - **NOAA APT**: FM demodulation, 2400 Hz AM subcarrier sync frame cross-correlation, dual-channel (Visible & Thermal IR) high-resolution image rendering.
  - **METEOR-M2 LRPT**: QPSK demodulation, Costas loop carrier recovery, RRC matched filtering, Viterbi & Reed-Solomon error correction for high-resolution multispectral imagery.
  - **SSTV**: ISS (International Space Station) and amateur radio space image decoding (Martin, Scottie, and other modes).
- 🤖 **AI Meteorological Insights & Cat Animation** — Passes newly decoded satellite images and reception metadata to Multimodal LLMs (Google Gemini, OpenAI, Anthropic, or local Ollama) to produce weather front, cyclone vortex, and cloud pattern analysis, accompanied by a delightful animated cat character during reasoning.
- 📼 **Raw I/Q Data Recording & Replay** — Capture live raw RF signals to disk and replay them anytime with different filter parameters or decoder algorithms.
- 💾 **Local-First Persistent Storage** — Embedded bbolt key-value database securely stores all pass records, decoded images, and AI reports completely on your local machine.
- 🌐 **Bilingual UI** — Seamless switching between English and Japanese interfaces.

---

## 🛠️ Hardware Requirements

To receive signals from orbiting weather satellites and decode high-definition imagery, the following hardware setup is recommended:

### 1. SDR Receiver (Software Defined Radio)
![RTL-SDR Receiver](images/hardware_rtlsdr.jpg)

- **Recommended Model**: **RTL-SDR Blog V3 / V4**
- **Interface**: USB 2.0 / 3.0
- **Features**:
  - High sensitivity in the 137 MHz VHF weather satellite band.
  - **Built-in Software-Switchable Bias-Tee (4.5V DC)**: Power your active low-noise amplifier (LNA) directly through the coaxial cable with a single click in twnoaa—no external DC power injectors or extra power cables required.

### 2. Receiving Antenna
<img src="images/hardware_antenna.jpg" width="560" alt="V-Dipole Antenna Balcony Installation Example" />

*Example of a 120° V-dipole antenna mounted on a balcony railing using a clamp fixture*

- **V-Dipole Antenna**:
  - Element length for 137 MHz: ~53.4 cm per element (~107 cm total tip-to-tip)
  - Angle: ~120° V-shape mounted horizontally
  - Cost-effective, easy to build or purchase as a kit, and highly effective for overhead satellite passes.
- **QFH Antenna (Quadrifilar Helix)**:
  - Omnidirectional circular polarization (RHCP) antenna designed specifically for polar-orbiting satellites, offering balanced reception gain from low elevation up to zenith.
- **Turnstile Antenna (Crossed Dipole)**:
  - Omnidirectional 90° phased circular-polarization antenna.

### 3. LNA (Low Noise Amplifier) & Bandpass Filter
<img src="images/hardware_lna.jpg" width="360" alt="Nooelec SAWbird+ NOAA LNA Module" />

*Nooelec SAWbird+ NOAA LNA module (inline SMA coaxial cabling, powered via Bias-Tee)*

- **Recommended Model**: **Nooelec SAWbird+ NOAA** or **RTL-SDR Blog Wideband / Filtered LNA**
- Weather satellite signals arrive from ~850 km in space with very low power. An LNA placed near the antenna provides high gain (~30 dB) with an ultra-low noise figure, while the built-in SAW filter rejects strong out-of-band interference (FM broadcast 76–108 MHz, cellular, and land mobile radios).
- **Powering**: Automatically powered via coaxial cable by turning on the Bias-Tee switch in twnoaa.

### 4. Coaxial Cable & Connectors
- 50 Ω low-loss coaxial cable (RG58, 3D-2V, LMR-195/200, etc., with SMA connectors).

---

## 📡 Antenna Installation & LNA Connection Guide

Because satellite RF signals are very faint, proper antenna placement and LNA cabling order are critical for clear, snow-free imagery.

### Antenna Placement
1. **Clear Line of Sight**:
   - Install the antenna in an open location with an unobstructed view of the sky (e.g., roof, balcony railing, or garden mast).
   - Nearby buildings, dense trees, metal fences, and power lines attenuate VHF signals and introduce sync dropouts.
2. **V-Dipole Orientation**:
   - Keep the V-shaped element plane horizontal (parallel to the ground), and orient the opening of the V pointing True North and True South (since weather satellites travel in near-polar North-South orbits).

### LNA Connection Sequence
> [!IMPORTANT]
> **Always connect the LNA directly at the antenna feedpoint (before long cable runs)!**

```
[Antenna] ──(Short jumper cable)──> [LNA Input] ──> [LNA Output] ──(Long Coax Cable)──> [RTL-SDR Dongle] ──> [Host PC]
```

- **Why place the LNA near the antenna?**
  If the LNA is placed at the indoor end of a long coaxial cable, both the attenuated satellite signal and the noise picked up along the cable run will be amplified together, resulting in poor Signal-to-Noise Ratio (SNR). Amplifying the pure, un-attenuated signal right at the antenna preserves the best possible SNR.
- **Weatherproofing**:
  Enclose outdoor LNAs in a weatherproof junction box and wrap all outdoor RF connectors with self-amalgamating waterproof tape.
- **Activating Bias-Tee**:
  Toggle the **Bias-Tee** switch to ON in twnoaa's SDR control panel to supply 4.5V DC to the LNA through the coaxial line (confirm that the LNA power LED lights up).

---

## 🖥️ User Interface & Operation Guide

twnoaa features a streamlined tabbed layout for tracking, receiving, and analyzing satellite imagery.

### 1. Pass Tracker
![Pass Tracker](images/screen_tracker.png)

- Automatically fetches live TLE orbital elements from CelesTrak and calculates upcoming passes for NOAA 15/18/19 and METEOR-M2.
- **Data Columns**: Satellite Name, Frequency, Acquisition of Signal (AOS), Maximum Elevation, Loss of Signal (LOS), and live Countdown.
- **Action**: Click "Select Pass" to automatically configure the SDR frequency and demodulation mode. Passes with Max Elevation > 20° typically provide excellent reception.

### 2. 3D Globe View
![Globe View](images/screen_globe.png)

- Real-time 3D interactive Earth view depicting current satellite positions, orbit tracks, and reception footprints relative to your ground station coordinates.

### 3. SDR Control & Spectrum
![SDR Control & Spectrum](images/screen_sdr.png)

- **Hardware Control**: Center frequency tuning, manual/auto gain adjustment, Bias-Tee (4.5V) toggle, and raw I/Q signal recording.
- **Simulation Mode**: Run full end-to-end reception, demodulation, and AI analysis tests using internal synthetic signals even without hardware.
- **Live FFT Spectrum & Waterfall**: High-speed real-time visualization of signal bandwidth and Doppler frequency shift.
- **Decoder Preview**: Real-time line-by-line image rendering as APT, LRPT, or SSTV frames are decoded.

### 4. Image Gallery
![Image Gallery](images/screen_gallery.png)

- Grid gallery of all saved satellite passes and decoded images.
- **Full-Screen Viewer**: Interactive high-resolution viewer with center-locked zoom and pan for examining cloud structures, frontal boundaries, and coastlines.
- **Raw Replay**: Re-decode previously recorded raw I/Q files with customized DSP parameters.

### 5. AI Meteorological Analysis
![AI Analysis](images/screen_ai.png)

- Sends decoded satellite images and pass metadata to multimodal LLMs for automated meteorological analysis.
- **Cat Animation**: A charming detective cat animation modal keeps you entertained while the AI model processes the imagery.
- **Structured Report**: Generates professional markdown reports identifying cyclonic vortices, cold/warm fronts, typhoon eye structures, and convective cloud patterns.

### 6. Settings
![Settings](images/screen_settings.png)

- **Station Coordinates**: Set latitude, longitude, and elevation (with one-click IP geolocation lookup).
- **AI Providers**: Configure Google Gemini, OpenAI, Anthropic Claude, or local Ollama with API keys, models, and custom prompts.
- **UI Language**: Switch between English and Japanese.

---

## 🔍 Supported Satellites & Transmissions

| Target | Frequency | Mode / Modulation | Resolution / Details |
|---|---|---|---|
| **NOAA 15** | 137.620 MHz | APT (FM / AM 2.4 kHz Subcarrier) | 4 km/px, 2-channel (Visible & Thermal IR) |
| **NOAA 18** | 137.9125 MHz | APT (FM / AM 2.4 kHz Subcarrier) | 4 km/px, 2-channel (Visible & Thermal IR) |
| **NOAA 19** | 137.100 MHz | APT (FM / AM 2.4 kHz Subcarrier) | 4 km/px, 2-channel (Visible & Thermal IR) |
| **METEOR-M2-3/4** | 137.100 / 137.900 MHz | LRPT (72k/80k QPSK) | 1 km/px, Digital high-resolution multispectral |
| **ISS / SSTV** | 145.800 MHz | SSTV (FM / Various SSTV modes) | Space events & school contact picture transmissions |

---

## 🤖 AI Meteorological Analysis

When a satellite pass is completed and the image is decoded, the metadata (Satellite, Frequency, Timestamp, Maximum Elevation, Coordinates) is packaged with the image and submitted to the configured multimodal LLM:

- **Synoptic Weather Analysis**: Identification of low-pressure cyclonic vortices, cold/warm fronts, typhoon eye structures, and convective cloud bands.
- **Landmass & Geographic Recognition**: Detection of coastlines, peninsulas, islands, snow/ice cover, and sea surface temperature gradients.
- **Structured Markdown Report**: Clear, professional weather observations suitable for amateur radio satellite trackers.

### Supported AI Providers
- **Google Gemini** (`gemini-2.5-flash`, `gemini-1.5-pro`, etc.)
- **OpenAI** (`gpt-4o`, `gpt-4o-mini`, etc.)
- **Anthropic Claude** (`claude-3-5-sonnet`, etc.)
- **Ollama** (Local models: `llava`, `minicpm-v`, etc.)

---

## 🛠️ Tech Stack

| Component | Technology |
|---|---|
| **Desktop Framework** | [Wails v2](https://wails.io/) |
| **Backend** | Go 1.27 + CGO (`librtlsdr`) |
| **Frontend** | Svelte 5 (Runes) + TypeScript + Tailwind CSS |
| **Database** | [bbolt](https://github.com/etcd-io/bbolt) (Embedded Key-Value Store) |
| **SDR / DSP** | `github.com/jpoirier/gortlsdr`, Pure-Go DSP (WFM, QPSK, FFT, Envelope Extraction) |
| **Orbit Engine** | Pure-Go SGP4 propagation model + CelesTrak TLE client |
| **AI Integration** | `langchaingo` + Multimodal LLM providers |
| **Packaging** | macOS PKG (Apple Notarization), Windows ZIP, Linux tar.gz |

---

## 🚀 Installation & Releases

### Download Prebuilt Packages

Visit the [Releases](https://github.com/twsnmp/twnoaa/releases) page to get binaries for your operating system:

- **macOS (Apple Silicon)**: Download `twnoaa-v0.1.0-darwin-arm64.pkg` and run the signed installer (notarized by Apple).
- **Windows (x64)**: Download `twnoaa-windows-amd64.zip`, extract to any directory, and run `twnoaa.exe` (includes required `rtlsdr.dll`).
- **Linux (x64)**: Download `twnoaa-linux-amd64.tar.gz`, extract and run (requires `librtlsdr0`, `libwebkit2gtk-4.1-0`).

---

## 💻 Development Setup

### Prerequisites

- [mise](https://mise.jdx.dev/) (Toolchain manager for Go and Node.js)
- [Wails CLI v2.15+](https://wails.io/docs/gettingstarted/installation)
- RTL-SDR library (`librtlsdr`):
  - macOS: `brew install librtlsdr`
  - Ubuntu/Debian: `sudo apt-get install -y librtlsdr-dev libgtk-3-dev libwebkit2gtk-4.1-dev`
  - Windows: prebuilt 64-bit DLL & headers

### Setup Dependencies

```bash
# Install Go and Node via mise
mise install

# Install frontend dependencies
cd frontend
npm install
cd ..
```

### Live Development Mode

```bash
mise run dev
# or
wails dev
```

### Production Build

```bash
# Local build
mise run build

# macOS signed and notarized installer package (.pkg)
mise run release-mac
```

---

## 🗄️ Data Directories

Configuration, pass logs, decoded images, and the bbolt database are stored locally in the following OS directories:

| Platform | Default Path |
|---|---|
| **macOS** | `~/Library/Application Support/twnoaa/` |
| **Windows** | `%APPDATA%\twnoaa\` |
| **Linux** | `~/.config/twnoaa/` |

---

## 📄 License

[Apache License 2.0](LICENSE)
