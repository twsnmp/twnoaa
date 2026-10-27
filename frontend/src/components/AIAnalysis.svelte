<script>
  import { onMount } from 'svelte'
  import { i18n } from '../i18n.svelte.js'
  import {
    Bot,
    Sparkles,
    FileText,
    Satellite,
    AlertTriangle,
    Copy,
    Check,
    X,
    Loader2,
    RefreshCw
  } from '@lucide/svelte'
  import AnimatedCatLogo from './common/AnimatedCatLogo.svelte'
  import AIMarkdown from './common/AIMarkdown.svelte'

  let { record = $bindable(null), onSelectRecord = () => {} } = $props()

  let isAnalyzing = $state(false)
  let showThinkingModal = $state(false)
  let statusMsg = $state('')
  let records = $state([])
  let copied = $state(false)

  let analysisResult = $derived(record?.ai_analysis || '')
  let hasImage = $derived(!!record?.image_path)

  async function loadRecords() {
    if (window.go?.main?.App) {
      try {
        const fetched = await window.go.main.App.GetPassRecords()
        records = (fetched && Array.isArray(fetched)) ? [...fetched] : []
        if (records.length > 0) {
          if (!record || !records.find(r => r.id === record.id)) {
            record = records[0]
            onSelectRecord(records[0])
          } else {
            const current = records.find(r => r.id === record.id)
            if (current) record = current
          }
        } else {
          record = null
        }
      } catch (err) {
        console.error("Failed to load pass records for AI Analysis:", err)
        records = []
      }
    }
  }

  function handleSelectRecordChange(e) {
    const selectedId = e.target.value
    const found = records.find(r => r.id === selectedId)
    if (found) {
      record = found
      onSelectRecord(found)
      statusMsg = ''
    }
  }

  async function triggerAnalysis() {
    if (!record || !window.go?.main?.App || !hasImage) return
    isAnalyzing = true
    showThinkingModal = true
    statusMsg = i18n.t('ai.statusRunning')

    try {
      const res = await window.go.main.App.RunAIAnalysis(record.id, i18n.currentLocale)
      if (res && res.startsWith("Error:")) {
        statusMsg = i18n.t('ai.statusError') + res
        isAnalyzing = false
        showThinkingModal = false
      } else {
        statusMsg = res
      }
    } catch (err) {
      statusMsg = i18n.t('ai.statusError') + err
      isAnalyzing = false
      showThinkingModal = false
    }
  }

  async function handleCopyReport() {
    if (!analysisResult) return
    try {
      await navigator.clipboard.writeText(analysisResult)
      copied = true
      setTimeout(() => {
        copied = false
      }, 2000)
    } catch (e) {
      console.error("Failed to copy AI analysis result:", e)
    }
  }

  function handleCloseThinkingModal() {
    showThinkingModal = false
  }

  onMount(() => {
    loadRecords()

    if (window.runtime?.EventsOn) {
      window.runtime.EventsOn('ai_complete', (updatedRec) => {
        if (record && updatedRec.id === record.id) {
          record = updatedRec
          isAnalyzing = false
          showThinkingModal = false
          statusMsg = i18n.t('ai.statusCompleted')
        }
        loadRecords()
      })

      window.runtime.EventsOn('ai_error', (errMsg) => {
        isAnalyzing = false
        showThinkingModal = false
        statusMsg = i18n.t('ai.statusFailed') + errMsg
      })

      window.runtime.EventsOn('decode_complete', () => {
        loadRecords()
      })
    }
  })
</script>

