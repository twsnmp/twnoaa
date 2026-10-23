<script>
  import { onMount, onDestroy } from 'svelte'
  import { i18n } from '../i18n.svelte.js'
  import { Play, Pause, RotateCcw, RotateCw, Activity, Radio, Music, AlertCircle, Zap, FastForward } from '@lucide/svelte'

  let { record = null, filePath = '', title = '' } = $props()

  let spectrumData = $state(null)
  let loading = $state(false)
  let errorMsg = $state('')
  let selectedSource = $state('auto') // 'auto' | 'raw_iq' | 'audio_wav'

  let currentFrameIndex = $state(0)
  let isPlaying = $state(false)
  let playbackSpeed = $state(1)
  let playTimer = null

  let waterfallCanvas = $state(null)
  let spectrumCanvas = $state(null)

  let hasRawIQ = $derived(
    Boolean(record?.raw_iq_path) || (filePath && (filePath.endsWith('.iq') || filePath.endsWith('.raw')))
  )

  let hasWAV = $derived(
    Boolean(record?.audio_path) || (filePath && filePath.endsWith('.wav'))
  )

  let currentFrame = $derived(
    spectrumData?.frames && spectrumData.frames.length > 0
      ? spectrumData.frames[Math.min(currentFrameIndex, spectrumData.frames.length - 1)]
      : null
  )

  let targetFreqHz = $derived.by(() => {
    if (!spectrumData) return null
    if (spectrumData.type === 'audio_wav') {
      return 2400 // APT subcarrier frequency (2.4 kHz)
    }
    if (record?.frequency && record.frequency > 0) {
      return record.frequency
    }
    return spectrumData.center_freq_hz || null
  })

  let targetBandwidthHz = $derived.by(() => {
    if (!spectrumData) return 0
    if (spectrumData.type === 'audio_wav') {
      return 2000 // ±1 kHz for audio subcarrier
    }
    if (record?.sat_name && record.sat_name.toUpperCase().startsWith('METEOR')) {
      return 80000 // ±40 kHz for 72kbaud OQPSK LRPT signal bandwidth
    }
    return 40000 // ±20 kHz for RF APT signal bandwidth
  })

  let peakInfo = $derived.by(() => {
    if (!currentFrame || !currentFrame.bins_db || !spectrumData) {
      return { freq: 'N/A', dbfs: 'N/A', freqHz: 0 }
    }
    const dbs = currentFrame.bins_db
    let maxDb = -200
    let maxIdx = 0
    for (let i = 0; i < dbs.length; i++) {
      if (dbs[i] > maxDb) {
        maxDb = dbs[i]
        maxIdx = i
      }
    }

    const freqSpan = spectrumData.freq_end_hz - spectrumData.freq_start_hz
    const peakFreqHz = spectrumData.freq_start_hz + (freqSpan * maxIdx) / (dbs.length - 1)

    let freqText = ''
    if (spectrumData.type === 'rf_iq') {
      freqText = `${(peakFreqHz / 1e6).toFixed(4)} MHz`
    } else {
      freqText = `${(peakFreqHz / 1e3).toFixed(2)} kHz`
    }

    return {
      freqHz: peakFreqHz,
      freq: freqText,
      dbfs: `${maxDb.toFixed(1)} dBFS`
    }
  })

  let targetInfo = $derived.by(() => {
    if (!currentFrame || !currentFrame.bins_db || !spectrumData || !targetFreqHz) {
      return { freq: 'N/A', dbfs: 'N/A', delta: '', inBand: false, targetDb: -100 }
    }
    const freqSpan = spectrumData.freq_end_hz - spectrumData.freq_start_hz
    if (freqSpan <= 0) {
      return { freq: 'N/A', dbfs: 'N/A', delta: '', inBand: false, targetDb: -100 }
    }

    const targetRatio = (targetFreqHz - spectrumData.freq_start_hz) / freqSpan
    const dbs = currentFrame.bins_db
    let targetDb = -100

    if (targetRatio >= 0 && targetRatio <= 1) {
      const floatIdx = targetRatio * (dbs.length - 1)
      const i0 = Math.floor(floatIdx)
      const i1 = Math.min(dbs.length - 1, i0 + 1)
      const frac = floatIdx - i0
      targetDb = dbs[i0] * (1 - frac) + dbs[i1] * frac
    }

    let freqText = ''
    if (spectrumData.type === 'rf_iq') {
      freqText = `${(targetFreqHz / 1e6).toFixed(4)} MHz`
    } else {
      freqText = `${(targetFreqHz / 1e3).toFixed(2)} kHz`
    }

    let deltaText = ''
    if (peakInfo && peakInfo.freqHz) {
      const deltaHz = peakInfo.freqHz - targetFreqHz
      if (spectrumData.type === 'rf_iq') {
        const deltaKHz = deltaHz / 1e3
        deltaText = `${deltaKHz >= 0 ? '+' : ''}${deltaKHz.toFixed(1)} kHz`
      } else {
        deltaText = `${deltaHz >= 0 ? '+' : ''}${deltaHz.toFixed(0)} Hz`
      }
    }

    return {
      freqHz: targetFreqHz,
      freq: freqText,
      dbfs: `${targetDb.toFixed(1)} dBFS`,
      delta: deltaText,
      inBand: targetRatio >= 0 && targetRatio <= 1,
      targetDb: targetDb
    }
  })

  async function loadSpectrum() {
    loading = true
    errorMsg = ''
    stopPlayback()

    try {
      if (window.go?.main?.App) {
        let res = null
        if (record?.id) {
          res = await window.go.main.App.GetRecordSpectrum(record.id, selectedSource)
        } else if (filePath) {
          res = await window.go.main.App.AnalyzeSpectrumFile(filePath)
        }

        if (res && res.frames && res.frames.length > 0) {
          spectrumData = res
          currentFrameIndex = 0
          setTimeout(() => {
            drawWaterfall()
            drawInstantSpectrum()
          }, 50)
        } else {
          spectrumData = null
          errorMsg = i18n.t('gallery.spectrumUnavailable')
        }
      } else {
        errorMsg = 'Backend API unavailable'
      }
    } catch (err) {
      console.error('Failed to load spectrum:', err)
      errorMsg = 'Error: ' + (err.message || err)
      spectrumData = null
    } finally {
      loading = false
    }
  }

  $effect(() => {
    record?.id
    filePath
    selectedSource
    loadSpectrum()
  })

  $effect(() => {
    currentFrameIndex
    drawInstantSpectrum()
  })

  function togglePlay() {
    if (isPlaying) {
      stopPlayback()
    } else {
      startPlayback()
    }
  }

  function startPlayback() {
    if (!spectrumData || !spectrumData.frames || spectrumData.frames.length === 0) return
    isPlaying = true

    if (currentFrameIndex >= spectrumData.frames.length - 1) {
      currentFrameIndex = 0
    }

    const intervalMs = Math.max(25, Math.round(125 / playbackSpeed))
    playTimer = setInterval(() => {
      if (currentFrameIndex < spectrumData.frames.length - 1) {
        currentFrameIndex++
      } else {
        stopPlayback()
      }
    }, intervalMs)
  }

  function stopPlayback() {
    isPlaying = false
    if (playTimer) {
      clearInterval(playTimer)
      playTimer = null
    }
  }

  function toggleSpeed() {
    const speeds = [0.5, 1, 2, 4]
    const idx = speeds.indexOf(playbackSpeed)
    playbackSpeed = speeds[(idx + 1) % speeds.length]
    if (isPlaying) {
      stopPlayback()
      startPlayback()
    }
  }

  function handleSeek(e) {
    const val = parseInt(e.target.value, 10)
    currentFrameIndex = val
  }

  function stepFrame(delta) {
    if (!spectrumData?.frames) return
    const max = spectrumData.frames.length - 1
    currentFrameIndex = Math.min(Math.max(0, currentFrameIndex + delta), max)
  }

  // Waterfall Click to Seek
  function handleWaterfallClick(e) {
    if (!waterfallCanvas || !spectrumData?.frames) return
    const rect = waterfallCanvas.getBoundingClientRect()
    const y = e.clientY - rect.top
    const ratio = Math.max(0, Math.min(1, y / rect.height))
    currentFrameIndex = Math.floor(ratio * (spectrumData.frames.length - 1))
  }

  // Color mapping helper: 0.0 ~ 1.0 -> RGB
  function getHeatmapColor(val) {
    const v = Math.max(0, Math.min(1, val))
    let r = 0, g = 0, b = 0
    if (v < 0.25) {
      // Dark navy to Blue
      b = Math.floor(v * 4 * 255)
    } else if (v < 0.5) {
      // Blue to Cyan
      g = Math.floor((v - 0.25) * 4 * 255)
      b = 255
    } else if (v < 0.75) {
      // Cyan to Yellow
      r = Math.floor((v - 0.5) * 4 * 255)
      g = 255
      b = Math.floor((1 - (v - 0.5) * 4) * 255)
    } else {
      // Yellow to Red/White
      r = 255
      g = Math.floor((1 - (v - 0.75) * 4) * 255)
      b = Math.floor((v - 0.75) * 4 * 128)
    }
    return [r, g, b]
  }

  function drawWaterfall() {
    if (!waterfallCanvas || !spectrumData || !spectrumData.frames) return
    const ctx = waterfallCanvas.getContext('2d')
    const width = waterfallCanvas.width
    const height = waterfallCanvas.height
    const frames = spectrumData.frames
    const numBins = spectrumData.num_bins

    const startX = 35
    const plotWidth = width - 45

    // 1. Clear background & draw left gutter
    ctx.fillStyle = '#090d16'
    ctx.fillRect(0, 0, width, height)

    // 2. Generate Waterfall Heatmap for plot area (plotWidth x height)
    const imgData = ctx.createImageData(plotWidth, height)
    const data = imgData.data

    for (let y = 0; y < height; y++) {
      const frameIdx = Math.floor((y / height) * frames.length)
      const frame = frames[frameIdx]
      const bins = frame ? frame.bins : []

      for (let x = 0; x < plotWidth; x++) {
        const binIdx = Math.floor((x / plotWidth) * numBins)
        const val = bins[binIdx] || 0
        const [r, g, b] = getHeatmapColor(val)

        const pixelIdx = (y * plotWidth + x) * 4
        data[pixelIdx] = r
        data[pixelIdx + 1] = g
        data[pixelIdx + 2] = b
        data[pixelIdx + 3] = 255
      }
    }

    ctx.putImageData(imgData, startX, 0)

    // 3. Draw Left Time Axis Gutter & Ticks
    ctx.strokeStyle = 'rgba(51, 65, 85, 0.6)'
    ctx.lineWidth = 1
    ctx.beginPath()
    ctx.moveTo(startX, 0)
    ctx.lineTo(startX, height)
    ctx.stroke()

    ctx.fillStyle = 'rgba(148, 163, 184, 0.7)'
    ctx.font = '9px monospace'
    ctx.textAlign = 'right'
    ctx.fillText('00:00', 30, 10)
    ctx.fillText(formatTime(spectrumData.duration_sec / 2), 30, height / 2 + 3)
    ctx.fillText(formatTime(spectrumData.duration_sec), 30, height - 4)

    // 4. Draw Target Bandwidth & Frequency Guide Line on Waterfall
    const freqSpan = spectrumData.freq_end_hz - spectrumData.freq_start_hz
    if (targetFreqHz && freqSpan > 0) {
      const r = (targetFreqHz - spectrumData.freq_start_hz) / freqSpan
      if (r >= 0 && r <= 1) {
        const targetX = startX + r * plotWidth

        // Bandwidth highlight shading
        if (targetBandwidthHz > 0) {
          const bwHalf = (targetBandwidthHz / 2 / freqSpan) * plotWidth
          const leftBw = Math.max(startX, targetX - bwHalf)
          const rightBw = Math.min(startX + plotWidth, targetX + bwHalf)
          ctx.fillStyle = 'rgba(16, 185, 129, 0.16)'
          ctx.fillRect(leftBw, 0, rightBw - leftBw, height)

          // Band boundary lines
          ctx.save()
          ctx.strokeStyle = 'rgba(16, 185, 129, 0.35)'
          ctx.setLineDash([2, 2])
          ctx.beginPath()
          ctx.moveTo(leftBw, 0)
          ctx.lineTo(leftBw, height)
          ctx.moveTo(rightBw, 0)
          ctx.lineTo(rightBw, height)
          ctx.stroke()
          ctx.restore()
        }

        // Target Vertical Guideline
        ctx.save()
        ctx.strokeStyle = '#10b981'
        ctx.lineWidth = 1.5
        ctx.setLineDash([4, 3])
        ctx.beginPath()
        ctx.moveTo(targetX, 0)
        ctx.lineTo(targetX, height)
        ctx.stroke()

        // Markers at top & bottom
        ctx.fillStyle = '#10b981'
        // Top arrow (points down)
        ctx.beginPath()
        ctx.moveTo(targetX - 4, 0)
        ctx.lineTo(targetX + 4, 0)
        ctx.lineTo(targetX, 6)
        ctx.closePath()
        ctx.fill()

        // Bottom arrow (points up)
        ctx.beginPath()
        ctx.moveTo(targetX - 4, height)
        ctx.lineTo(targetX + 4, height)
        ctx.lineTo(targetX, height - 6)
        ctx.closePath()
        ctx.fill()
        ctx.restore()
      }
    }
  }

  function drawInstantSpectrum() {
    if (!spectrumCanvas || !currentFrame || !currentFrame.bins_db || !spectrumData) return
    const ctx = spectrumCanvas.getContext('2d')
    const width = spectrumCanvas.width
    const height = spectrumCanvas.height

    ctx.clearRect(0, 0, width, height)

    const plotWidth = width - 45
    const plotHeight = height - 24
    const startX = 35

    // Background Grid
    ctx.strokeStyle = 'rgba(51, 65, 85, 0.4)'
    ctx.lineWidth = 1

    // Horizontal Grid Lines (-20, -40, -60, -80 dBFS)
    const dbLevels = [0, -20, -40, -60, -80, -100]
    ctx.fillStyle = 'rgba(148, 163, 184, 0.6)'
    ctx.font = '10px monospace'
    ctx.textAlign = 'right'

    for (const db of dbLevels) {
      const y = ((0 - db) / 100) * plotHeight + 12
      ctx.beginPath()
      ctx.moveTo(startX, y)
      ctx.lineTo(width - 10, y)
      ctx.stroke()
      ctx.fillText(`${db}`, 30, y + 3)
    }

    // Vertical Frequency Grid Lines
    const numVert = 5
    for (let i = 0; i < numVert; i++) {
      const x = startX + (i / (numVert - 1)) * plotWidth
      ctx.beginPath()
      ctx.moveTo(x, 10)
      ctx.lineTo(x, height - 12)
      ctx.stroke()
    }

    // Frequency conversion helper
    const freqSpan = spectrumData.freq_end_hz - spectrumData.freq_start_hz
    const getXForFreq = (freq) => {
      if (freqSpan <= 0) return null
      const r = (freq - spectrumData.freq_start_hz) / freqSpan
      return startX + r * plotWidth
    }

    // Draw Target Bandwidth Shading if applicable
    if (targetFreqHz && targetBandwidthHz > 0 && freqSpan > 0) {
      const leftX = Math.max(startX, Math.min(startX + plotWidth, getXForFreq(targetFreqHz - targetBandwidthHz / 2)))
      const rightX = Math.max(startX, Math.min(startX + plotWidth, getXForFreq(targetFreqHz + targetBandwidthHz / 2)))
      const bwWidth = rightX - leftX
      if (bwWidth > 0) {
        ctx.fillStyle = 'rgba(16, 185, 129, 0.12)'
        ctx.fillRect(leftX, 12, bwWidth, plotHeight)
        // Band edges
        ctx.save()
        ctx.strokeStyle = 'rgba(16, 185, 129, 0.3)'
        ctx.setLineDash([2, 2])
        ctx.beginPath()
        ctx.moveTo(leftX, 12)
        ctx.lineTo(leftX, 12 + plotHeight)
        ctx.moveTo(rightX, 12)
        ctx.lineTo(rightX, 12 + plotHeight)
        ctx.stroke()
        ctx.restore()
      }
    }

    // Draw Spectrum Path
    const dbs = currentFrame.bins_db

    ctx.beginPath()
    for (let i = 0; i < dbs.length; i++) {
      const x = startX + (i / (dbs.length - 1)) * plotWidth
      const db = Math.max(-100, Math.min(0, dbs[i]))
      const y = 12 + ((0 - db) / 100) * plotHeight

      if (i === 0) {
        ctx.moveTo(x, y)
      } else {
        ctx.lineTo(x, y)
      }
    }

    // Gradient fill under spectrum
    const gradient = ctx.createLinearGradient(0, 12, 0, height - 12)
    gradient.addColorStop(0, 'rgba(6, 182, 212, 0.4)')
    gradient.addColorStop(0.7, 'rgba(6, 182, 212, 0.1)')
    gradient.addColorStop(1, 'rgba(6, 182, 212, 0.0)')

    ctx.lineTo(startX + plotWidth, 12 + plotHeight)
    ctx.lineTo(startX, 12 + plotHeight)
    ctx.closePath()
    ctx.fillStyle = gradient
    ctx.fill()

    // Stroke Spectrum Line
    ctx.beginPath()
    for (let i = 0; i < dbs.length; i++) {
      const x = startX + (i / (dbs.length - 1)) * plotWidth
      const db = Math.max(-100, Math.min(0, dbs[i]))
      const y = 12 + ((0 - db) / 100) * plotHeight

      if (i === 0) {
        ctx.moveTo(x, y)
      } else {
        ctx.lineTo(x, y)
      }
    }
    ctx.strokeStyle = '#22d3ee'
    ctx.lineWidth = 1.8
    ctx.stroke()

    // Draw Target Frequency Guideline & Marker
    if (targetFreqHz && freqSpan > 0) {
      const targetX = getXForFreq(targetFreqHz)
      if (targetX >= startX && targetX <= startX + plotWidth) {
        // Vertical dashed line
        ctx.save()
        ctx.strokeStyle = '#10b981'
        ctx.lineWidth = 1.5
        ctx.setLineDash([4, 3])
        ctx.beginPath()
        ctx.moveTo(targetX, 12)
        ctx.lineTo(targetX, 12 + plotHeight)
        ctx.stroke()
        ctx.restore()

        // Target marker on spectrum curve
        const targetDb = targetInfo.targetDb ?? -100
        const clampedDb = Math.max(-100, Math.min(0, targetDb))
        const targetY = 12 + ((0 - clampedDb) / 100) * plotHeight

        ctx.beginPath()
        ctx.arc(targetX, targetY, 4, 0, 2 * Math.PI)
        ctx.fillStyle = '#10b981'
        ctx.fill()
        ctx.strokeStyle = '#ffffff'
        ctx.lineWidth = 1.5
        ctx.stroke()

        // Top Target Pin and Badge
        ctx.save()
        ctx.fillStyle = '#10b981'
        ctx.beginPath()
        ctx.moveTo(targetX, 12)
        ctx.lineTo(targetX - 4, 6)
        ctx.lineTo(targetX + 4, 6)
        ctx.closePath()
        ctx.fill()

        ctx.font = 'bold 8.5px monospace'
        ctx.textAlign = 'center'
        const labelText = 'TARGET'
        const textWidth = ctx.measureText(labelText).width
        const badgeX = Math.max(startX + textWidth / 2 + 4, Math.min(startX + plotWidth - textWidth / 2 - 4, targetX))

        ctx.fillStyle = 'rgba(6, 78, 59, 0.95)'
        ctx.fillRect(badgeX - textWidth / 2 - 3, 0, textWidth + 6, 11)
        ctx.strokeStyle = '#10b981'
        ctx.lineWidth = 1
        ctx.strokeRect(badgeX - textWidth / 2 - 3, 0, textWidth + 6, 11)

        ctx.fillStyle = '#6ee7b7'
        ctx.fillText(labelText, badgeX, 8.5)
        ctx.restore()
      }
    }

    // Highlight Peak Point
    let maxDb = -200
    let maxIdx = 0
    for (let i = 0; i < dbs.length; i++) {
      if (dbs[i] > maxDb) {
        maxDb = dbs[i]
        maxIdx = i
      }
    }
    const peakX = startX + (maxIdx / (dbs.length - 1)) * plotWidth
    const peakY = 12 + ((0 - maxDb) / 100) * plotHeight

    ctx.beginPath()
    ctx.arc(peakX, peakY, 4, 0, 2 * Math.PI)
    ctx.fillStyle = '#f43f5e'
    ctx.fill()
    ctx.strokeStyle = '#ffffff'
    ctx.lineWidth = 1.5
    ctx.stroke()
  }

  function formatTime(sec) {
    if (isNaN(sec) || sec < 0) return '00:00'
    const m = Math.floor(sec / 60)
    const s = Math.floor(sec % 60)
    const ms = Math.floor((sec % 1) * 10)
    return `${m.toString().padStart(2, '0')}:${s.toString().padStart(2, '0')}.${ms}`
  }

  onMount(() => {
    loadSpectrum()
  })

  onDestroy(() => {
    stopPlayback()
  })
