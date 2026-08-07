# twnoaa 📡🌏

**twnoaa** is a cross-platform desktop application designed for automated NOAA weather satellite signal reception and real-time image decoding. Powered by Go (backend DSP, SGP4 orbital propagation, and RTL-SDR driver) and Wails + Svelte 5 (frontend UI), it provides an end-to-end solution for amateur radio and weather satellite enthusiasts.

### Key Features
- **Automated Orbit Prediction:** In-app SGP4 orbital propagator with automatic TLE updates from Celestrak to predict passes for NOAA 15, 18, and 19.
- **Smart Auto-Selection:** Automatically identifies and switches frequencies for the optimal satellite based on current elevation and AOS (Acquisition of Signal) schedules.
- **SDR Integration:** Direct I/Q data streaming and control for RTL-SDR devices via `librtlsdr`.
- **On-the-Fly DSP & Decoding:**
  - Real-time WFM demodulation and downsampling.
  - AM envelope extraction for NOAA APT (Automatic Picture Transmission) signals.
  - Global cross-correlation sync frame alignment for drift-free image rendering.
- **Modern UI:** Responsive and reactive dashboard built with Svelte 5 and Wails.

### Tech Stack
- **Backend:** Go (Cgo / `librtlsdr`, SGP4 propagator, DSP engine, Image generator)
- **Framework:** [Wails v2](https://wails.io/)
- **Frontend:** Svelte 5 + Tailwind CSS / TypeScript

