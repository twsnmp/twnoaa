const translations = {
  en: {
    title: "twnoaa Satellite Station",
    subtitle: "NOAA Weather Satellite Receiver & AI Engine",
    tabs: {
      tracker: "Pass Tracker",
      globe: "3D Orbit",
      sdr: "SDR Receiver",
      gallery: "APT Gallery",
      ai: "AI Analysis",
      settings: "Settings"
    },
    globe: {
      title: "3D Satellite Orbit Watch",
      subtitle: "Interactive 3D Earth Globe & Orbit Visualizer",
      station: "Observer Station",
      activeSats: "Weather Satellites"
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
      noPasses: "No upcoming satellite passes detected in the next 24 hours.",
      autoRecordActive: "Auto Pass Scheduler Active",
      autoRecordOff: "Auto Pass Scheduler Off",
      autoRecordComplete: "Auto-recording & decoding completed successfully.",
      autoRecordFailed: "Auto-recording & decoding failed."
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
      signalLabel: "Signal Quality:",
      rssiLabel: "RSSI:",
      signalExcellent: "Excellent",
      signalGood: "Good",
      signalFair: "Fair",
      signalWeak: "Weak",
      signalUnknown: "N/A",
      viewFull: "Full Image",
      viewChanA: "Channel A (Visible)",
      viewChanB: "Channel B (Infrared)",
      runAI: "Analyze with AI",
      aiReady: "AI Analysis Ready",
      imageRes: "Image Resolution: 2080px line scan",
      fitAspect: "Aspect Fit",
      stretchY: "Stretch Height",
      clickToZoom: "Click image to view full size",
      delete: "Delete Record",
      deleteConfirm: "Are you sure you want to delete this pass record and its image files?",
      cancel: "Cancel",
      imageNotFound: "Image file not found or failed to decode.",
      signalChartTitle: "Signal & Noise History Chart",
      signalHistoryTab: "Signal Graph",
      imageTab: "APT Image",
      signalLevel: "Signal Level (dBFS)",
      noiseLevel: "Noise Floor (dBFS)",
      snr: "SNR (dB)",
      timeOffset: "Elapsed Time",
      noSignalData: "No signal/noise history data available for this pass.",
      testDecodeWav: "Decode from WAV File (Test)",
      decodingWav: "Decoding WAV file..."
    },
    ai: {
      title: "AI Meteorological Image Analysis",
      subtitle: "Multi-modal Meteorological LLM Engine (Gemini / OpenAI / Ollama / Anthropic)",
      provider: "Active LLM Provider",
      autoAnalyze: "Auto-analyze newly decoded passes",
      analyzeNow: "Run AI Analysis Now",
      noAnalysis: "Select a pass record from the gallery or dropdown to run AI meteorological analysis.",
      noPassSelected: "No Pass Selected for Analysis",
      selectPass: "Select Pass Record",
      noImageAvailable: "No valid image file available for analysis",
      noPassAvailable: "No satellite pass records found",
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
      autoRecord: "Auto-record Satellite Passes",
      autoRecordDesc: "Automatically tune, record, and decode satellite passes when elevation reaches 5° or higher.",
      autoRecordDuration: "Auto Record Duration (seconds)",
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
      globe: "3D衛星軌道",
      sdr: "SDR受信用・DSP",
      gallery: "APT画像ギャラリー",
      ai: "AI気象解析",
      settings: "システム設定"
    },
    globe: {
      title: "3D 衛星軌道モニタ",
      subtitle: "インタラクティブ 3D 地球儀 ＆ 軌道ビジュアル",
      station: "地上局観測地",
      activeSats: "気象衛星一覧"
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
      noPasses: "直近24時間に通過する気象衛星はありません。",
      autoRecordActive: "自動録音スケジューラー稼働中",
      autoRecordOff: "自動録音スケジューラーOFF",
      autoRecordComplete: "自動録音・デコードが完了しました",
      autoRecordFailed: "自動録音・デコードでエラーが発生しました"
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
      title: "デコード済み APT 衛星画像ギャラリー",
      subtitle: "デコード済み APT 衛星画像 ＆ 受信履歴",
      empty: "まだデコードされた画像がありません。SDR受信タブで衛星パスを録音・デコードしてください。",
      noImages: "デコードされたAPT画像はまだありません",
      decodedPasses: "デコード済みパス",
      sat: "衛星名",
      date: "受信日時",
      maxElev: "最大仰角",
      maxElevLabel: "最大仰角:",
      signalLabel: "受信状態:",
      rssiLabel: "RSSI:",
      signalExcellent: "良好",
      signalGood: "普通",
      signalFair: "可",
      signalWeak: "弱",
      signalUnknown: "未計測",
      viewFull: "全体画像",
      viewChanA: "チャンネルA (可視光)",
      viewChanB: "チャンネルB (赤外線)",
      runAI: "AI気象解析を実行",
      aiReady: "AI解析完了",
      imageRes: "解像度: 2080px ラインスキャン",
      fitAspect: "標準アスペクト比",
      stretchY: "縦拡大表示 (視認性重視)",
      clickToZoom: "クリックで拡大表示",
      delete: "レコード削除",
      deleteConfirm: "この受信履歴と関連画像ファイルを削除しますか？",
      cancel: "キャンセル",
      imageNotFound: "画像ファイルが存在しないか、受信・復調に失敗したデータです。",
      signalChartTitle: "受信信号 ＆ ノイズ時系列変化グラフ",
      signalHistoryTab: "信号変化グラフ",
      imageTab: "APT画像",
      signalLevel: "受信信号レベル (dBFS)",
      noiseLevel: "ノイズレベル (dBFS)",
      snr: "S/N比 (SNR dB)",
      timeOffset: "経過時間",
      noSignalData: "このパスの信号・ノイズ時系列データは記録されていません。",
      testDecodeWav: "WAVファイルからデコード (テスト)",
      decodingWav: "WAVファイルをデコード中..."
    },
    ai: {
      title: "AI気象解析システム (マルチLLM連携)",
      subtitle: "マルチモーダル気象 LLM エンジン (Gemini / OpenAI / Ollama / Anthropic)",
      provider: "使用するLLMプロバイダー",
      autoAnalyze: "新しい画像の自動AI解析",
      analyzeNow: "AI気象解析を実行",
      noAnalysis: "ギャラリーまたは上部の選択メニューから衛星データを選択してAI解析を実行してください。",
      noPassSelected: "解析対象のパスが選択されていません",
      selectPass: "解析対象パスの選択",
      noImageAvailable: "解析可能な画像データが存在しません",
      noPassAvailable: "デコード済みの衛星パスデータがありません",
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
      autoRecord: "衛星通過時の自動録音・デコード",
      autoRecordDesc: "衛星が地上局上空を通過する時刻（仰角5°以上）になると、自動的にSDR受信・録音・画像デコードを実行します。",
      autoRecordDuration: "自動録音時間 (秒)",
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

