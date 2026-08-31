<script>
  import { onMount } from 'svelte'
  import { i18n } from '../i18n.svelte.js'
  import { Image, Bot, Sparkles, Trash2, AlertTriangle, X, Radio, Activity, Upload, Music, Volume2, PlayCircle, Layers, CheckSquare, Square, Filter } from '@lucide/svelte'
  import SignalChart from './SignalChart.svelte'
  import MultiSignalChart from './MultiSignalChart.svelte'
  import AudioPlayer from './AudioPlayer.svelte'
  import SpectrumViewer from './SpectrumViewer.svelte'

  let { onRunAI = () => {} } = $props()

  let records = $state([])
  let selectedRecord = $state(null)
  let selectedRecordIds = $state([])
  let activeChannel = $state('full')
  let currentImageBase64 = $state('')
  let recordToDelete = $state(null)
  let viewMode = $state('stretch') // 'stretch' | 'fit'
  let showZoomModal = $state(false)
  let detailViewTab = $state('image') // 'image' | 'signal' | 'compare' | 'audio' | 'spectrum'
  let decodingWav = $state(false)
  let wavStatusMsg = $state('')
  let standaloneWavPath = $state('')
  let showStandaloneAudioModal = $state(false)
  let standaloneSpectrumPath = $state('')
  let showStandaloneSpectrumModal = $state(false)

  let selectedRecordsList = $derived(
    records.filter(r => selectedRecordIds.includes(r.id))
  )

  let uniqueSatsWithData = $derived(
    [...new Set(records.filter(r => r.signal_history && r.signal_history.length > 0).map(r => r.sat_name))]
  )

  function toggleRecordSelection(id, e) {
    if (e) e.stopPropagation()
    if (selectedRecordIds.includes(id)) {
      selectedRecordIds = selectedRecordIds.filter(x => x !== id)
    } else {
      selectedRecordIds = [...selectedRecordIds, id]
    }
  }

  function handleDeselectRecord(id) {
    selectedRecordIds = selectedRecordIds.filter(x => x !== id)
  }

  function selectRecent5() {
    const withData = records.filter(r => r.signal_history && r.signal_history.length > 0)
    selectedRecordIds = withData.slice(0, 5).map(r => r.id)
    detailViewTab = 'compare'
  }

  function selectHighElevation() {
    const withData = records.filter(r => r.signal_history && r.signal_history.length > 0 && r.max_elevation >= 45)
    selectedRecordIds = withData.map(r => r.id)
    detailViewTab = 'compare'
  }

  function selectBySat(satName) {
    const withData = records.filter(r => r.sat_name === satName && r.signal_history && r.signal_history.length > 0)
    selectedRecordIds = withData.map(r => r.id)
    detailViewTab = 'compare'
  }

  function selectAllWithData() {
    const withData = records.filter(r => r.signal_history && r.signal_history.length > 0)
    selectedRecordIds = withData.map(r => r.id)
    detailViewTab = 'compare'
  }

  function clearSelection() {
    selectedRecordIds = []
  }

  async function handleOpenAndPlayWAV() {
    if (window.go?.main?.App?.SelectWAVFile) {
      try {
        const path = await window.go.main.App.SelectWAVFile()
        if (path) {
          standaloneWavPath = path
          showStandaloneAudioModal = true
        }
      } catch (err) {
        console.error("Failed to select WAV file:", err)
      }
    }
  }

  async function handleOpenSpectrumFile() {
    if (window.go?.main?.App?.SelectSpectrumFile) {
      try {
        const path = await window.go.main.App.SelectSpectrumFile()
        if (path) {
          standaloneSpectrumPath = path
          showStandaloneSpectrumModal = true
        }
      } catch (err) {
        console.error("Failed to select spectrum file:", err)
      }
    }
  }

  async function handleDecodeWAV() {
    if (window.go?.main?.App?.SelectAndDecodeWAV) {
      try {
        decodingWav = true
        wavStatusMsg = i18n.t('gallery.decodingWav')
        const msg = await window.go.main.App.SelectAndDecodeWAV()
        wavStatusMsg = msg
        setTimeout(() => {
          decodingWav = false
          wavStatusMsg = ''
        }, 4000)
      } catch (err) {
        console.error("Failed to decode WAV file:", err)
        wavStatusMsg = "Error: " + (err.message || err)
        setTimeout(() => {
          decodingWav = false
          wavStatusMsg = ''
        }, 4000)
      }
    }
  }

  async function handleDecodeRawIQ() {
    if (window.go?.main?.App) {
      try {
        decodingWav = true
        wavStatusMsg = i18n.t('gallery.decodingRawIQ')
        const msg = await window.go.main.App.SelectAndDecodeRawIQ()
        wavStatusMsg = msg
        setTimeout(() => {
          decodingWav = false
          wavStatusMsg = ''
        }, 4000)
      } catch (err) {
        console.error("Failed to decode Raw IQ file:", err)
        wavStatusMsg = "Error: " + (err.message || err)
        setTimeout(() => {
          decodingWav = false
          wavStatusMsg = ''
        }, 4000)
      }
    }
  }

  function getSignalStatus(rec) {
    if (!rec) {
      return {
        level: 'unknown',
        label: i18n.t('gallery.signalUnknown'),
        badgeClass: 'bg-slate-800/80 text-slate-400 border-slate-700/60',
        bars: 0,
        barBg: 'bg-slate-600',
        rssiText: 'N/A',
        detailText: 'N/A'
      }
    }

    const history = rec.signal_history || []
    if (history.length > 0) {
      // 1. Median SNR
      const snrs = history.map(p => p.snr).sort((a, b) => a - b)
      const mid = Math.floor(snrs.length / 2)
      const medianSNR = snrs.length % 2 !== 0 ? snrs[mid] : (snrs[mid - 1] + snrs[mid]) / 2

      // 2. Good duration (SNR >= 10dB)
      const goodCount = history.filter(p => p.snr >= 10.0).length
      const goodRatio = (goodCount / history.length) * 100

      // 3. Top 10% SNR
      const snrsDesc = [...snrs].reverse()
      const topCount = Math.max(1, Math.ceil(snrsDesc.length * 0.1))
      const top10SNR = snrsDesc.slice(0, topCount).reduce((a, b) => a + b, 0) / topCount

      const goodPercent = `${goodRatio.toFixed(0)}%`
      const detailText = `SNR +${medianSNR.toFixed(1)}dB (${i18n.t('gallery.goodDuration').split(' ')[0]} ${goodPercent})`

      // 良好 (Excellent): 良好受信時間が40%以上 かつ 中央値SNR 8dB以上
      if (goodRatio >= 40 && medianSNR >= 8.0) {
        return {
          level: 'excellent',
          label: i18n.t('gallery.signalExcellent'),
          badgeClass: 'bg-emerald-500/15 text-emerald-300 border-emerald-500/30',
          bars: 4,
          barBg: 'bg-emerald-400',
          rssiText: detailText,
          detailText
        }
      // 普通 (Good): 良好受信時間が20%以上
      } else if (goodRatio >= 20) {
        return {
          level: 'good',
          label: i18n.t('gallery.signalGood'),
          badgeClass: 'bg-cyan-500/15 text-cyan-300 border-cyan-500/30',
          bars: 3,
          barBg: 'bg-cyan-400',
          rssiText: detailText,
          detailText
        }
      // 可 (Fair): 良好受信時間が10%以上 (1分以上の有効受信があり一部画像が見える)
      } else if (goodRatio >= 10) {
        return {
          level: 'fair',
          label: i18n.t('gallery.signalFair'),
          badgeClass: 'bg-amber-500/15 text-amber-300 border-amber-500/30',
          bars: 2,
          barBg: 'bg-amber-400',
          rssiText: detailText,
          detailText
        }
      // 弱 (Weak): 良好受信時間が10%未満 (ほぼ全編砂嵐・復調不可)
      } else {
        return {
          level: 'weak',
          label: i18n.t('gallery.signalWeak'),
          badgeClass: 'bg-rose-500/15 text-rose-300 border-rose-500/30',
          bars: 1,
          barBg: 'bg-rose-400',
          rssiText: detailText,
          detailText
        }
      }
    }

    // Fallback: when signal_history is not available, use RSSI
    if (rec.avg_rssi === undefined || rec.avg_rssi === 0 || rec.avg_rssi === -100) {
      return {
        level: 'unknown',
        label: i18n.t('gallery.signalUnknown'),
        badgeClass: 'bg-slate-800/80 text-slate-400 border-slate-700/60',
        bars: 0,
        barBg: 'bg-slate-600',
        rssiText: 'N/A',
        detailText: 'N/A'
      }
    }

    const rssi = rec.avg_rssi
    const peak = rec.peak_rssi
    const rssiText = peak ? `${rssi.toFixed(1)} dBFS (Peak ${peak.toFixed(1)})` : `${rssi.toFixed(1)} dBFS`

    if (rssi >= -20 && peak >= -12) {
      return {
        level: 'excellent',
        label: i18n.t('gallery.signalExcellent'),
        badgeClass: 'bg-emerald-500/15 text-emerald-300 border-emerald-500/30',
        bars: 4,
        barBg: 'bg-emerald-400',
        rssiText,
        detailText: rssiText
      }
    } else if (rssi >= -28) {
      return {
        level: 'good',
        label: i18n.t('gallery.signalGood'),
        badgeClass: 'bg-cyan-500/15 text-cyan-300 border-cyan-500/30',
        bars: 3,
        barBg: 'bg-cyan-400',
        rssiText,
        detailText: rssiText
      }
    } else if (rssi >= -36) {
      return {
        level: 'fair',
        label: i18n.t('gallery.signalFair'),
        badgeClass: 'bg-amber-500/15 text-amber-300 border-amber-500/30',
        bars: 2,
        barBg: 'bg-amber-400',
        rssiText,
        detailText: rssiText
      }
    } else {
      return {
        level: 'weak',
        label: i18n.t('gallery.signalWeak'),
        badgeClass: 'bg-rose-500/15 text-rose-300 border-rose-500/30',
        bars: 1,
        barBg: 'bg-rose-400',
        rssiText,
        detailText: rssiText
      }
    }
  }

  function formatGain(rec) {
    if (!rec) return 'N/A'
    if (rec.gain_auto) {
      return i18n.t('gallery.gainAuto')
    }
    if (rec.gain !== undefined && rec.gain !== null && rec.gain > 0) {
      return `${(rec.gain / 10.0).toFixed(1)} dB`
    }
    return 'N/A'
  }

  let selectedSignal = $derived(getSignalStatus(selectedRecord))

  async function loadRecords() {
    if (window.go?.main?.App) {
      try {
        const fetched = await window.go.main.App.GetPassRecords()
        records = (fetched && Array.isArray(fetched)) ? [...fetched] : []
        if (records.length > 0) {
          if (!selectedRecord || !records.find(r => r.id === selectedRecord.id)) {
            selectedRecord = records[0]
          }
          if (selectedRecordIds.length === 0) {
            const withData = records.filter(r => r.signal_history && r.signal_history.length > 0)
            selectedRecordIds = withData.slice(0, Math.min(3, withData.length)).map(r => r.id)
          } else {
            selectedRecordIds = selectedRecordIds.filter(id => records.some(r => r.id === id))
          }
        } else {
          selectedRecord = null
          selectedRecordIds = []
        }
      } catch (err) {
        console.error("Failed to load pass records:", err)
        records = []
      }
    }
  }

  async function updateImageSrc() {
    if (!selectedRecord) {
      currentImageBase64 = ''
      return
    }
    const path = activeChannel === 'full' 
      ? selectedRecord.image_path 
      : (activeChannel === 'chanA' ? selectedRecord.channel_a : selectedRecord.channel_b)

    if (window.go?.main?.App && path) {
      try {
        currentImageBase64 = await window.go.main.App.GetImageBase64(path)
      } catch (err) {
        console.error("Failed to load image base64:", err)
        currentImageBase64 = ''
      }
    } else {
      currentImageBase64 = ''
    }
  }

  $effect(() => {
    selectedRecord
    activeChannel
    updateImageSrc()
  })

  function promptDelete(rec, e) {
    if (e) {
      e.stopPropagation()
      e.preventDefault()
    }
    recordToDelete = rec
  }

  async function confirmDelete() {
    if (!recordToDelete) return
    const recId = recordToDelete.id
    recordToDelete = null

    // Optimistically update frontend state immediately for responsive UI
    records = records.filter(r => r.id !== recId)
    if (selectedRecord?.id === recId) {
      selectedRecord = records.length > 0 ? records[0] : null
    }

    if (window.go?.main?.App) {
      try {
        await window.go.main.App.DeletePassRecord(recId)
        await loadRecords()
      } catch (err) {
        console.error("Failed to delete record:", err)
      }
    }
  }

  function cancelDelete() {
    recordToDelete = null
  }

  onMount(() => {
    loadRecords()
    if (window.runtime?.EventsOn) {
      window.runtime.EventsOn('decode_complete', () => {
        loadRecords()
      })
      window.runtime.EventsOn('ai_complete', () => {
        loadRecords()
      })
    }
  })
