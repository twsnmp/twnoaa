const translations = {
  en: {
    title: "twnoaa Satellite Station",
    subtitle: "NOAA Weather Satellite Receiver & AI Engine",
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
      refresh: "Refresh TLEs",
      activePass: "ACTIVE PASS",
      position: "Position",
      tuneNow: "Tune & Receive Now",
      action: "Action",
      tune: "Tune Receiver",
      noPasses: "No upcoming satellite passes detected in the next 24 hours."
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
      recording: "Recording in progress...",
      hwActive: "Direct librtlsdr Hardware Control Active",
      simActive: "Software IQ Simulation Engine Active",
      forceSim: "Force Sim Mode",
      tuningTitle: "Tuning & Demodulator",
      presets: "NOAA Satellite Presets",
      band137: "137 MHz Band",
      streamPaused: "Stream Paused",
      streamPausedHint: "Click \"Start Stream\" above to visualize real-time RF spectrum waterfall.",
      recordingProgress: "Recording 30s pass & demodulating FM...",
      decodeSuccess: "Decoding Complete! View image in Gallery.",
      decodeFailed: "Decode Failed: ",
      deviceFound: "RTL-SDR Device #{id} ({count} found)",
      noHardware: "No Hardware (Simulation Mode)"
    },
    gallery: {
      title: "Decoded APT Image Gallery",
      subtitle: "Decoded APT Satellite PNG Images & Reception History",
      empty: "No decoded images found yet. Record a satellite pass to decode images.",
      noImages: "No Decoded APT Images Yet",
      decodedPasses: "Decoded Passes",
      sat: "Satellite",
      date: "Captured Date",
      maxElev: "Max Elev",
      maxElevLabel: "Max Elev:",
      viewFull: "Full Image",
      viewChanA: "Channel A (Visible)",
      viewChanB: "Channel B (Infrared)",
      runAI: "Analyze with AI",
      aiReady: "AI Analysis Ready",
      imageRes: "Image Resolution: 2080px line scan"
    },
    ai: {
      title: "AI Meteorological Image Analysis",
      subtitle: "Multi-modal Meteorological LLM Engine (Gemini / OpenAI / Ollama / Anthropic)",
      provider: "Active LLM Provider",
      autoAnalyze: "Auto-analyze newly decoded passes",
      analyzeNow: "Run AI Analysis Now",
      noAnalysis: "Select a pass record from the gallery to run AI meteorological analysis.",
      noPassSelected: "No Pass Selected for Analysis",
      analyzing: "Running LLM analysis...",
      reportTitle: "AI Meteorological Report",
      noReport: "No analysis report generated yet. Click \"Run AI Meteorological Analysis\" to generate insights.",
      statusRunning: "Running multi-modal LLM meteorological analysis...",
      statusCompleted: "Analysis completed successfully!",
      statusError: "Error starting AI analysis: ",
      statusFailed: "AI Analysis Failed: "
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
    subtitle: "NOAA 気象衛星受信機 ＆ AI エンジン",
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
      refresh: "TLE更新",
      activePass: "衛星飛来中",
      position: "現在位置",
      tuneNow: "チューニング＆受信",
      action: "操作",
      tune: "チューニング",
      noPasses: "直近24時間に通過する気象衛星はありません。"
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
      recording: "信号録音・復調デコード中...",
      hwActive: "ダイレクト librtlsdr ハードウェア制御 アクティブ",
      simActive: "ソフトウェア IQ シミュレーションエンジン アクティブ",
      forceSim: "シミュレーションモード強制",
      tuningTitle: "チューニング ＆ 復調器",
      presets: "NOAA 衛星プリセット",
      band137: "137 MHz 帯",
      streamPaused: "ストリーミング停止中",
      streamPausedHint: "上部の「受信開始」をクリックしてリアルタイムRFスペクトラム・ウォーターフォールを表示します。",
      recordingProgress: "30秒間の衛星パスを録音中＆FM復調中...",
      decodeSuccess: "デコード完了！ ギャラリーで画像を確認できます。",
      decodeFailed: "デコード失敗: ",
      deviceFound: "RTL-SDR デバイス #{id} ({count}件検出)",
      noHardware: "SDRハードウェア非検出 (シミュレーションモード)"
    },
    gallery: {
      title: "解密済み APT 衛星画像ギャラリー",
      subtitle: "解密済み APT 衛星画像 ＆ 受信履歴",
      empty: "まだ解密画像がありません。SDR受信タブで衛星パスを録音・デコードしてください。",
      noImages: "解密されたAPT画像はまだありません",
      decodedPasses: "解密済みパス",
      sat: "衛星名",
      date: "受信日時",
      maxElev: "最大仰角",
      maxElevLabel: "最大仰角:",
      viewFull: "全体画像",
      viewChanA: "チャンネルA (可視光)",
      viewChanB: "チャンネルB (赤外線)",
      runAI: "AI気象解析を実行",
      aiReady: "AI解析完了",
      imageRes: "解像度: 2080px ラインスキャン"
    },
    ai: {
      title: "AI気象解析システム (マルチLLM連携)",
      subtitle: "マルチモーダル気象 LLM エンジン (Gemini / OpenAI / Ollama / Anthropic)",
      provider: "使用するLLMプロバイダー",
      autoAnalyze: "新しい画像の自動AI解析",
      analyzeNow: "AI気象解析を実行",
      noAnalysis: "ギャラリーから衛星データを選択してAI解析を実行してください。",
      noPassSelected: "解析対象のパスが選択されていません",
      analyzing: "LLM解析処理を実行中...",
      reportTitle: "AI 気象解析レポート",
      noReport: "解析レポートはまだ生成されていません。「AI気象解析を実行」をクリックして生成してください。",
      statusRunning: "マルチモーダル LLM 気象解析を実行中...",
      statusCompleted: "AI解析が正常に完了しました！",
      statusError: "AI解析の開始エラー: ",
      statusFailed: "AI解析に失敗しました: "
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