</script>

<div class="bg-slate-950/90 border border-slate-800 rounded-2xl p-5 shadow-2xl space-y-4">
  <!-- Header / Meta Controls -->
  <div class="flex flex-wrap items-center justify-between gap-3 border-b border-slate-800/80 pb-3">
    <div class="flex items-center gap-2.5">
      <div class="p-2 bg-cyan-950/80 text-cyan-400 border border-cyan-500/30 rounded-xl">
        <Activity class="w-4 h-4" />
      </div>
      <div>
        <h4 class="text-xs font-bold text-slate-100 flex items-center gap-2">
          {title || i18n.t('gallery.spectrumTitle')}
          {#if spectrumData}
            <span class="px-2 py-0.5 rounded-md text-[10px] font-mono border {spectrumData.type === 'rf_iq' ? 'bg-indigo-950/60 text-indigo-300 border-indigo-500/40' : 'bg-amber-950/60 text-amber-300 border-amber-500/40'}">
              {spectrumData.type === 'rf_iq' ? 'Raw IQ (RF Spectrum)' : 'Audio WAV (Audio Spectrum)'}
            </span>
          {/if}
        </h4>
        <p class="text-[10px] text-slate-400 font-mono mt-0.5">
          {#if spectrumData}
            {spectrumData.file_name} • {(spectrumData.sample_rate / 1e3).toFixed(1)} kHz • {spectrumData.duration_sec}s
            {#if record?.gain_auto}
              • Gain: AUTO (AGC)
            {:else if record?.gain && record.gain > 0}
              • Gain: {(record.gain / 10.0).toFixed(1)} dB
            {/if}
          {:else}
            {i18n.t('gallery.spectrumSubtitle')}
          {/if}
        </p>
      </div>
    </div>

    <!-- Source Selector & Live Stats -->
    <div class="flex items-center gap-2.5">
      {#if hasRawIQ && hasWAV}
        <div class="flex items-center gap-1 bg-slate-900 p-1 rounded-xl border border-slate-800 text-[11px]">
          <button
            type="button"
            onclick={() => selectedSource = 'raw_iq'}
            class="px-2.5 py-1 rounded-lg font-medium transition-all flex items-center gap-1.5 cursor-pointer {selectedSource === 'raw_iq' || (selectedSource === 'auto' && spectrumData?.type === 'rf_iq') ? 'bg-indigo-600 text-white' : 'text-slate-400 hover:text-slate-200'}"
          >
            <Radio class="w-3 h-3" />
            <span>Raw IQ</span>
          </button>
          <button
            type="button"
            onclick={() => selectedSource = 'audio_wav'}
            class="px-2.5 py-1 rounded-lg font-medium transition-all flex items-center gap-1.5 cursor-pointer {selectedSource === 'audio_wav' || (selectedSource === 'auto' && spectrumData?.type === 'audio_wav') ? 'bg-amber-600 text-white' : 'text-slate-400 hover:text-slate-200'}"
          >
            <Music class="w-3 h-3" />
            <span>WAV</span>
          </button>
        </div>
      {/if}

      {#if currentFrame}
        <!-- Target Badge -->
        {#if targetInfo.inBand}
          <div class="flex items-center gap-1.5 bg-emerald-950/50 px-3 py-1.5 rounded-xl border border-emerald-500/30 text-[11px] font-mono">
            <span class="w-2 h-2 rounded-full bg-emerald-400"></span>
            <span class="text-emerald-400 font-semibold">{i18n.t('gallery.targetFreq')}:</span>
            <span class="text-emerald-200 font-bold">{targetInfo.freq}</span>
            <span class="text-emerald-600/70">|</span>
            <span class="text-emerald-300 font-bold">{targetInfo.dbfs}</span>
            {#if targetInfo.delta}
              <span class="text-slate-400 text-[10px] ml-1">({i18n.t('gallery.freqDelta')}: <span class="text-amber-300">{targetInfo.delta}</span>)</span>
            {/if}
          </div>
        {/if}

        <!-- Peak Badge -->
        <div class="flex items-center gap-1.5 bg-slate-900/90 px-3 py-1.5 rounded-xl border border-slate-800 text-[11px] font-mono">
          <span class="w-2 h-2 rounded-full bg-rose-500"></span>
          <span class="text-slate-400">{i18n.t('gallery.peakFreq')}:</span>
          <span class="text-cyan-300 font-bold">{peakInfo.freq}</span>
          <span class="text-slate-600">|</span>
          <span class="text-rose-400 font-bold">{peakInfo.dbfs}</span>
        </div>
      {/if}
    </div>
  </div>

  {#if loading}
    <div class="py-16 text-center text-xs text-cyan-400 animate-pulse flex items-center justify-center gap-2">
      <Activity class="w-5 h-5 animate-spin" />
      <span>{i18n.t('gallery.loadingSpectrum')}</span>
    </div>
  {:else if errorMsg}
    <div class="py-8 px-4 bg-red-950/30 border border-red-500/30 rounded-xl text-xs text-red-300 flex items-center justify-center gap-2">
      <AlertCircle class="w-4 h-4 shrink-0 text-red-400" />
      <span>{errorMsg}</span>
    </div>
  {:else if spectrumData && spectrumData.frames && spectrumData.frames.length > 0}
    <!-- Visualizers Grid -->
    <div class="space-y-4">
      <!-- 1. Instantaneous FFT Spectrum Line Chart -->
      <div class="space-y-1.5">
        <div class="flex items-center justify-between text-[11px] text-slate-400 px-1">
          <span class="font-semibold text-slate-300 flex items-center gap-1.5">
            <Activity class="w-3.5 h-3.5 text-cyan-400" />
            {i18n.t('gallery.instantSpectrumTitle')}
            <span class="font-mono text-cyan-400 text-[10px]">({formatTime(currentFrame?.time_sec || 0)})</span>
          </span>
          <div class="flex items-center gap-3 text-[10px] font-mono">
            {#if targetInfo.inBand}
              <span class="flex items-center gap-1">
                <span class="w-2 h-2 rounded-full bg-emerald-400 inline-block"></span>
                <span class="text-emerald-300 font-semibold">{i18n.t('gallery.targetFreq')}</span>
              </span>
            {/if}
            <span class="flex items-center gap-1">
              <span class="w-2 h-2 rounded-full bg-rose-500 inline-block"></span>
              <span class="text-rose-300 font-semibold">{i18n.t('gallery.peakFreq')}</span>
            </span>
            <span class="text-slate-500">dBFS vs Frequency</span>
          </div>
        </div>

        <div class="relative bg-slate-900/90 rounded-xl border border-slate-800 overflow-hidden p-2">
          <canvas
            bind:this={spectrumCanvas}
            width="640"
            height="150"
            class="w-full h-[150px] block"
          ></canvas>

          <!-- Frequency Axis Labels at bottom -->
          {#if spectrumData.freq_labels && spectrumData.freq_labels.length > 0}
            <div class="flex justify-between text-[9px] font-mono text-slate-400 px-9 pt-1 border-t border-slate-800/60 mt-1">
              {#each spectrumData.freq_labels as lbl}
                <span>{lbl}</span>
              {/each}
            </div>
          {/if}
        </div>
      </div>

      <!-- 2. Waterfall Spectrogram Canvas -->
      <div class="space-y-1.5">
        <div class="flex items-center justify-between text-[11px] text-slate-400 px-1">
          <span class="font-semibold text-slate-300 flex items-center gap-1.5">
            <Radio class="w-3.5 h-3.5 text-indigo-400" />
            {i18n.t('gallery.waterfallTitle')}
          </span>
          <div class="flex items-center gap-3 text-[10px]">
            {#if targetInfo.inBand}
              <span class="flex items-center gap-1 text-[10px] font-mono">
                <span class="w-2 h-2 rounded-full bg-emerald-400 inline-block"></span>
                <span class="text-emerald-300 font-semibold">{i18n.t('gallery.targetFreq')} ({targetInfo.freq})</span>
              </span>
            {/if}
            <span class="text-slate-500">
              Click anywhere on waterfall to jump timeline
            </span>
          </div>
        </div>

        <div class="relative bg-slate-900/90 rounded-xl border border-slate-800 overflow-hidden p-2 cursor-crosshair">
          <canvas
            bind:this={waterfallCanvas}
            width="640"
            height="220"
            onclick={handleWaterfallClick}
            class="w-full h-[220px] block"
          ></canvas>

          <!-- Interactive Time Cursor Overlay -->
          {#if spectrumData.frames.length > 0}
            {@const cursorTopPercent = (currentFrameIndex / (spectrumData.frames.length - 1)) * 100}
            <div
              class="absolute pointer-events-none border-b-2 border-rose-500/90 shadow-[0_0_8px_rgba(244,63,94,0.8)] flex items-center justify-between px-2"
              style="top: {cursorTopPercent}%; transform: translateY(-50%); left: 8px; right: 8px;"
            >
              <span class="bg-rose-600 text-white font-mono text-[9px] font-bold px-1 py-0.2 rounded shadow">
                {formatTime(currentFrame?.time_sec || 0)}
              </span>
              <span class="bg-rose-950/80 text-rose-300 font-mono text-[9px] px-1 py-0.2 rounded border border-rose-500/30">
                Frame {currentFrameIndex + 1}/{spectrumData.frames.length}
              </span>
            </div>
          {/if}

          <!-- Frequency Axis Labels at bottom of Waterfall -->
          {#if spectrumData.freq_labels && spectrumData.freq_labels.length > 0}
            <div class="flex justify-between text-[9px] font-mono text-slate-400 px-9 pt-1 border-t border-slate-800/60 mt-1">
              {#each spectrumData.freq_labels as lbl}
                <span>{lbl}</span>
              {/each}
            </div>
          {/if}
        </div>
      </div>

      <!-- 3. Timeline Playback & Controls -->
      <div class="bg-slate-900/90 rounded-xl border border-slate-800 p-3.5 space-y-3">
        <!-- Progress / Seek Range -->
        <div class="space-y-1">
          <input
            type="range"
            min="0"
            max={spectrumData.frames.length - 1}
            step="1"
            value={currentFrameIndex}
            oninput={handleSeek}
            class="w-full h-1.5 bg-slate-800 rounded-lg appearance-none cursor-pointer accent-cyan-400 hover:accent-cyan-300 transition-all"
          />
          <div class="flex items-center justify-between text-[11px] font-mono text-slate-400">
            <span>{formatTime(currentFrame?.time_sec || 0)}</span>
            <span>{formatTime(spectrumData.duration_sec)}</span>
          </div>
        </div>

        <!-- Buttons Row -->
        <div class="flex flex-wrap items-center justify-between gap-3 pt-1">
          <div class="flex items-center gap-2">
            <button
              type="button"
              onclick={() => stepFrame(-10)}
              title="Step -10 frames"
              class="p-2 rounded-xl bg-slate-950 hover:bg-slate-800 text-slate-300 border border-slate-800 transition-all active:scale-95 cursor-pointer"
            >
              <RotateCcw class="w-3.5 h-3.5" />
            </button>

            <button
              type="button"
              onclick={togglePlay}
              title={isPlaying ? i18n.t('gallery.pauseTimeline') : i18n.t('gallery.playTimeline')}
              class="px-4 py-2 rounded-xl font-semibold text-xs transition-all flex items-center gap-2 shadow-lg active:scale-95 cursor-pointer {isPlaying ? 'bg-amber-600 hover:bg-amber-500 text-white shadow-amber-600/30' : 'bg-cyan-600 hover:bg-cyan-500 text-white shadow-cyan-600/30'}"
            >
              {#if isPlaying}
                <Pause class="w-4 h-4 fill-current" />
                <span>{i18n.t('gallery.pauseTimeline')}</span>
              {:else}
                <Play class="w-4 h-4 fill-current" />
                <span>{i18n.t('gallery.playTimeline')}</span>
              {/if}
            </button>

            <button
              type="button"
              onclick={() => stepFrame(10)}
              title="Step +10 frames"
              class="p-2 rounded-xl bg-slate-950 hover:bg-slate-800 text-slate-300 border border-slate-800 transition-all active:scale-95 cursor-pointer"
            >
              <RotateCw class="w-3.5 h-3.5" />
            </button>
          </div>

          <!-- Playback Speed -->
          <button
            type="button"
            onclick={toggleSpeed}
            class="px-3 py-1.5 bg-slate-950 hover:bg-slate-800 text-slate-300 border border-slate-800 rounded-xl text-xs font-mono font-semibold flex items-center gap-1.5 transition-all cursor-pointer"
            title="Change Playback Speed"
          >
            <FastForward class="w-3.5 h-3.5 text-cyan-400" />
            <span>{playbackSpeed}x</span>
          </button>
        </div>
      </div>
    </div>
  {/if}
</div>
