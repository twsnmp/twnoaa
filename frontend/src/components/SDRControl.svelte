<script>
  import { onMount, onDestroy } from 'svelte'
  import { i18n } from '../i18n.svelte.js'
  import { Radio, Sliders, Play, Square, Disc, AlertCircle, Cpu } from '@lucide/svelte'

  let { initialFreq = 137100000, initialSat = 'NOAA 19', onDecoded = () => {} } = $props()

  let centerFreq = $state(initialFreq)
  let tunerGain = $state(400)
  let autoGain = $state(false)
  let forceSim = $state(false)

  let sdrInfo = $state({ count: 0, name: 'Scanning...', is_present: false })
  let isStreaming = $state(false)
  let isRecording = $state(false)
  let recordStatus = $state('')

  let canvasEl
  let ctx

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
        }
      } catch (err) {
        console.error("Failed to load config:", err)
      }
    }
  }

  async function handleGainChange() {
    if (isStreaming && window.go?.main?.App) {
      try {
        await window.go.main.App.SetSDRGain(Number(tunerGain), autoGain)
      } catch (err) {
        console.error("Failed to set SDR gain:", err)
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

  async function startRecord() {
    if (!window.go?.main?.App) return
    isRecording = true
    recordStatus = i18n.t('sdr.recordingProgress')

    try {
      const satName = centerFreq === 137620000 ? 'NOAA 15' : (centerFreq === 137912500 ? 'NOAA 18' : 'NOAA 19')
      const msg = await window.go.main.App.StartRecordingAndDecode(satName, centerFreq, 30)
      recordStatus = msg
    } catch (err) {
      recordStatus = i18n.t('sdr.decodeFailed') + err
      isRecording = false
    }
  }

  function tunePreset(freq) {
    centerFreq = freq
    if (isStreaming) {
      toggleStream().then(() => toggleStream())
    }
  }

  onMount(() => {
    loadSDRInfo()
    loadConfig()

    if (canvasEl) {
      ctx = canvasEl.getContext('2d')
    }

    if (window.runtime?.EventsOn) {
      window.runtime.EventsOn('sdr_spectrum', (bins) => {
        drawWaterfall(bins)
      })

      window.runtime.EventsOn('decode_complete', (record) => {
        isRecording = false
        recordStatus = i18n.t('sdr.decodeSuccess')
        onDecoded(record)
      })

      window.runtime.EventsOn('decode_error', (errMsg) => {
        isRecording = false
        recordStatus = i18n.t('sdr.decodeFailed') + errMsg
      })
    }
  })

  onDestroy(() => {
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
        <div class="grid grid-cols-3 gap-2">
          <button
            onclick={() => tunePreset(137620000)}
            class="py-2 px-3 rounded-xl text-xs font-bold border transition-all {centerFreq === 137620000 ? 'bg-cyan-600 border-cyan-500 text-white' : 'bg-slate-950 border-slate-800 text-slate-300 hover:border-slate-700'}"
          >
            NOAA 15
            <span class="block text-[10px] font-mono text-cyan-300">137.620</span>
          </button>

          <button
            onclick={() => tunePreset(137912500)}
            class="py-2 px-3 rounded-xl text-xs font-bold border transition-all {centerFreq === 137912500 ? 'bg-cyan-600 border-cyan-500 text-white' : 'bg-slate-950 border-slate-800 text-slate-300 hover:border-slate-700'}"
          >
            NOAA 18
            <span class="block text-[10px] font-mono text-cyan-300">137.9125</span>
          </button>

          <button
            onclick={() => tunePreset(137100000)}
            class="py-2 px-3 rounded-xl text-xs font-bold border transition-all {centerFreq === 137100000 ? 'bg-cyan-600 border-cyan-500 text-white' : 'bg-slate-950 border-slate-800 text-slate-300 hover:border-slate-700'}"
          >
            NOAA 19
            <span class="block text-[10px] font-mono text-cyan-300">137.100</span>
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
          step="1000"
          class="w-full bg-slate-950 border border-slate-800 rounded-xl px-3.5 py-2 text-sm font-mono text-cyan-400 font-bold focus:outline-none focus:border-cyan-500"
        />
      </div>

      <!-- Gain Slider -->
      <div>
        <div class="flex items-center justify-between mb-1">
          <label for="gain-slider" class="text-xs font-semibold text-slate-400 uppercase tracking-wider">{i18n.t('sdr.gain')}</label>
          <span class="text-xs font-mono text-cyan-400 font-bold">{(tunerGain / 10).toFixed(1)} dB</span>
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
          class="w-full accent-cyan-500 bg-slate-950 rounded-lg h-2 cursor-pointer"
        />
      </div>

      <!-- Action Record Button -->
      <div class="pt-2">
        <button
          onclick={startRecord}
          disabled={isRecording}
          class="w-full py-3 bg-gradient-to-r from-cyan-600 to-blue-600 hover:from-cyan-500 hover:to-blue-500 text-white rounded-xl text-xs font-bold shadow-lg shadow-cyan-600/30 flex items-center justify-center gap-2 transition-all disabled:opacity-50"
        >
          <Disc class="w-4 h-4 {isRecording ? 'animate-spin text-amber-300' : ''}" />
          {i18n.t('sdr.startRecord')}
        </button>

        {#if recordStatus}
          <p class="text-xs font-medium text-cyan-300 mt-2 text-center bg-cyan-950/40 p-2 rounded-lg border border-cyan-800/40">
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

        <div class="flex items-center gap-2 text-xs font-mono text-slate-400">
          <span>{(centerFreq / 1000000).toFixed(3)} MHz 帯</span>
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

        {#if !isStreaming}
          <div class="absolute inset-0 bg-slate-950/80 backdrop-blur-sm flex flex-col items-center justify-center p-6 text-center">
            <Radio class="w-12 h-12 text-slate-600 mb-3" />
            <p class="text-sm font-semibold text-slate-300">{i18n.t('sdr.streamPaused')}</p>
            <p class="text-xs text-slate-500 mt-1">{i18n.t('sdr.streamPausedHint')}</p>
          </div>
        {/if}
      </div>
    </div>
  </div>
</div>