</script>

<div class="p-6 space-y-6 overflow-y-auto max-h-[calc(100vh-65px)]">
  <div class="flex items-center justify-between flex-wrap gap-4">
    <div>
      <h2 class="text-xl font-bold text-slate-100 flex items-center gap-2">
        <Image class="w-5 h-5 text-cyan-400" />
        {i18n.t('gallery.title')}
      </h2>
      <p class="text-xs text-slate-400 mt-1">{i18n.t('gallery.subtitle')}</p>
    </div>

    <div class="flex items-center gap-3">
      {#if wavStatusMsg}
        <span class="text-xs text-cyan-400 font-medium animate-pulse">{wavStatusMsg}</span>
      {/if}
      <button
        type="button"
        onclick={handleOpenSpectrumFile}
        class="px-3.5 py-2 bg-slate-900 hover:bg-slate-800 text-slate-200 border border-slate-700 hover:border-cyan-500/50 rounded-xl text-xs font-semibold flex items-center gap-2 transition-all shadow-md active:scale-95 cursor-pointer"
        title={i18n.t('gallery.openSpectrumFile')}
      >
        <Activity class="w-4 h-4 text-cyan-400" />
        {i18n.t('gallery.analyzeSpectrum')}
      </button>
      <button
        type="button"
        onclick={handleOpenAndPlayWAV}
        class="px-3.5 py-2 bg-slate-900 hover:bg-slate-800 text-slate-200 border border-slate-700 hover:border-cyan-500/50 rounded-xl text-xs font-semibold flex items-center gap-2 transition-all shadow-md active:scale-95 cursor-pointer"
        title={i18n.t('gallery.openWavFile')}
      >
        <Music class="w-4 h-4 text-cyan-400" />
        {i18n.t('gallery.playWavFile')}
      </button>
      <button
        type="button"
        onclick={handleDecodeWAV}
        disabled={decodingWav}
        class="px-3.5 py-2 bg-cyan-950/60 hover:bg-cyan-900/80 text-cyan-300 border border-cyan-500/40 rounded-xl text-xs font-semibold flex items-center gap-2 transition-all shadow-md active:scale-95 disabled:opacity-50 cursor-pointer"
      >
        <Upload class="w-4 h-4 text-cyan-400" />
        {i18n.t('gallery.testDecodeWav')}
      </button>
      <button
        type="button"
        onclick={handleDecodeRawIQ}
        disabled={decodingWav}
        class="px-3.5 py-2 bg-indigo-950/60 hover:bg-indigo-900/80 text-indigo-300 border border-indigo-500/40 rounded-xl text-xs font-semibold flex items-center gap-2 transition-all shadow-md active:scale-95 disabled:opacity-50 cursor-pointer"
      >
        <Radio class="w-4 h-4 text-indigo-400" />
        {i18n.t('gallery.testDecodeRawIQ')}
      </button>
    </div>
  </div>

  {#if records.length === 0}
    <div class="bg-slate-900/90 border border-slate-800 rounded-2xl p-12 text-center space-y-4">
      <Image class="w-12 h-12 text-slate-600 mx-auto" />
      <h3 class="text-base font-semibold text-slate-300">{i18n.t('gallery.noImages')}</h3>
      <p class="text-xs text-slate-500 max-w-md mx-auto">
        {i18n.t('gallery.empty')}
      </p>
      <div class="pt-2 flex items-center justify-center gap-3">
        <button
          type="button"
          onclick={handleOpenAndPlayWAV}
          class="px-4 py-2.5 bg-slate-800 hover:bg-slate-700 text-slate-200 border border-slate-700 rounded-xl text-xs font-bold inline-flex items-center gap-2 transition-all shadow-lg active:scale-95 cursor-pointer"
        >
          <Music class="w-4 h-4 text-cyan-400" />
          {i18n.t('gallery.playWavFile')}
        </button>
        <button
          type="button"
          onclick={handleDecodeWAV}
          disabled={decodingWav}
          class="px-4 py-2.5 bg-cyan-600 hover:bg-cyan-500 text-white rounded-xl text-xs font-bold inline-flex items-center gap-2 transition-all shadow-lg active:scale-95 disabled:opacity-50 cursor-pointer"
        >
          <Upload class="w-4 h-4" />
          {i18n.t('gallery.testDecodeWav')}
        </button>
      </div>
    </div>
  {:else}
    <div class="grid grid-cols-1 lg:grid-cols-3 gap-6">
      <!-- Decoded Passes List Column -->
      <div class="bg-slate-900/90 border border-slate-800 rounded-2xl p-4 shadow-lg space-y-3 flex flex-col max-h-[calc(100vh-160px)]">
        <div class="flex items-center justify-between gap-2 pb-1 border-b border-slate-800/80">
          <h3 class="text-xs font-bold text-slate-400 uppercase tracking-wider">
            {i18n.t('gallery.decodedPasses')} <span class="font-mono text-slate-300">({records.length})</span>
          </h3>

          <button
            type="button"
            onclick={() => {
              detailViewTab = 'compare'
              if (selectedRecordIds.length === 0 && selectedRecord) {
                selectedRecordIds = [selectedRecord.id]
              }
            }}
            class="px-2.5 py-1 bg-cyan-950/60 hover:bg-cyan-900/80 text-cyan-300 border border-cyan-500/40 rounded-lg text-[11px] font-semibold flex items-center gap-1.5 transition-all shadow-sm active:scale-95 cursor-pointer"
            title={i18n.t('gallery.compareSignals')}
          >
            <Layers class="w-3.5 h-3.5 text-cyan-400" />
            <span>{i18n.t('gallery.multiCompareTab')}</span>
            {#if selectedRecordIds.length > 0}
              <span class="px-1 py-0.2 text-[10px] font-mono rounded bg-cyan-500/20 text-cyan-200 border border-cyan-500/30">
                {selectedRecordIds.length}
              </span>
            {/if}
          </button>
        </div>

        <!-- Quick Selection Presets Bar -->
        <div class="flex flex-wrap items-center gap-1.5 py-1 text-[11px] bg-slate-950/60 p-2 rounded-xl border border-slate-800/70">
          <span class="text-[10px] text-slate-400 font-semibold flex items-center gap-1 mr-0.5">
            <Filter class="w-3 h-3 text-cyan-400" />
          </span>
          <button
            type="button"
            onclick={selectRecent5}
            class="px-2 py-0.5 bg-slate-900 hover:bg-slate-800 text-slate-300 hover:text-cyan-300 border border-slate-700/80 rounded-md text-[10px] font-medium transition-colors cursor-pointer"
          >
            {i18n.t('gallery.presetRecent5')}
          </button>
          <button
            type="button"
            onclick={selectHighElevation}
            class="px-2 py-0.5 bg-slate-900 hover:bg-slate-800 text-slate-300 hover:text-cyan-300 border border-slate-700/80 rounded-md text-[10px] font-medium transition-colors cursor-pointer"
          >
            {i18n.t('gallery.presetHighElev')}
          </button>
          {#each uniqueSatsWithData as sat}
            <button
              type="button"
              onclick={() => selectBySat(sat)}
              class="px-2 py-0.5 bg-slate-900 hover:bg-slate-800 text-slate-300 hover:text-cyan-300 border border-slate-700/80 rounded-md text-[10px] font-medium transition-colors cursor-pointer"
            >
              {sat}
            </button>
          {/each}
          <div class="ml-auto flex items-center gap-1">
            <button
              type="button"
              onclick={selectAllWithData}
              class="px-1.5 py-0.5 text-slate-400 hover:text-slate-200 text-[10px] transition-colors cursor-pointer"
            >
              {i18n.t('gallery.selectAll')}
            </button>
            <span class="text-slate-600">|</span>
            <button
              type="button"
              onclick={clearSelection}
              class="px-1.5 py-0.5 text-slate-400 hover:text-rose-400 text-[10px] transition-colors cursor-pointer"
            >
              {i18n.t('gallery.clearSelection')}
            </button>
          </div>
        </div>

        <!-- Pass Items List -->
        <div class="space-y-2 flex-1 overflow-y-auto pr-1">
          {#each records as rec}
            <div
              role="button"
              tabindex="0"
              onclick={() => selectedRecord = rec}
              onkeydown={(e) => e.key === 'Enter' && (selectedRecord = rec)}
              class="w-full p-2.5 rounded-xl border text-left transition-all flex items-center justify-between gap-2.5 group cursor-pointer {selectedRecord?.id === rec.id ? 'bg-cyan-950/40 border-cyan-500/50 text-slate-100 ring-1 ring-cyan-500/30' : 'bg-slate-950/60 border-slate-800 text-slate-400 hover:border-slate-700'}"
            >
              <div class="flex items-center gap-2.5 min-w-0">
                <!-- Checkbox for multi-pass selection -->
                <input
                  type="checkbox"
                  checked={selectedRecordIds.includes(rec.id)}
                  onclick={(e) => toggleRecordSelection(rec.id, e)}
                  class="w-3.5 h-3.5 rounded border-slate-700 bg-slate-900 text-cyan-500 focus:ring-cyan-500/30 focus:ring-offset-slate-950 cursor-pointer shrink-0"
                  title="Select for multi-pass comparison"
                />

                <div class="min-w-0">
                  <span class="text-xs font-bold text-slate-200 block truncate">{rec.sat_name}</span>
                  <div class="flex items-center gap-1.5 text-[10px] text-slate-400 truncate">
                    <span class="text-cyan-400 font-mono">{(rec.frequency / 1e6).toFixed(4)} MHz</span>
                    <span>•</span>
                    <span>{rec.max_elevation}°{rec.max_elevation_az ? ` (${rec.max_elevation_az}°)` : ''}</span>
                    {#if rec.gain_auto || (rec.gain && rec.gain > 0)}
                      <span>•</span>
                      <span class="text-amber-300/90 font-mono font-medium">{formatGain(rec)}</span>
                    {/if}
                  </div>
                  <span class="text-[10px] text-slate-500 block mt-0.5 font-mono">{new Date(rec.timestamp).toLocaleString()}</span>
                </div>
              </div>

              <div class="flex items-center gap-1.5 shrink-0">
                {#if rec.audio_path}
                  <span class="p-1 bg-cyan-500/10 text-cyan-400 rounded-md border border-cyan-500/20" title="WAV Audio Available">
                    <Music class="w-3 h-3" />
                  </span>
                {/if}

                {#if (rec.signal_history && rec.signal_history.length > 0) || (rec.avg_rssi !== undefined && rec.avg_rssi !== 0 && rec.avg_rssi !== -100)}
                  {@const sig = getSignalStatus(rec)}
                  <span class="px-1.5 py-1 rounded-lg border text-[10px] font-medium flex items-center gap-1 {sig.badgeClass}" title={`${i18n.t('gallery.signalLabel')} ${sig.label} - ${sig.detailText}`}>
                    <div class="inline-flex items-end gap-0.5 h-2.5">
                      <span class="w-0.5 rounded-xs h-[30%] {sig.bars >= 1 ? sig.barBg : 'bg-slate-700/60'}"></span>
                      <span class="w-0.5 rounded-xs h-[50%] {sig.bars >= 2 ? sig.barBg : 'bg-slate-700/60'}"></span>
                      <span class="w-0.5 rounded-xs h-[75%] {sig.bars >= 3 ? sig.barBg : 'bg-slate-700/60'}"></span>
                      <span class="w-0.5 rounded-xs h-[100%] {sig.bars >= 4 ? sig.barBg : 'bg-slate-700/60'}"></span>
                    </div>
                  </span>
                {/if}

                {#if rec.ai_analysis}
                  <span class="p-1.5 bg-emerald-500/20 text-emerald-400 rounded-lg border border-emerald-500/30" title={i18n.t('gallery.aiReady')}>
                    <Sparkles class="w-3.5 h-3.5" />
                  </span>
                {/if}

                <button
                  type="button"
                  onclick={(e) => promptDelete(rec, e)}
                  class="p-1.5 text-slate-500 hover:text-red-400 hover:bg-red-500/10 rounded-lg border border-transparent hover:border-red-500/20 transition-all cursor-pointer"
                  title={i18n.t('gallery.delete')}
                >
                  <Trash2 class="w-3.5 h-3.5" />
                </button>
              </div>
            </div>
          {/each}
        </div>
      </div>

      <!-- Main Detail / Comparison Panel -->
      {#if selectedRecord}
        <div class="lg:col-span-2 bg-slate-900/90 border border-slate-800 rounded-2xl p-5 shadow-lg space-y-4">
          <div class="flex flex-wrap items-center justify-between gap-4 border-b border-slate-800 pb-3">
            <div>
              <h3 class="text-lg font-bold text-slate-100 flex items-center gap-2 whitespace-nowrap">
                {selectedRecord.sat_name}
                <span class="text-xs font-mono text-cyan-400 font-normal">({(selectedRecord.frequency / 1e6).toFixed(4)} MHz)</span>
              </h3>
              <div class="flex flex-wrap items-center gap-2 text-xs text-slate-400 mt-1">
                <span>{new Date(selectedRecord.timestamp).toLocaleString()}</span>
                <span>•</span>
                <span>{i18n.t('gallery.maxElevLabel')} <b class="text-slate-200">{selectedRecord.max_elevation}°{selectedRecord.max_elevation_az ? ` (${selectedRecord.max_elevation_az}°)` : ''}</b></span>
                {#if selectedRecord.gain_auto || (selectedRecord.gain && selectedRecord.gain > 0)}
                  <span>•</span>
                  <span>{i18n.t('gallery.gainLabel')} <b class="text-amber-300 font-mono">{formatGain(selectedRecord)}</b></span>
                {/if}
                <span>•</span>
                <span class="inline-flex items-center gap-1.5 px-2.5 py-0.5 rounded-lg border text-[11px] font-medium transition-all {selectedSignal.badgeClass}">
                  <div class="inline-flex items-end gap-0.5 h-3">
                    <span class="w-1 rounded-xs h-[30%] {selectedSignal.bars >= 1 ? selectedSignal.barBg : 'bg-slate-700/60'}"></span>
                    <span class="w-1 rounded-xs h-[50%] {selectedSignal.bars >= 2 ? selectedSignal.barBg : 'bg-slate-700/60'}"></span>
                    <span class="w-1 rounded-xs h-[75%] {selectedSignal.bars >= 3 ? selectedSignal.barBg : 'bg-slate-700/60'}"></span>
                    <span class="w-1 rounded-xs h-[100%] {selectedSignal.bars >= 4 ? selectedSignal.barBg : 'bg-slate-700/60'}"></span>
                  </div>
                  <span>{i18n.t('gallery.signalLabel')} <b>{selectedSignal.label}</b></span>
                  {#if selectedSignal.level !== 'unknown'}
                    <span class="font-mono opacity-80 text-[10px]">({selectedSignal.detailText})</span>
                  {/if}
                </span>
              </div>
            </div>

            <div class="flex flex-wrap items-center gap-2.5">
              <!-- Detail Tabs -->
              <div class="flex items-center gap-1 bg-slate-950 p-1 rounded-xl border border-slate-800 text-xs">
                <button
                  onclick={() => detailViewTab = 'image'}
                  class="px-2.5 py-1 rounded-lg font-medium transition-colors flex items-center gap-1.5 {detailViewTab === 'image' ? 'bg-cyan-600 text-white' : 'text-slate-400 hover:text-slate-200'}"
                >
                  <Image class="w-3.5 h-3.5" />
                  {i18n.t('gallery.imageTab')}
                </button>
                <button
                  onclick={() => detailViewTab = 'signal'}
                  class="px-2.5 py-1 rounded-lg font-medium transition-colors flex items-center gap-1.5 {detailViewTab === 'signal' ? 'bg-indigo-600 text-white' : 'text-slate-400 hover:text-slate-200'}"
                >
                  <Activity class="w-3.5 h-3.5" />
                  {i18n.t('gallery.signalHistoryTab')}
                </button>
                <button
                  onclick={() => detailViewTab = 'compare'}
                  class="px-2.5 py-1 rounded-lg font-medium transition-colors flex items-center gap-1.5 {detailViewTab === 'compare' ? 'bg-emerald-600 text-white' : 'text-slate-400 hover:text-slate-200'}"
                >
                  <Layers class="w-3.5 h-3.5" />
                  {i18n.t('gallery.multiCompareTab')}
                  {#if selectedRecordIds.length > 0}
                    <span class="ml-0.5 px-1 py-0.2 rounded text-[10px] font-mono {detailViewTab === 'compare' ? 'bg-emerald-800 text-white' : 'bg-slate-800 text-cyan-300'}">
                      {selectedRecordIds.length}
                    </span>
                  {/if}
                </button>
                <button
                  onclick={() => detailViewTab = 'audio'}
                  class="px-2.5 py-1 rounded-lg font-medium transition-colors flex items-center gap-1.5 {detailViewTab === 'audio' ? 'bg-amber-600 text-white' : 'text-slate-400 hover:text-slate-200'}"
                >
                  <Music class="w-3.5 h-3.5" />
                  {i18n.t('gallery.audioTab')}
                </button>
                <button
                  onclick={() => detailViewTab = 'spectrum'}
                  class="px-2.5 py-1 rounded-lg font-medium transition-colors flex items-center gap-1.5 {detailViewTab === 'spectrum' ? 'bg-indigo-600 text-white' : 'text-slate-400 hover:text-slate-200'}"
                >
                  <Activity class="w-3.5 h-3.5" />
                  {i18n.t('gallery.spectrumTab')}
                </button>
              </div>

              <!-- Image Options (Channel & Aspect Ratio Selects) -->
              {#if detailViewTab === 'image'}
                <div class="flex items-center gap-2">
                  <select
                    bind:value={activeChannel}
                    class="bg-slate-950 text-slate-200 border border-slate-700/80 rounded-xl px-2.5 py-1.5 text-xs font-medium focus:outline-none focus:border-cyan-500 cursor-pointer transition-colors shadow-sm"
                  >
                    <option value="full">{i18n.t('gallery.viewFull')}</option>
                    <option value="chanA">{i18n.t('gallery.viewChanA')}</option>
                    <option value="chanB">{i18n.t('gallery.viewChanB')}</option>
                  </select>

                  <select
                    bind:value={viewMode}
                    class="bg-slate-950 text-slate-200 border border-slate-700/80 rounded-xl px-2.5 py-1.5 text-xs font-medium focus:outline-none focus:border-cyan-500 cursor-pointer transition-colors shadow-sm"
                  >
                    <option value="stretch">{i18n.t('gallery.stretchY')}</option>
                    <option value="fit">{i18n.t('gallery.fitAspect')}</option>
                  </select>
                </div>
              {/if}

              <button
                type="button"
                onclick={(e) => promptDelete(selectedRecord, e)}
                class="px-2.5 py-1.5 bg-red-500/10 hover:bg-red-500/20 text-red-400 rounded-xl border border-red-500/30 text-xs font-medium flex items-center gap-1.5 transition-all cursor-pointer"
                title={i18n.t('gallery.delete')}
              >
                <Trash2 class="w-3.5 h-3.5" />
                <span>{i18n.t('gallery.delete')}</span>
              </button>
            </div>
          </div>

          {#if detailViewTab === 'image'}
            <div class="relative bg-slate-950 rounded-xl overflow-hidden border border-slate-800 min-h-[360px] flex items-center justify-center p-4">
              {#if currentImageBase64}
                <button
                  type="button"
                  onclick={() => showZoomModal = true}
                  class="w-full flex items-center justify-center group focus:outline-none cursor-zoom-in"
                  title={i18n.t('gallery.clickToZoom')}
                >
                  <img
                    src={currentImageBase64}
                    alt="APT Satellite Capture"
                    style="image-rendering: pixelated;"
                    class="rounded-lg shadow-xl transition-all {viewMode === 'stretch' ? 'w-full h-[360px] object-fill' : 'max-h-[460px] object-contain'}"
                  />
                </button>
              {:else}
                <div class="text-center py-12 px-6 space-y-3">
                  <AlertTriangle class="w-10 h-10 text-amber-500/80 mx-auto" />
                  <p class="text-xs text-slate-300 font-medium max-w-sm">
                    {i18n.t('gallery.imageNotFound')}
                  </p>
                </div>
              {/if}
            </div>
          {:else if detailViewTab === 'signal'}
            <div class="space-y-3">
              <div class="flex items-center justify-between px-1">
                <span class="text-xs text-slate-400 flex items-center gap-1.5">
                  <Activity class="w-4 h-4 text-emerald-400" />
                  {i18n.t('gallery.signalChartTitle')}
                </span>
                <button
                  type="button"
                  onclick={() => {
                    if (!selectedRecordIds.includes(selectedRecord.id)) {
                      selectedRecordIds = [...selectedRecordIds, selectedRecord.id]
                    }
                    detailViewTab = 'compare'
                  }}
                  class="text-xs text-cyan-400 hover:text-cyan-300 flex items-center gap-1 font-medium hover:underline cursor-pointer"
                >
                  <Layers class="w-3.5 h-3.5" />
                  {i18n.t('gallery.multiCompareTab')}
                </button>
              </div>
              <div class="bg-slate-950/60 rounded-xl border border-slate-800 p-4">
                <SignalChart signalHistory={selectedRecord.signal_history || []} />
              </div>
            </div>
          {:else if detailViewTab === 'compare'}
            <div class="bg-slate-950/60 rounded-xl border border-slate-800 p-4">
              <MultiSignalChart
                selectedRecords={selectedRecordsList.length > 0 ? selectedRecordsList : (selectedRecord ? [selectedRecord] : [])}
                onDeselect={handleDeselectRecord}
              />
            </div>
          {:else if detailViewTab === 'audio'}
            <div class="space-y-4">
              <AudioPlayer
                audioPath={selectedRecord.audio_path}
                title={`${selectedRecord.sat_name} APT Recorded Audio`}
              />

              <div class="bg-slate-950/60 rounded-xl border border-slate-800 p-4 space-y-2 text-xs text-slate-400">
                <h5 class="font-bold text-slate-300 flex items-center gap-2">
                  <Activity class="w-4 h-4 text-cyan-400" />
                  APT Audio Stream Information
                </h5>
                <div class="grid grid-cols-2 sm:grid-cols-5 gap-3 pt-2 font-mono text-[11px]">
                  <div class="bg-slate-900/80 p-2.5 rounded-lg border border-slate-800">
                    <span class="text-slate-500 block text-[10px]">Sampling Rate</span>
                    <span class="text-slate-200 font-bold">20,800 Hz (Mono)</span>
                  </div>
                  <div class="bg-slate-900/80 p-2.5 rounded-lg border border-slate-800">
                    <span class="text-slate-500 block text-[10px]">Subcarrier</span>
                    <span class="text-slate-200 font-bold">2,400 Hz AM</span>
                  </div>
                  <div class="bg-slate-900/80 p-2.5 rounded-lg border border-slate-800">
                    <span class="text-slate-500 block text-[10px]">Tuner Gain</span>
                    <span class="text-amber-300 font-bold">{formatGain(selectedRecord)}</span>
                  </div>
                  <div class="bg-slate-900/80 p-2.5 rounded-lg border border-slate-800">
                    <span class="text-slate-500 block text-[10px]">Max Elevation</span>
                    <span class="text-slate-200 font-bold">{selectedRecord.max_elevation}°</span>
                  </div>
                  <div class="bg-slate-900/80 p-2.5 rounded-lg border border-slate-800">
                    <span class="text-slate-500 block text-[10px]">Average RSSI</span>
                    <span class="text-slate-200 font-bold">{selectedRecord.avg_rssi ? selectedRecord.avg_rssi.toFixed(1) : 'N/A'} dBFS</span>
                  </div>
                </div>
              </div>
            </div>
          {:else if detailViewTab === 'spectrum'}
            <div class="space-y-4">
              <SpectrumViewer
                record={selectedRecord}
                title={`${selectedRecord.sat_name} FFT Spectrum`}
              />
            </div>
          {/if}

          <div class="flex items-center justify-between pt-2">
            <span class="text-xs text-slate-500 font-mono">{i18n.t('gallery.imageRes')}</span>
            <button
              onclick={() => onRunAI(selectedRecord)}
              class="px-5 py-2.5 bg-gradient-to-r from-cyan-600 to-indigo-600 hover:from-cyan-500 hover:to-indigo-500 text-white rounded-xl text-xs font-bold shadow-lg shadow-cyan-600/30 flex items-center gap-2 transition-all"
            >
              <Bot class="w-4 h-4 text-cyan-300" />
              {i18n.t('gallery.runAI')}
            </button>
          </div>
        </div>
      {/if}
    </div>
  {/if}
</div>

<!-- Image Zoom Modal -->
{#if showZoomModal && currentImageBase64}
  <div class="fixed inset-0 z-50 bg-slate-950/90 backdrop-blur-md flex flex-col items-center justify-center p-6">
    <div class="absolute top-4 right-4 flex items-center gap-3">
      <button
        onclick={() => showZoomModal = false}
        class="p-2 bg-slate-800 hover:bg-slate-700 text-slate-200 rounded-xl transition-all"
      >
        <X class="w-5 h-5" />
      </button>
    </div>
    <div class="max-w-5xl w-full max-h-[85vh] overflow-auto p-2 bg-slate-900 border border-slate-800 rounded-2xl flex items-center justify-center">
      <img
        src={currentImageBase64}
        alt="APT Satellite Capture Full Zoom"
        style="image-rendering: pixelated;"
        class="w-full object-contain rounded-xl"
      />
    </div>
  </div>
{/if}

<!-- Custom Delete Confirmation Modal -->
{#if recordToDelete}
  <div class="fixed inset-0 z-50 bg-slate-950/80 backdrop-blur-sm flex items-center justify-center p-4">
    <div class="bg-slate-900 border border-slate-800 rounded-2xl p-6 max-w-md w-full shadow-2xl space-y-4">
      <div class="flex items-center justify-between">
        <div class="flex items-center gap-3 text-red-400">
          <div class="p-2.5 bg-red-500/10 border border-red-500/20 rounded-xl">
            <Trash2 class="w-5 h-5 text-red-400" />
          </div>
          <div>
            <h3 class="text-sm font-bold text-slate-100">{i18n.t('gallery.delete')}</h3>
            <p class="text-xs text-slate-400 font-mono mt-0.5">{recordToDelete.sat_name} ({new Date(recordToDelete.timestamp).toLocaleString()})</p>
          </div>
        </div>
        <button
          onclick={cancelDelete}
          class="p-1 text-slate-400 hover:text-slate-200 rounded-lg hover:bg-slate-800 transition-all"
        >
          <X class="w-4 h-4" />
        </button>
      </div>

      <p class="text-xs text-slate-300 leading-relaxed">
        {i18n.t('gallery.deleteConfirm')}
      </p>

      <div class="flex items-center justify-end gap-2 pt-2">
        <button
          type="button"
          onclick={cancelDelete}
          class="px-4 py-2 bg-slate-800 hover:bg-slate-700 text-slate-300 rounded-xl text-xs font-semibold transition-all"
        >
          {i18n.t('gallery.cancel')}
        </button>
        <button
          type="button"
          onclick={confirmDelete}
          class="px-4 py-2 bg-red-600 hover:bg-red-500 text-white rounded-xl text-xs font-semibold shadow-lg shadow-red-600/30 transition-all"
        >
          {i18n.t('gallery.delete')}
        </button>
      </div>
    </div>
  </div>
{/if}

<!-- Standalone WAV Audio Playback Modal -->
{#if showStandaloneAudioModal && standaloneWavPath}
  <div class="fixed inset-0 z-50 bg-slate-950/80 backdrop-blur-sm flex items-center justify-center p-4">
    <div class="bg-slate-900 border border-slate-800 rounded-2xl p-6 max-w-lg w-full shadow-2xl space-y-4">
      <div class="flex items-center justify-between">
        <div class="flex items-center gap-3 text-cyan-400">
          <div class="p-2.5 bg-cyan-500/10 border border-cyan-500/20 rounded-xl">
            <Music class="w-5 h-5 text-cyan-400" />
          </div>
          <div>
            <h3 class="text-sm font-bold text-slate-100">{i18n.t('gallery.playWavFile')}</h3>
            <p class="text-xs text-slate-400 font-mono mt-0.5 truncate max-w-xs">{standaloneWavPath.split(/[/\\]/).pop()}</p>
          </div>
        </div>
        <button
          onclick={() => showStandaloneAudioModal = false}
          class="p-1.5 text-slate-400 hover:text-slate-200 rounded-lg hover:bg-slate-800 transition-all"
        >
          <X class="w-4 h-4" />
        </button>
      </div>

      <AudioPlayer
        audioPath={standaloneWavPath}
        title={standaloneWavPath.split(/[/\\]/).pop()}
      />

      <div class="flex items-center justify-between pt-2 text-xs">
        <button
          type="button"
          onclick={handleOpenAndPlayWAV}
          class="text-cyan-400 hover:text-cyan-300 transition-colors flex items-center gap-1.5 cursor-pointer"
        >
          <Upload class="w-3.5 h-3.5" />
          {i18n.t('gallery.openWavFile')}
        </button>

        <button
          type="button"
          onclick={() => showStandaloneAudioModal = false}
          class="px-4 py-2 bg-slate-800 hover:bg-slate-700 text-slate-300 rounded-xl text-xs font-semibold transition-all cursor-pointer"
        >
          {i18n.t('gallery.cancel')}
        </button>
      </div>
    </div>
  </div>
{/if}

<!-- Standalone Spectrum Analysis Modal -->
{#if showStandaloneSpectrumModal && standaloneSpectrumPath}
  <div class="fixed inset-0 z-50 bg-slate-950/85 backdrop-blur-md flex items-center justify-center p-4">
    <div class="bg-slate-900 border border-slate-800 rounded-2xl p-6 max-w-4xl w-full shadow-2xl space-y-4 max-h-[90vh] overflow-y-auto">
      <div class="flex items-center justify-between">
        <div class="flex items-center gap-3 text-cyan-400">
          <div class="p-2.5 bg-cyan-500/10 border border-cyan-500/20 rounded-xl">
            <Activity class="w-5 h-5 text-cyan-400" />
          </div>
          <div>
            <h3 class="text-sm font-bold text-slate-100">{i18n.t('gallery.analyzeSpectrum')}</h3>
            <p class="text-xs text-slate-400 font-mono mt-0.5 truncate max-w-md">{standaloneSpectrumPath}</p>
          </div>
        </div>
        <button
          onclick={() => showStandaloneSpectrumModal = false}
          class="p-1.5 text-slate-400 hover:text-slate-200 rounded-lg hover:bg-slate-800 transition-all cursor-pointer"
        >
          <X class="w-4 h-4" />
        </button>
      </div>

      <SpectrumViewer
        filePath={standaloneSpectrumPath}
        title={standaloneSpectrumPath.split(/[/\\]/).pop()}
      />

      <div class="flex items-center justify-between pt-2 text-xs">
        <button
          type="button"
          onclick={handleOpenSpectrumFile}
          class="text-cyan-400 hover:text-cyan-300 transition-colors flex items-center gap-1.5 cursor-pointer"
        >
          <Upload class="w-3.5 h-3.5" />
          {i18n.t('gallery.openSpectrumFile')}
        </button>

        <button
          type="button"
          onclick={() => showStandaloneSpectrumModal = false}
          class="px-4 py-2 bg-slate-800 hover:bg-slate-700 text-slate-300 rounded-xl text-xs font-semibold transition-all cursor-pointer"
        >
          {i18n.t('gallery.cancel')}
        </button>
      </div>
    </div>
  </div>
{/if}


