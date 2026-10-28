# twnoaa 📡🛰️

**twnoaa** is an all-in-one desktop application for automated weather satellite pass tracking, RTL-SDR radio reception, real-time DSP demodulation, multi-format satellite image decoding, and multimodal AI-powered meteorological analysis. Built with [Wails](https://wails.io/), [Svelte 5](https://svelte.dev/), and Go. Part of the TWSNMP project family.

> 🇯🇵 [日本語版 README はこちら](README_ja.md)

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