<div class="p-6 space-y-6 overflow-y-auto max-h-[calc(100vh-65px)]">
  <!-- Header -->
  <div class="flex flex-wrap items-center justify-between gap-4">
    <div>
      <h2 class="text-xl font-bold text-slate-100 flex items-center gap-2">
        <Bot class="w-5 h-5 text-cyan-400" />
        {i18n.t('ai.title')}
      </h2>
      <p class="text-xs text-slate-400 mt-1">{i18n.t('ai.subtitle')}</p>
    </div>

    {#if records.length > 0}
      <div class="flex items-center gap-2">
        <label for="record-select" class="text-xs font-medium text-slate-400">{i18n.t('ai.selectPass')}:</label>
        <select
          id="record-select"
          value={record?.id || ''}
          onchange={handleSelectRecordChange}
          class="bg-slate-900 border border-slate-700 text-slate-200 text-xs rounded-xl px-3 py-2 focus:outline-none focus:border-cyan-500 font-mono"
        >
          {#each records as rec}
            <option value={rec.id}>
              {rec.sat_name} - {new Date(rec.timestamp).toLocaleString()} {(rec.frequency / 1e6).toFixed(3)}MHz
            </option>
          {/each}
        </select>
      </div>
    {/if}
  </div>

  {#if !record}
    <div class="bg-slate-900/90 border border-slate-800 rounded-2xl p-12 text-center space-y-3">
      <Bot class="w-12 h-12 text-slate-600 mx-auto" />
      <h3 class="text-base font-semibold text-slate-300">{i18n.t('ai.noPassAvailable')}</h3>
      <p class="text-xs text-slate-500 max-w-md mx-auto">
        {i18n.t('ai.noAnalysis')}
      </p>
    </div>
  {:else}
    <div class="bg-slate-900/90 border border-slate-800 rounded-2xl p-5 shadow-lg space-y-5">
      <!-- Target satellite information card -->
      <div class="flex flex-wrap items-center justify-between gap-4 bg-slate-950 p-4 rounded-xl border border-slate-800">
        <div class="flex items-center gap-3">
          <div class="p-3 bg-cyan-500/10 text-cyan-400 rounded-xl border border-cyan-500/30">
            <Satellite class="w-6 h-6" />
          </div>
          <div>
            <h3 class="text-base font-bold text-slate-100">{record.sat_name}</h3>
            <p class="text-xs font-mono text-cyan-400">
              {(record.frequency / 1e6).toFixed(4)} MHz • {new Date(record.timestamp).toLocaleString()}
            </p>
          </div>
        </div>

        <button
          onclick={triggerAnalysis}
          disabled={isAnalyzing || !hasImage}
          class="px-5 py-2.5 bg-gradient-to-r from-cyan-600 to-indigo-600 hover:from-cyan-500 hover:to-indigo-500 text-white rounded-xl text-xs font-bold shadow-lg shadow-cyan-600/30 flex items-center gap-2 transition-all disabled:opacity-40 disabled:cursor-not-allowed disabled:hover:from-cyan-600 disabled:hover:to-indigo-600 cursor-pointer"
        >
          {#if isAnalyzing}
            <RefreshCw class="w-4 h-4 animate-spin" />
            <span>{i18n.t('ai.analyzing')}</span>
          {:else}
            <Sparkles class="w-4 h-4 text-amber-300" />
            <span>{analysisResult ? i18n.t('ai.analyzeNow') : i18n.t('ai.analyzeNow')}</span>
          {/if}
        </button>
      </div>

      {#if !hasImage}
        <div class="p-3 bg-amber-950/40 border border-amber-800/40 rounded-xl text-xs text-amber-300 font-medium flex items-center gap-2">
          <AlertTriangle class="w-4 h-4 text-amber-400 shrink-0" />
          <span>{i18n.t('ai.noImageAvailable')}</span>
        </div>
      {/if}

      {#if statusMsg}
        <div class="p-3 bg-cyan-950/40 border border-cyan-800/40 rounded-xl text-xs text-cyan-300 font-medium flex items-center justify-between">
          <span class="flex items-center gap-2">
            {#if isAnalyzing}
              <Loader2 class="w-4 h-4 animate-spin text-cyan-400 shrink-0" />
            {/if}
            {statusMsg}
          </span>
          {#if isAnalyzing && !showThinkingModal}
            <button
              type="button"
              onclick={() => (showThinkingModal = true)}
              class="text-xs text-cyan-400 hover:text-cyan-300 underline font-semibold cursor-pointer"
            >
              アニメーションを表示
            </button>
          {/if}
        </div>
      {/if}

      <!-- Report Section -->
      <div class="bg-slate-950 rounded-xl border border-slate-800 p-6 space-y-4">
        <div class="flex items-center justify-between border-b border-slate-800 pb-3">
          <h4 class="text-sm font-bold text-slate-300 uppercase tracking-wider flex items-center gap-2">
            <FileText class="w-4 h-4 text-cyan-400" />
            {i18n.t('ai.reportTitle')}
          </h4>

          {#if analysisResult}
            <button
              type="button"
              onclick={handleCopyReport}
              class="flex items-center gap-1.5 rounded-lg border border-slate-700 bg-slate-900 px-3 py-1.5 text-xs text-slate-300 hover:bg-slate-800 hover:text-white transition-colors cursor-pointer"
            >
              {#if copied}
                <Check class="h-3.5 w-3.5 text-emerald-400" />
                <span class="text-emerald-400 font-semibold">{i18n.t('ai.copied')}</span>
              {:else}
                <Copy class="h-3.5 w-3.5 text-slate-400" />
                <span>{i18n.t('ai.copyReport')}</span>
              {/if}
            </button>
          {/if}
        </div>

        {#if analysisResult}
          <div class="pt-1">
            <AIMarkdown content={analysisResult} />
          </div>
        {:else if isAnalyzing}
          <!-- Embedded mini waiting indicator if user closed modal -->
          <div class="flex flex-col items-center justify-center py-10 space-y-3">
            <AnimatedCatLogo mode="loading" size="md" />
            <p class="text-xs text-cyan-400 animate-pulse font-medium">{i18n.t('ai.statusRunning')}</p>
          </div>
        {:else}
          <div class="py-12 text-center text-slate-500 text-xs">
            {i18n.t('ai.noReport')}
          </div>
        {/if}
      </div>
    </div>
  {/if}
</div>

<!-- AI Thinking Cat Modal -->
{#if showThinkingModal}
  <div
    class="fixed inset-0 z-50 flex items-center justify-center bg-black/75 backdrop-blur-sm p-4 animate-in fade-in duration-200"
    role="dialog"
    aria-modal="true"
  >
    <div
      class="relative flex flex-col items-center w-full max-w-md rounded-2xl border border-cyan-500/40 bg-slate-900/95 p-6 shadow-2xl shadow-cyan-950/60 backdrop-blur-xl transition-all overflow-hidden z-10 text-slate-100"
    >
      <!-- Optional Close Button to let user dismiss the modal while keeping inference running -->
      <button
        type="button"
        onclick={handleCloseThinkingModal}
        class="absolute top-4 right-4 rounded-xl p-1.5 text-slate-400 hover:bg-slate-800 hover:text-slate-200 transition-colors cursor-pointer"
        aria-label={i18n.t('ai.closeModal')}
        title={i18n.t('ai.cancelWait')}
      >
        <X class="h-4 w-4" />
      </button>

      <!-- Animated Cat Logo with Radar, Scan Beam & Pulse -->
      <div class="my-4">
        <AnimatedCatLogo mode="loading" size="lg" />
      </div>

      <!-- Title & Spinner -->
      <div class="text-center mt-2 space-y-2 w-full">
        <h3 class="text-base font-bold text-slate-100 flex items-center justify-center gap-2">
          <Loader2 class="h-4 w-4 animate-spin text-cyan-400" />
          <span>{i18n.t('ai.modalTitle')}</span>
        </h3>

        <p class="text-xs text-slate-400 font-medium leading-relaxed max-w-sm mx-auto">
          {i18n.t('ai.modalDesc')}
        </p>

        <!-- Cyber Loading Bar -->
        <div class="mt-4 w-full bg-slate-800 rounded-full h-1.5 overflow-hidden border border-slate-700/60">
          <div class="h-full bg-gradient-to-r from-cyan-500 via-teal-400 to-indigo-500 rounded-full animate-pulse w-full"></div>
        </div>

        <div class="pt-3">
          <button
            type="button"
            onclick={handleCloseThinkingModal}
            class="text-[11px] text-slate-400 hover:text-slate-200 underline transition-colors cursor-pointer"
          >
            {i18n.t('ai.cancelWait')}
          </button>
        </div>
      </div>
    </div>
  </div>
{/if}
