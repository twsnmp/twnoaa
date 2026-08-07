const translations = {
  en: {
    title: "twnoaa Satellite Station",
    tabs: {
      tracker: "Pass Tracker",
      sdr: "SDR Receiver",
      gallery: "APT Gallery",
      ai: "AI Analysis",
      settings: "Settings"
    },
    status: {
      active: "RECEIVING SATELLITE",
      standby: "STANDBY / MONITORING",
      sim: "SIMULATION MODE"
    },
    tracker: {
      liveStatus: "Live Satellite Sky Watch",
      upcomingPasses: "Upcoming Passes (24 Hours)",
      satName: "Satellite",
      freq: "Frequency",
      elevation: "Elevation",
      azimuth: "Azimuth",
      doppler: "Doppler Shift",
      aos: "AOS Time",
      los: "LOS Time",
      maxElev: "Max Elev",
      duration: "Duration",
      refresh: "Refresh TLEs"
    },
    sdr: {
      title: "SDR Signal Receiver & DSP Demodulator",
      device: "Selected Hardware",
      centerFreq: "Center Frequency (Hz)",
      gain: "Tuner Gain (dB)",
      sampleRate: "Sample Rate",
      waterfall: "Real-time FFT Spectrum Waterfall",
      startStream: "Start Stream",
      stopStream: "Stop Stream",
      startRecord: "Record & Decode APT (30s)",
      recording: "Recording in progress..."
    },
    gallery: {
      title: "Decoded APT Image Gallery",
      empty: "No decoded images found yet. Record a satellite pass to decode images.",
      sat: "Satellite",
      date: "Captured Date",
      maxElev: "Max Elev",
      viewFull: "Full Image",
      viewChanA: "Channel A (Visible)",
      viewChanB: "Channel B (Infrared)",
      runAI: "Analyze with AI"
    },
    ai: {
      title: "AI Meteorological Image Analysis",
      provider: "Active LLM Provider",
      autoAnalyze: "Auto-analyze newly decoded passes",
      analyzeNow: "Run AI Analysis Now",
      noAnalysis: "Select a pass record from the gallery to run AI meteorological analysis.",
      analyzing: "Running LLM analysis..."
    },
    settings: {
      title: "Station Settings & Location",
      subtitle: "Observer Station Coordinates, SDR & LLM API Configuration",
      observerTitle: "Observer Location Coordinates",
      observerCity: "Observer Location Name",
      lat: "Latitude (°N)",
      lon: "Longitude (°E)",
      alt: "Altitude (km)",
      autoLocation: "Auto Detect via IP",
      sdrConfig: "SDR Configuration",
      simMode: "Force Simulation Mode",
      simDesc: "Synthesize IQ signal offline for testing without RTL-SDR hardware.",
      defaultGain: "Default Tuner Gain (dB * 10)",
      llmConfig: "LLM Provider Settings",
      activeProvider: "Active AI Provider",
      autoAnalyzeDesc: "Trigger AI meteorological analysis automatically when a pass finishes decoding.",
      geminiKey: "Google Gemini API Key",
      openaiKey: "OpenAI API Key",
      anthropicKey: "Anthropic API Key",
      ollamaUrl: "Ollama URL",
      ollamaModel: "Ollama Model",
      save: "Save Settings"
    }
  },
  ja: {
    title: "twnoaa 気象衛星受信ステーション",
    tabs: {
      tracker: "衛星追跡・パス予測",
      sdr: "SDR受信用・DSP",
      gallery: "APT画像ギャラリー",
      ai: "AI気象解析",
      settings: "システム設定"
    },
    status: {
      active: "衛星信号受信可能 (ACTIVE)",
      standby: "待機中・圏外",
      sim: "シミュレーションモード"
    },
    tracker: {
      liveStatus: "リアルタイム衛星飛来モニタ",
      upcomingPasses: "直近24時間の通過スケジュール",
      satName: "衛星名",
      freq: "中心周波数",
      elevation: "現在仰角",
      azimuth: "方位角",
      doppler: "ドップラーシフト",
      aos: "通過開始 (AOS)",
      los: "通過終了 (LOS)",
      maxElev: "最大仰角 (MEL)",
      duration: "所要時間",
      refresh: "TLE更新"
    },
    sdr: {
      title: "SDR受信機 & リアルタイムDSP復調器",
      device: "検出ハードウェア",
      centerFreq: "中心周波数 (Hz)",
      gain: "チューナーゲイン (dB)",
      sampleRate: "サンプリングレート",
      waterfall: "リアルタイム FFT スペクトラム・ウォーターフォール",
      startStream: "受信開始",
      stopStream: "受信停止",
      startRecord: "録音＆APTデコード実行 (30秒)",
      recording: "信号録音・復調デコード中..."
    },
    gallery: {
      title: "解密済み APT 衛星画像ギャラリー",
      empty: "まだ解密画像がありません。SDR受信タブで衛星パスを録音・デコードしてください。",
      sat: "衛星名",
      date: "受信日時",
      maxElev: "最大仰角",
      viewFull: "全体画像",
      viewChanA: "チャンネルA (可視光)",
      viewChanB: "チャンネルB (赤外線)",
      runAI: "AI気象解析を実行"
    },
    ai: {
      title: "AI気象解析システム (マルチLLM連携)",
      provider: "使用するLLMプロバイダー",
      autoAnalyze: "新しい画像の自動AI解析",
      analyzeNow: "AI気象解析を実行",
      noAnalysis: "ギャラリーから衛星データを選択してAI解析を実行してください。",
      analyzing: "LLM解析処理を実行中..."
    },
    settings: {
      title: "地上局位置 ＆ SDR・LLM設定",
      subtitle: "地上局観測地の座標、SDR受信機、LLM APIの設定",
      observerTitle: "観測地点の座標設定",
      observerCity: "観測地点名",
      lat: "緯度 (°N)",
      lon: "経度 (°E)",
      alt: "標高 (km)",
      autoLocation: "IPアドレスから自動取得",
      sdrConfig: "SDR受信機設定",
      simMode: "シミュレーションモードを強制使用",
      simDesc: "RTL-SDR ハードウェアがない環境でテスト用 IQ 信号を疑似生成します。",
      defaultGain: "デフォルトチューナーゲイン (dB * 10)",
      llmConfig: "LLMプロバイダー設定",
      activeProvider: "アクティブ AI プロバイダー",
      autoAnalyzeDesc: "衛星パスのデコード完了時に自動で AI 気象解析を実行します。",
      geminiKey: "Google Gemini API Key",
      openaiKey: "OpenAI API Key",
      anthropicKey: "Anthropic API Key",
      ollamaUrl: "Ollama サーバー URL",
      ollamaModel: "Ollama モデル名",
      save: "設定を保存"
    }
  }
}

function detectOSLanguage() {
  const lang = typeof navigator !== 'undefined' ? (navigator.language || navigator.languages[0]) : 'ja'
  return lang.startsWith('ja') ? 'ja' : 'en'
}

class I18nManager {
  currentLocale = $state(detectOSLanguage())

  setLocale(lang) {
    if (translations[lang]) {
      this.currentLocale = lang
    }
  }

  t(path) {
    const keys = path.split('.')
    let translation = translations[this.currentLocale]
    for (const key of keys) {
      if (translation && translation[key] !== undefined) {
        translation = translation[key]
      } else {
        return keys[keys.length - 1]
      }
    }
    return translation
  }
}

export const i18n = new I18nManager()
