<script>
  import { onMount, onDestroy } from 'svelte'
  import { i18n } from '../i18n.svelte.js'
  import { Radio, Sliders, Play, Square, Disc, AlertCircle, Cpu, Zap } from '@lucide/svelte'

  let { initialFreq = 137900000, initialSat = 'METEOR-M2 4', onDecoded = () => {} } = $props()

  let centerFreq = $state(initialFreq)
  let selectedSatName = $state(initialSat)
  let tunerGain = $state(144)
  let autoGain = $state(false)
  let biasTee = $state(true)
  let forceSim = $state(false)
  let sampleRate = $state(2080000)

  const SATELLITE_LIST = [
    { id: 'meteor24', name: 'METEOR-M2 4', freq: 137900000, displayFreq: '137.900', active: true, mode: 'LRPT' },
    { id: 'meteor23', name: 'METEOR-M2 3', freq: 137900000, displayFreq: '137.900', active: true, mode: 'LRPT' },
    { id: 'noaa19', name: 'NOAA 19', freq: 137100000, displayFreq: '137.100', active: false, mode: 'APT (EOL)' },
    { id: 'noaa15', name: 'NOAA 15', freq: 137620000, displayFreq: '137.620', active: false, mode: 'APT (EOL)' },
    { id: 'noaa18', name: 'NOAA 18', freq: 137912500, displayFreq: '137.9125', active: false, mode: 'APT (EOL)' }
  ]

  let sdrInfo = $state({ count: 0, name: 'Scanning...', is_present: false })
  let isStreaming = $state(false)
  let isRecording = $state(false)
  let isDecoding = $state(false)
  let isAutoRecording = $state(false)
  let autoRecordSat = $state('')
  let recordStatus = $state('')
  let recordRemainingSec = $state(30)
  let recordProgress = $state(0)
  let recordTimer = null

  let canvasEl
  let ctx

  let freqMarkers = $derived.by(() => {
    const curFreq = Number(centerFreq)
    const span = sampleRate || 2080000
    const startFreq = curFreq - span / 2
    const endFreq = curFreq + span / 2

    // Check if the current center frequency matches one of the NOAA satellite frequencies
    const isSatTuned = SATELLITE_LIST.some(s => s.freq === curFreq)

    if (isSatTuned) {
      // Show unique satellite frequencies that fall within the current bandwidth
      const markers = []
      const seenFreqs = new Set()
      for (const sat of SATELLITE_LIST) {
        if (sat.freq >= startFreq && sat.freq <= endFreq) {
          if (seenFreqs.has(sat.freq)) continue
          seenFreqs.add(sat.freq)
          const ratio = (sat.freq - startFreq) / span
          const isPrimary = sat.freq === curFreq
          const labelName = sat.freq === 137900000 ? 'METEOR-M2 3/4' : sat.name
          markers.push({
            name: labelName,
            freq: sat.freq,
            displayFreq: `${sat.displayFreq} MHz`,
            ratio: ratio,
            percent: ratio * 100,
            isPrimary: isPrimary,
            color: isPrimary ? '#10b981' : '#38bdf8',
            badgeBg: isPrimary
              ? 'bg-emerald-950/90 border-emerald-500 text-emerald-300'
              : 'bg-sky-950/90 border-sky-500 text-sky-300'
          })
        }
      }
      return markers
    } else {
      // Custom frequency: only show the center reference frequency line
      return [{
        name: i18n.t('sdr.centerFreqMarker') || 'Center',
        freq: curFreq,
        displayFreq: `${(curFreq / 1e6).toFixed(4)} MHz`,
        ratio: 0.5,
        percent: 50,
        isPrimary: true,
        color: '#10b981',
        badgeBg: 'bg-emerald-950/90 border-emerald-500 text-emerald-300'
      }]
    }
  })

  function getDeviceDisplayName(info) {
    if (!info) return ''
    if (info.count > 0) {
      return i18n.t('sdr.deviceFound').replace('{id}', '0').replace('{count}', info.count)
    }
    return i18n.t('sdr.noHardware')
  }

  async function loadSDRInfo() {
    if (window.go?.main?.App) {
      try {
        sdrInfo = await window.go.main.App.GetSDRDeviceInfo()
      } catch (err) {
        console.error("Failed to load SDR info:", err)
      }
    }
  }

  async function loadConfig() {
    if (window.go?.main?.App) {
      try {
        const cfg = await window.go.main.App.GetConfig()
        if (cfg) {
          if (cfg.tuner_gain !== undefined) tunerGain = cfg.tuner_gain
          if (cfg.tuner_gain_auto !== undefined) autoGain = cfg.tuner_gain_auto
          if (cfg.bias_tee !== undefined) biasTee = cfg.bias_tee
          if (cfg.sample_rate) sampleRate = cfg.sample_rate
        }
      } catch (err) {
        console.error("Failed to load config:", err)
      }
    }
  }

  async function checkInitialRecordingStatus() {
    if (window.go?.main?.App?.IsRecording) {
      try {
        const rec = await window.go.main.App.IsRecording()
        if (rec) {
          isRecording = true
          recordStatus = i18n.t('sdr.recording')
        }
      } catch (err) {
        console.error("Failed to check initial recording status:", err)
      }
    }
  }

  async function handleGainChange() {
    if (window.go?.main?.App) {
      try {
        await window.go.main.App.SetSDRGain(Number(tunerGain), autoGain)
      } catch (err) {
        console.error("Failed to set SDR gain:", err)
      }
    }
  }

  async function handleFreqChange() {
    if (window.go?.main?.App && isStreaming) {
      try {
        await window.go.main.App.SetSDRCenterFreq(Number(centerFreq))
      } catch (err) {
        console.error("Failed to set SDR frequency:", err)
      }
    }
  }

  async function handleBiasTeeChange() {
    if (window.go?.main?.App) {
      try {
        await window.go.main.App.SetSDRBiasTee(biasTee)
      } catch (err) {
        console.error("Failed to set SDR Bias-Tee:", err)
      }
    }
  }

  async function toggleStream() {
    if (!window.go?.main?.App) return
    if (isStreaming) {
      await window.go.main.App.StopSDR()
      isStreaming = false
    } else {
      const res = await window.go.main.App.StartSDR(centerFreq, 2080000, tunerGain, autoGain, forceSim)
      isStreaming = true
    }
  }

  function clearRecordTimer() {
    if (recordTimer) {
      clearInterval(recordTimer)
      recordTimer = null
    }
  }

  function startAutoRecordTimer(satName, duration) {
    clearRecordTimer()
    isAutoRecording = true
    autoRecordSat = satName
    isRecording = true
    isDecoding = false
    recordRemainingSec = duration
    recordProgress = 0
    recordStatus = i18n.t('sdr.autoRecordingInProgress').replace('{sat}', satName).replace('{seconds}', duration.toString())

    const startTime = Date.now()
    recordTimer = setInterval(() => {
      const elapsed = (Date.now() - startTime) / 1000
      const remaining = Math.max(0, Math.ceil(duration - elapsed))
      recordRemainingSec = remaining
      recordProgress = Math.min(100, Math.round((elapsed / duration) * 100))

      if (remaining > 0) {
        recordStatus = i18n.t('sdr.autoRecordingInProgress').replace('{sat}', satName).replace('{seconds}', remaining.toString())
      } else {
        clearRecordTimer()
        isDecoding = true
        recordProgress = 100
        recordStatus = i18n.t('sdr.decodingProgress')
      }
    }, 250)
  }

  async function startRecord() {
    if (!window.go?.main?.App) return
    if (isRecording || isDecoding || isAutoRecording) return
    clearRecordTimer()

    const totalDuration = 30
    isRecording = true
    isDecoding = false
    recordRemainingSec = totalDuration
    recordProgress = 0
    recordStatus = i18n.t('sdr.recordingCountdown').replace('{seconds}', totalDuration.toString())

    const startTime = Date.now()
    recordTimer = setInterval(() => {
      const elapsed = (Date.now() - startTime) / 1000
      const remaining = Math.max(0, Math.ceil(totalDuration - elapsed))
      recordRemainingSec = remaining
      recordProgress = Math.min(100, Math.round((elapsed / totalDuration) * 100))

      if (remaining > 0) {
        recordStatus = i18n.t('sdr.recordingCountdown').replace('{seconds}', remaining.toString())
      } else {
        clearRecordTimer()
        isDecoding = true
        recordProgress = 100
        recordStatus = i18n.t('sdr.decodingProgress')
      }
    }, 250)

    try {
      let satName = selectedSatName
      if (!satName) {
        satName = centerFreq === 137900000 ? 'METEOR-M2 4' : (centerFreq === 137620000 ? 'NOAA 15' : (centerFreq === 137912500 ? 'NOAA 18' : 'NOAA 19'))
      }
      const err = await window.go.main.App.StartRecordingAndDecode(satName, centerFreq, totalDuration, Number(tunerGain), autoGain)
      if (err) {
        throw new Error(err)
      }
    } catch (err) {
      clearRecordTimer()
      isRecording = false
      isDecoding = false
      recordStatus = i18n.t('sdr.decodeFailed') + (err?.message || err)
    }
  }

  function tunePreset(freq, satName = '') {
    centerFreq = freq
    if (satName) selectedSatName = satName
    if (isStreaming) {
      handleFreqChange()
    }
  }

  onMount(() => {
    loadSDRInfo()
    loadConfig()
    checkInitialRecordingStatus()

    if (canvasEl) {
      ctx = canvasEl.getContext('2d')
    }

    if (window.runtime?.EventsOn) {
      window.runtime.EventsOn('sdr_spectrum', (bins) => {
        if (isStreaming) {
          drawWaterfall(bins)
        }
      })

      window.runtime.EventsOn('auto_record_started', (data) => {
        startAutoRecordTimer(data?.sat_name || 'NOAA', data?.duration || 30)
      })

      window.runtime.EventsOn('auto_record_ended', () => {
        clearRecordTimer()
        isAutoRecording = false
        isRecording = false
        isDecoding = false
      })

      window.runtime.EventsOn('decode_complete', (record) => {
        clearRecordTimer()
        isAutoRecording = false
        isRecording = false
        isDecoding = false
        recordProgress = 100
        recordStatus = i18n.t('sdr.decodeSuccess')
        onDecoded(record)
      })

      window.runtime.EventsOn('decode_error', (errMsg) => {
        clearRecordTimer()
        isAutoRecording = false
        isRecording = false
        isDecoding = false
        recordStatus = i18n.t('sdr.decodeFailed') + errMsg
      })
    }
  })

  onDestroy(() => {
    clearRecordTimer()
    if (window.runtime?.EventsOff) {
      window.runtime.EventsOff('sdr_spectrum')
      window.runtime.EventsOff('auto_record_started')
      window.runtime.EventsOff('auto_record_ended')
      window.runtime.EventsOff('decode_complete')
      window.runtime.EventsOff('decode_error')
    }
    if (isStreaming && window.go?.main?.App) {
      window.go.main.App.StopSDR()
    }
  })

  function drawWaterfall(bins) {
    if (!ctx || !canvasEl) return
    const width = canvasEl.width
    const height = canvasEl.height

    const imgData = ctx.getImageData(0, 0, width, height - 1)
    ctx.putImageData(imgData, 0, 1)

    const binWidth = width / bins.length
    for (let i = 0; i < bins.length; i++) {
      const val = Math.floor(bins[i] * 255)
      let r = 0, g = 0, b = 0
      if (val < 64) {
        b = val * 4
      } else if (val < 128) {
        g = (val - 64) * 4
        b = 255
      } else if (val < 192) {
        r = (val - 128) * 4
        g = 255
      } else {
        r = 255
        g = 255 - (val - 192) * 4
      }

      ctx.fillStyle = `rgb(${r},${g},${b})`
      ctx.fillRect(i * binWidth, 0, binWidth + 1, 1)
    }
  }
