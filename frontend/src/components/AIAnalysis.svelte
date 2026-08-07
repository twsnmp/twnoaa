<script>
  import { onMount } from 'svelte'
  import { i18n } from '../i18n.svelte.js'
  import { Bot, Sparkles, FileText, Satellite, AlertTriangle } from '@lucide/svelte'

  let { record = $bindable(null), onSelectRecord = () => {} } = $props()

  let isAnalyzing = $state(false)
  let statusMsg = $state('')
  let records = $state([])

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
    statusMsg = i18n.t('ai.statusRunning')

    try {
      const res = await window.go.main.App.RunAIAnalysis(record.id)
      if (res && res.startsWith("Error:")) {
        statusMsg = i18n.t('ai.statusError') + res
        isAnalyzing = false
      } else {
        statusMsg = res
      }
    } catch (err) {
      statusMsg = i18n.t('ai.statusError') + err
      isAnalyzing = false
    }
  }

  onMount(() => {
    loadRecords()

    if (window.runtime?.EventsOn) {
      window.runtime.EventsOn('ai_complete', (updatedRec) => {
        if (record && updatedRec.id === record.id) {
          record = updatedRec
          isAnalyzing = false
          statusMsg = i18n.t('ai.statusCompleted')
        }
        loadRecords()
      })

      window.runtime.EventsOn('ai_error', (errMsg) => {
        isAnalyzing = false
        statusMsg = i18n.t('ai.statusFailed') + errMsg
      })

      window.runtime.EventsOn('decode_complete', () => {
        loadRecords()
      })
    }
  })
</script>

<div class="p-6 space-y-6 overflow-y-auto max-h-[calc(100vh-65px)]">
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
      <div class="flex flex-wrap items-center justify-between gap-4 bg-slate-950 p-4 rounded-xl border border-slate-800">
        <div class="flex items-center gap-3">
          <div class="p-3 bg-cyan-500/10 text-cyan-400 rounded-xl border border-cyan-500/30">
            <Satellite class="w-6 h-6" />
          </div>
          <div>
            <h3 class="text-base font-bold text-slate-100">{record.sat_name}</h3>
            <p class="text-xs font-mono text-cyan-400">{(record.frequency / 1e6).toFixed(4)} MHz • {new Date(record.timestamp).toLocaleString()}</p>
          </div>
        </div>

        <button
          onclick={triggerAnalysis}
          disabled={isAnalyzing || !hasImage}
          class="px-5 py-2.5 bg-gradient-to-r from-cyan-600 to-indigo-600 hover:from-cyan-500 hover:to-indigo-500 text-white rounded-xl text-xs font-bold shadow-lg shadow-cyan-600/30 flex items-center gap-2 transition-all disabled:opacity-40 disabled:cursor-not-allowed disabled:hover:from-cyan-600 disabled:hover:to-indigo-600"
        >
          <Sparkles class="w-4 h-4 text-amber-300 {isAnalyzing ? 'animate-spin' : ''}" />
          {isAnalyzing ? i18n.t('ai.analyzing') : i18n.t('ai.analyzeNow')}
        </button>
      </div>

      {#if !hasImage}
        <div class="p-3 bg-amber-950/40 border border-amber-800/40 rounded-xl text-xs text-amber-300 font-medium flex items-center gap-2">
          <AlertTriangle class="w-4 h-4 text-amber-400 shrink-0" />
          <span>{i18n.t('ai.noImageAvailable')}</span>
        </div>
      {/if}

      {#if statusMsg}
        <div class="p-3 bg-cyan-950/40 border border-cyan-800/40 rounded-xl text-xs text-cyan-300 font-medium">
          {statusMsg}
        </div>
      {/if}

      <div class="bg-slate-950 rounded-xl border border-slate-800 p-6 space-y-4">
        <h4 class="text-sm font-bold text-slate-300 uppercase tracking-wider flex items-center gap-2 border-b border-slate-800 pb-3">
          <FileText class="w-4 h-4 text-cyan-400" />
          {i18n.t('ai.reportTitle')}
        </h4>

        {#if analysisResult}
          <div class="prose prose-invert max-w-none text-xs leading-relaxed text-slate-300 whitespace-pre-wrap font-sans">
            {analysisResult}
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

