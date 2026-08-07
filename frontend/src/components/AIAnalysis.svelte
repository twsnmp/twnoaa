<script>
  import { onMount } from 'svelte'
  import { i18n } from '../i18n.svelte.js'
  import { Bot, Sparkles, RefreshCw, FileText, Satellite, Cpu, AlertCircle } from '@lucide/svelte'

  let { record = null, onSelectRecord = () => {} } = $props()

  let isAnalyzing = $state(false)
  let statusMsg = $state('')

  let analysisResult = $derived(record?.ai_analysis || '')

  async function triggerAnalysis() {
    if (!record || !window.go?.main?.App) return
    isAnalyzing = true
    statusMsg = i18n.t('ai.statusRunning')

    try {
      const res = await window.go.main.App.RunAIAnalysis(record.id)
      statusMsg = res
    } catch (err) {
      statusMsg = i18n.t('ai.statusError') + err
      isAnalyzing = false
    }
  }

  onMount(() => {
    if (window.runtime?.EventsOn) {
      window.runtime.EventsOn('ai_complete', (updatedRec) => {
        if (record && updatedRec.id === record.id) {
          record = updatedRec
          isAnalyzing = false
          statusMsg = i18n.t('ai.statusCompleted')
        }
      })

      window.runtime.EventsOn('ai_error', (errMsg) => {
        isAnalyzing = false
        statusMsg = i18n.t('ai.statusFailed') + errMsg
      })
    }
  })
</script>

<div class="p-6 space-y-6 overflow-y-auto max-h-[calc(100vh-65px)]">
  <div class="flex items-center justify-between">
    <div>
      <h2 class="text-xl font-bold text-slate-100 flex items-center gap-2">
        <Bot class="w-5 h-5 text-cyan-400" />
        {i18n.t('ai.title')}
      </h2>
      <p class="text-xs text-slate-400 mt-1">{i18n.t('ai.subtitle')}</p>
    </div>
  </div>

  {#if !record}
    <div class="bg-slate-900/90 border border-slate-800 rounded-2xl p-12 text-center space-y-3">
      <Bot class="w-12 h-12 text-slate-600 mx-auto" />
      <h3 class="text-base font-semibold text-slate-300">{i18n.t('ai.noPassSelected')}</h3>
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
          disabled={isAnalyzing}
          class="px-5 py-2.5 bg-gradient-to-r from-cyan-600 to-indigo-600 hover:from-cyan-500 hover:to-indigo-500 text-white rounded-xl text-xs font-bold shadow-lg shadow-cyan-600/30 flex items-center gap-2 transition-all disabled:opacity-50"
        >
          <Sparkles class="w-4 h-4 text-amber-300 {isAnalyzing ? 'animate-spin' : ''}" />
          {isAnalyzing ? i18n.t('ai.analyzing') : i18n.t('ai.analyzeNow')}
        </button>
      </div>

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