</script>

<div class="p-6 space-y-6 overflow-y-auto max-h-[calc(100vh-65px)]">
  <!-- Hardware Status Banner -->
  <div class="bg-slate-900/90 border border-slate-800 rounded-2xl p-4 flex items-center justify-between shadow-lg">
    <div class="flex items-center gap-3">
      <div class="p-2.5 rounded-xl {sdrInfo.is_present && !forceSim ? 'bg-emerald-500/10 text-emerald-400 border border-emerald-500/30' : 'bg-amber-500/10 text-amber-400 border border-amber-500/30'}">
        <Cpu class="w-5 h-5" />
      </div>
      <div>
        <h3 class="text-sm font-bold text-slate-100">{getDeviceDisplayName(sdrInfo)}</h3>
        <p class="text-xs text-slate-400 font-mono">
          {#if sdrInfo.is_present && !forceSim}
            {i18n.t('sdr.hwActive')}
          {:else}
            {i18n.t('sdr.simActive')}
          {/if}
        </p>
      </div>
    </div>

    <div class="flex items-center gap-3">
      <label class="flex items-center gap-2 text-xs font-semibold {biasTee ? 'text-amber-300 font-bold' : 'text-slate-400'} cursor-pointer bg-slate-950 px-3 py-1.5 rounded-xl border {biasTee ? 'border-amber-500/50 bg-amber-500/10' : 'border-slate-800'} transition-all">
        <input type="checkbox" bind:checked={biasTee} onchange={handleBiasTeeChange} class="rounded bg-slate-800 border-slate-700 text-amber-500 focus:ring-amber-500" />
        <span class="flex items-center gap-1">
          <Zap class="w-3.5 h-3.5 {biasTee ? 'text-amber-400 fill-amber-400' : 'text-slate-500'}" />
          {i18n.t('sdr.biasTee')}
        </span>
      </label>

      <label class="flex items-center gap-2 text-xs font-semibold text-slate-300 cursor-pointer">
        <input type="checkbox" bind:checked={forceSim} class="rounded bg-slate-800 border-slate-700 text-cyan-500 focus:ring-cyan-500" />
        {i18n.t('sdr.forceSim')}
      </label>

      <button
        onclick={toggleStream}
        class="px-5 py-2 rounded-xl text-xs font-bold flex items-center gap-2 transition-all shadow-md {isStreaming ? 'bg-rose-600 hover:bg-rose-500 text-white shadow-rose-600/30' : 'bg-emerald-600 hover:bg-emerald-500 text-white shadow-emerald-600/30'}"
      >
        {#if isStreaming}
          <Square class="w-3.5 h-3.5 fill-current" />
          {i18n.t('sdr.stopStream')}
        {:else}
          <Play class="w-3.5 h-3.5 fill-current" />
          {i18n.t('sdr.startStream')}
        {/if}
      </button>
    </div>
  </div>

  <div class="grid grid-cols-1 lg:grid-cols-3 gap-6">
    <!-- SDR Controls Panel -->
    <div class="bg-slate-900/90 border border-slate-800 rounded-2xl p-5 shadow-lg space-y-5">
      <h3 class="text-base font-bold text-slate-100 flex items-center gap-2 border-b border-slate-800 pb-3">
        <Sliders class="w-4 h-4 text-cyan-400" />
        {i18n.t('sdr.tuningTitle')}
      </h3>

      <!-- Preset Buttons -->
      <div>
        <span class="text-xs font-semibold text-slate-400 uppercase tracking-wider block mb-2">{i18n.t('sdr.presets')}</span>
        <div class="grid grid-cols-2 gap-2 mb-2">
          <button
            onclick={() => tunePreset(137900000, 'METEOR-M2 3')}
            class="py-2 px-3 rounded-xl text-xs font-bold border transition-all {centerFreq === 137900000 && selectedSatName === 'METEOR-M2 3' ? 'bg-emerald-600 border-emerald-500 text-white' : 'bg-slate-950 border-emerald-800/60 text-slate-200 hover:border-emerald-600'}"
          >
            METEOR-M2 3
            <span class="block text-[10px] font-mono text-emerald-300">137.900 (LRPT)</span>
          </button>

          <button
            onclick={() => tunePreset(137900000, 'METEOR-M2 4')}
            class="py-2 px-3 rounded-xl text-xs font-bold border transition-all {centerFreq === 137900000 && selectedSatName !== 'METEOR-M2 3' ? 'bg-emerald-600 border-emerald-500 text-white' : 'bg-slate-950 border-emerald-800/60 text-slate-200 hover:border-emerald-600'}"
          >
            METEOR-M2 4
            <span class="block text-[10px] font-mono text-emerald-300">137.900 (LRPT)</span>
          </button>
        </div>
        <div class="grid grid-cols-3 gap-2">
          <button
            onclick={() => tunePreset(137620000, 'NOAA 15')}
            class="py-1.5 px-2 rounded-xl text-[11px] font-bold border transition-all opacity-75 {centerFreq === 137620000 ? 'bg-cyan-600 border-cyan-500 text-white opacity-100' : 'bg-slate-950 border-slate-800 text-slate-400 hover:border-slate-700'}"
          >
            NOAA 15 (停波)
            <span class="block text-[9px] font-mono text-cyan-300">137.620</span>
          </button>

          <button
            onclick={() => tunePreset(137912500, 'NOAA 18')}
            class="py-1.5 px-2 rounded-xl text-[11px] font-bold border transition-all opacity-75 {centerFreq === 137912500 ? 'bg-cyan-600 border-cyan-500 text-white opacity-100' : 'bg-slate-950 border-slate-800 text-slate-400 hover:border-slate-700'}"
          >
            NOAA 18 (停波)
            <span class="block text-[9px] font-mono text-cyan-300">137.9125</span>
          </button>

          <button
            onclick={() => tunePreset(137100000, 'NOAA 19')}
            class="py-1.5 px-2 rounded-xl text-[11px] font-bold border transition-all opacity-75 {centerFreq === 137100000 ? 'bg-cyan-600 border-cyan-500 text-white opacity-100' : 'bg-slate-950 border-slate-800 text-slate-400 hover:border-slate-700'}"
          >
            NOAA 19 (停波)
            <span class="block text-[9px] font-mono text-cyan-300">137.100</span>
          </button>
        </div>
      </div>

      <!-- Frequency Input -->
      <div>
        <label for="freq-input" class="text-xs font-semibold text-slate-400 uppercase tracking-wider block mb-1">{i18n.t('sdr.centerFreq')}</label>
        <input
          id="freq-input"
          type="number"
          bind:value={centerFreq}
          oninput={handleFreqChange}
          onchange={handleFreqChange}
          step="1000"
          class="w-full bg-slate-950 border border-slate-800 rounded-xl px-3.5 py-2 text-sm font-mono text-cyan-400 font-bold focus:outline-none focus:border-cyan-500"
        />
      </div>

      <!-- Gain Slider & Auto Gain -->
      <div class="space-y-2">
        <div class="flex items-center justify-between">
          <label for="gain-slider" class="text-xs font-semibold text-slate-400 uppercase tracking-wider">{i18n.t('sdr.gain')}</label>
          <div class="flex items-center gap-3">
            <label class="flex items-center gap-1.5 text-xs text-slate-300 font-semibold cursor-pointer select-none">
              <input
                type="checkbox"
                bind:checked={autoGain}
                onchange={handleGainChange}
                class="rounded bg-slate-950 border-slate-700 text-cyan-500 focus:ring-cyan-500"
              />
              <span class="text-[11px] text-cyan-300">{i18n.t('sdr.autoGain')}</span>
            </label>
            <span class="text-xs font-mono text-cyan-400 font-bold min-w-[55px] text-right">
              {autoGain ? i18n.t('sdr.autoGainBadge') : (tunerGain / 10).toFixed(1) + ' dB'}
            </span>
          </div>
        </div>
        <input
          id="gain-slider"
          type="range"
          min="0"
          max="500"
          step="10"
          bind:value={tunerGain}
          oninput={handleGainChange}
          disabled={autoGain}
          class="w-full accent-cyan-500 bg-slate-950 rounded-lg h-2 cursor-pointer disabled:opacity-40 disabled:cursor-not-allowed"
        />
      </div>

      <!-- Action Record Button & Progress -->
      <div class="pt-2 space-y-3">
        <button
          onclick={startRecord}
          disabled={isRecording || isDecoding || isAutoRecording}
          class="w-full py-3 bg-gradient-to-r from-cyan-600 to-blue-600 hover:from-cyan-500 hover:to-blue-500 text-white rounded-xl text-xs font-bold shadow-lg shadow-cyan-600/30 flex items-center justify-center gap-2 transition-all disabled:opacity-50 disabled:cursor-not-allowed"
        >
          <Disc class="w-4 h-4 {isRecording || isAutoRecording ? 'animate-spin text-amber-300' : ''}" />
          {#if isAutoRecording}
            {i18n.t('sdr.autoRecordingInProgress').replace('{sat}', autoRecordSat).replace('{seconds}', recordRemainingSec.toString())}
          {:else if isRecording}
            {i18n.t('sdr.recording')}
          {:else}
            {i18n.t('sdr.startRecord')}
          {/if}
        </button>

        {#if isRecording || isAutoRecording}
          <div class="bg-slate-950/80 p-3 rounded-xl border border-cyan-800/40 space-y-2">
            <div class="flex items-center justify-between text-xs">
              <span class="font-medium text-cyan-300 flex items-center gap-2">
                <span class="relative flex h-2 w-2">
                  <span class="animate-ping absolute inline-flex h-full w-full rounded-full bg-rose-400 opacity-75"></span>
                  <span class="relative inline-flex rounded-full h-2 w-2 bg-rose-500"></span>
                </span>
                {#if isDecoding}
                  {i18n.t('sdr.decodingProgress')}
                {:else if isAutoRecording}
                  {i18n.t('sdr.autoRecordingInProgress').replace('{sat}', autoRecordSat).replace('{seconds}', recordRemainingSec.toString())}
                {:else}
                  {i18n.t('sdr.recordingCountdown').replace('{seconds}', recordRemainingSec.toString())}
                {/if}
              </span>
              <span class="font-mono text-cyan-400 font-bold">{recordProgress}%</span>
            </div>
            <!-- Progress Bar -->
            <div class="w-full bg-slate-800/80 rounded-full h-2 overflow-hidden border border-slate-700/50">
              <div
                class="h-full bg-gradient-to-r from-cyan-500 via-blue-500 to-emerald-400 transition-all duration-200"
                style="width: {recordProgress}%"
              ></div>
            </div>
          </div>
        {:else if recordStatus}
          <p class="text-xs font-medium text-cyan-300 text-center bg-cyan-950/40 p-2.5 rounded-xl border border-cyan-800/40">
            {recordStatus}
          </p>
        {/if}
      </div>
    </div>

    <!-- Waterfall Canvas -->
    <div class="lg:col-span-2 bg-slate-900/90 border border-slate-800 rounded-2xl p-5 shadow-lg flex flex-col justify-between">
      <div class="flex items-center justify-between mb-3">
        <h3 class="text-base font-bold text-slate-100 flex items-center gap-2">
          <Radio class="w-4 h-4 text-cyan-400" />
          {i18n.t('sdr.waterfall')}
        </h3>

        <div class="flex items-center gap-3 text-xs font-mono text-slate-400">
          <div class="hidden sm:flex items-center gap-2 text-[10px]">
            {#each freqMarkers as marker}
              <span class="flex items-center gap-1">
                <span class="w-2 h-2 rounded-full {marker.isPrimary ? 'bg-emerald-400' : 'bg-sky-400'}"></span>
                <span class="{marker.isPrimary ? 'text-emerald-300 font-bold' : 'text-sky-300'}">{marker.name}</span>
              </span>
            {/each}
          </div>
          <span class="text-slate-300 font-semibold">{(centerFreq / 1000000).toFixed(3)} MHz 帯</span>
          <span class="w-2 h-2 rounded-full {isStreaming ? 'bg-emerald-400 animate-ping' : 'bg-slate-600'}"></span>
        </div>
      </div>

      <div class="relative w-full h-[340px] bg-slate-950 rounded-xl overflow-hidden border border-slate-800">
        <canvas
          bind:this={canvasEl}
          width="600"
          height="340"
          class="w-full h-full object-cover block"
        ></canvas>

        <!-- Frequency Guideline Overlay Layer -->
        <div class="absolute inset-0 pointer-events-none overflow-hidden select-none">
          {#each freqMarkers as marker}
            <!-- Bandwidth Shading for NOAA / METEOR satellites -->
            {#if marker.name.startsWith('NOAA') || marker.name.startsWith('METEOR')}
              {@const bwHalfPercent = ((marker.name.startsWith('METEOR') ? 40000 : 20000) / sampleRate) * 100}
              <div
                class="absolute top-0 bottom-0 pointer-events-none transition-all duration-300"
                style="left: {marker.percent - bwHalfPercent}%; width: {bwHalfPercent * 2}%; background-color: {marker.isPrimary ? 'rgba(16, 185, 129, 0.14)' : 'rgba(56, 189, 248, 0.08)'}; border-left: 1px dashed {marker.isPrimary ? 'rgba(16, 185, 129, 0.35)' : 'rgba(56, 189, 248, 0.25)'}; border-right: 1px dashed {marker.isPrimary ? 'rgba(16, 185, 129, 0.35)' : 'rgba(56, 189, 248, 0.25)'};"
              ></div>
            {/if}

            <!-- Vertical Guideline -->
            <div
              class="absolute top-0 bottom-0 pointer-events-none -translate-x-1/2 border-l transition-all duration-300"
              style="left: {marker.percent}%; border-color: {marker.color}; border-left-style: dashed; border-left-width: {marker.isPrimary ? '1.5px' : '1px'}; opacity: {marker.isPrimary ? '0.95' : '0.8'};"
            ></div>

            <!-- Top Marker Badge -->
            <div
              class="absolute top-1.5 -translate-x-1/2 flex flex-col items-center pointer-events-none z-10 transition-all duration-300"
              style="left: {marker.percent}%;"
            >
              <span class="px-2 py-0.5 rounded-md text-[9px] font-mono font-bold border shadow-lg flex items-center gap-1 backdrop-blur-md {marker.badgeBg}">
                <span class="w-1.5 h-1.5 rounded-full {marker.isPrimary ? 'bg-emerald-400' : 'bg-sky-400'}"></span>
                <span>{marker.name}</span>
                <span class="opacity-80 font-normal">({marker.displayFreq})</span>
              </span>
              <!-- Down Arrow Pin -->
              <span
                class="w-0 h-0 border-l-[4px] border-l-transparent border-r-[4px] border-r-transparent border-t-[5px]"
                style="border-top-color: {marker.color};"
              ></span>
            </div>

            <!-- Bottom Marker Pin -->
            <div
              class="absolute bottom-1 -translate-x-1/2 flex flex-col items-center pointer-events-none z-10 transition-all duration-300"
              style="left: {marker.percent}%;"
            >
              <!-- Up Arrow Pin -->
              <span
                class="w-0 h-0 border-l-[4px] border-l-transparent border-r-[4px] border-r-transparent border-b-[5px]"
                style="border-bottom-color: {marker.color};"
              ></span>
            </div>
          {/each}
        </div>

        {#if !isStreaming}
          <div class="absolute inset-0 bg-slate-950/80 backdrop-blur-sm flex flex-col items-center justify-center p-6 text-center z-20">
            <Radio class="w-12 h-12 text-slate-600 mb-3" />
            <p class="text-sm font-semibold text-slate-300">{i18n.t('sdr.streamPaused')}</p>
            <p class="text-xs text-slate-500 mt-1">{i18n.t('sdr.streamPausedHint')}</p>
          </div>
        {/if}
      </div>

      <!-- Frequency Axis Scale at bottom -->
      <div class="flex justify-between text-[10px] font-mono text-slate-400 px-2 pt-2 border-t border-slate-800/80 mt-2">
        <span>{((centerFreq - sampleRate / 2) / 1e6).toFixed(3)} MHz</span>
        <span class="text-cyan-400 font-semibold">{i18n.t('sdr.centerFreq')}: {(centerFreq / 1e6).toFixed(4)} MHz</span>
        <span>{((centerFreq + sampleRate / 2) / 1e6).toFixed(3)} MHz</span>
      </div>
    </div>
  </div>
</div>

