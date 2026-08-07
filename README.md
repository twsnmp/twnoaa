# twnoaa 📡🌏

**twnoaa** is an open-source, cross-platform desktop application built with Go, Wails v2, and Svelte 5. It provides an all-in-one automated solution for real-time NOAA weather satellite pass prediction, SDR frequency tuning, signal reception, DSP demodulation, APT image decoding, and AI-powered meteorological image analysis.

---

## 🌟 Key Features

1. **Automated Orbit Prediction & Satellite Selection**
   - Built-in SGP4 orbital propagator (zero external Go dependencies).
   - Fetches live TLEs from Celestrak for **NOAA 15, 18, and 19**.
   - Real-time pass calculator with auto-selection of the active/optimal satellite based on local observer coordinates and elevation.

2. **Direct Hardware Control & Real-Time DSP**
   - Direct integration with **RTL-SDR** devices via Cgo / `librtlsdr`.
   - Real-time streaming I/Q processing.
   - On-the-fly FM demodulation, low-pass filtering, and PCM resampling to 11025 Hz.

3. **Robust APT Image Decoding**
   - AM envelope detection and synchronization frame alignment (Cross-Correlation).
   - Generates high-resolution PNG satellite images with contrast enhancement and aspect correction.

4. **🤖 AI-Powered Meteorological Image Analysis (LLM Integration)**
   - Automatically passes newly decoded APT images along with pass metadata (Satellite name, Timestamp, Max Elevation, Location) to Multimodal LLMs (Gemini / OpenAI API).
   - Generates automated meteorological insights, cloud pattern recognition (low-pressure vortices, weather fronts, typhoon structures), and land/ocean landmark descriptions.

5. **Modern UI/UX**
   - Clean, reactive, and lightweight dashboard built with **Svelte 5** and Tailwind CSS running inside **Wails v2**.

---

## 🏗️ Architecture & Component Design

```
twnoaa/
├── app.go                  # Wails application bindings (Frontend <-> Backend bridge)
├── main.go                 # Application entry point & Wails initialization
├── pkg/
│   ├── tracker/            # SGP4 propagator, TLE parser, Pass scheduler
│   ├── sdr/                # RTL-SDR driver wrapper (librtlsdr), I/Q streaming
│   ├── dsp/                # WFM demodulation, Resampling, Envelope extraction
│   ├── decoder/            # APT sync frame alignment, PNG rendering
│   └── llm/                # Gemini / OpenAI API integration for image analysis
└── frontend/               # Svelte 5 + TypeScript + Tailwind CSS UI
```

---

## 🤖 LLM Analysis Specification

When an APT image (`.png`) is decoded, the backend invokes the LLM module.

### Metadata Payload & Prompt Structure
- **Inputs:** Base64-encoded APT PNG image + Metadata JSON
- **System Prompt Specification:**
  > You are a meteorologist analyzing a newly received APT (Automatic Picture Transmission) image from a NOAA weather satellite captured via an amateur RTL-SDR ground station.
  > 
  > **Reception Metadata:**
  > - **Satellite:** {Satellite Name}
  > - **Frequency:** {Frequency} MHz
  > - **Timestamp:** {YYYY-MM-DD HH:mm:ss} (JST)
  > - **Observer Location:** Kitamoto, Saitama, Japan (36.028°N, 139.533°E)
  > - **Max Elevation:** {Max Elevation}°
  > 
  > **Task:**
  > 1. Analyze the visible (Channel A) and infrared (Channel B) image halves.
  > 2. Identify significant weather systems (e.g., cold/warm fronts, low-pressure spirals, typhoon structures, coastal landlines).
  > 3. Provide a concise, professional weather analysis in Japanese markdown format suitable for amateur satellite trackers.

---

## 🛠️ Tech Stack

- **Backend:** Go 1.22+, Cgo (`librtlsdr`), Standard Math/SGP4 Engine
- **Desktop Framework:** [Wails v2](https://wails.io/)
- **Frontend:** Svelte 5, TypeScript, Tailwind CSS
- **AI Integrations:** Google Gemini API (`gemini-1.5-flash`), OpenAI API (`gpt-4o-mini`)

---

## 🚀 Development Setup for AI Agents / Contributors

### Prerequisites
- Go 1.22 or higher
- Node.js v18+ & npm
- `librtlsdr` library installed on target host (`brew install librtlsdr` on macOS)
- Wails CLI (`go install github.com/wailsapp/wails/v2/cmd/wails@latest`)

### Build & Run

```bash
# Clone the repository
git clone [https://github.com/your-username/twnoaa.git](https://github.com/your-username/twnoaa.git)
cd twnoaa

# Run in live-development mode
wails dev
```

---

## 📜 License

