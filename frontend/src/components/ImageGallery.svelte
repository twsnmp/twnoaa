<script>
  import { onMount } from 'svelte'
  import { i18n } from '../i18n.svelte.js'
  import { Image as ImageIcon, Bot, Calendar, Radio, Compass, Eye, Sparkles } from '@lucide/svelte'

  let { onRunAI = () => {} } = $props()

  let records = $state([])
  let selectedRecord = $state(null)
  let activeChannel = $state('full')

  async function loadRecords() {
    if (window.go?.main?.App) {
      try {
        records = await window.go.main.App.GetPassRecords()
        if (records.length > 0 && !selectedRecord) {
          selectedRecord = records[0]
        }
      } catch (err) {
        console.error("Failed to load pass records:", err)
      }
    }
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
  <div class="flex items-center justify-between">
    <div>
      <h2 class="text-xl font-bold text-slate-100 flex items-center gap-2">
        <ImageIcon class="w-5 h-5 text-cyan-400" />
        {i18n.t('gallery.title')}
      </h2>
      <p class="text-xs text-slate-400 mt-1">Decoded APT Satellite PNG Images & Reception History</p>
    </div>
  </div>

  {#if records.length === 0}
    <div class="bg-slate-900/90 border border-slate-800 rounded-2xl p-12 text-center space-y-3">
      <ImageIcon class="w-12 h-12 text-slate-600 mx-auto" />
      <h3 class="text-base font-semibold text-slate-300">No Decoded APT Images Yet</h3>
      <p class="text-xs text-slate-500 max-w-md mx-auto">
        {i18n.t('gallery.empty')}
      </p>
    </div>
  {:else}
    <div class="grid grid-cols-1 lg:grid-cols-3 gap-6">
      <div class="bg-slate-900/90 border border-slate-800 rounded-2xl p-4 shadow-lg space-y-3">
        <h3 class="text-xs font-bold text-slate-400 uppercase tracking-wider px-2">Decoded Passes ({records.length})</h3>

        <div class="space-y-2 max-h-[500px] overflow-y-auto pr-1">
          {#each records as rec}
            <button
              onclick={() => selectedRecord = rec}
              class="w-full p-3 rounded-xl border text-left transition-all flex items-center justify-between {selectedRecord?.id === rec.id ? 'bg-cyan-950/40 border-cyan-500/50 text-slate-100' : 'bg-slate-950/60 border-slate-800 text-slate-400 hover:border-slate-700'}"
            >
              <div>
                <span class="text-xs font-bold text-slate-200 block">{rec.sat_name}</span>
                <span class="text-[10px] text-cyan-400 font-mono block">{(rec.frequency / 1e6).toFixed(4)} MHz</span>
                <span class="text-[10px] text-slate-500 block">{new Date(rec.timestamp).toLocaleString()}</span>
              </div>

              {#if rec.ai_analysis}
                <span class="p-1.5 bg-emerald-500/20 text-emerald-400 rounded-lg border border-emerald-500/30" title="AI Analysis Ready">
                  <Sparkles class="w-3.5 h-3.5" />
                </span>
              {/if}
            </button>
          {/each}
        </div>
      </div>

      {#if selectedRecord}
        <div class="lg:col-span-2 bg-slate-900/90 border border-slate-800 rounded-2xl p-5 shadow-lg space-y-4">
          <div class="flex items-center justify-between border-b border-slate-800 pb-3">
            <div>
              <h3 class="text-lg font-bold text-slate-100 flex items-center gap-2">
                {selectedRecord.sat_name}
                <span class="text-xs font-mono text-cyan-400 font-normal">({(selectedRecord.frequency / 1e6).toFixed(4)} MHz)</span>
              </h3>
              <p class="text-xs text-slate-400">{new Date(selectedRecord.timestamp).toLocaleString()} • Max Elev: {selectedRecord.max_elevation}°</p>
            </div>

            <div class="flex items-center gap-1 bg-slate-950 p-1 rounded-xl border border-slate-800 text-xs">
              <button
                onclick={() => activeChannel = 'full'}
                class="px-2.5 py-1 rounded-lg font-medium transition-colors {activeChannel === 'full' ? 'bg-cyan-600 text-white' : 'text-slate-400 hover:text-slate-200'}"
              >
                {i18n.t('gallery.viewFull')}
              </button>
              <button
                onclick={() => activeChannel = 'chanA'}
                class="px-2.5 py-1 rounded-lg font-medium transition-colors {activeChannel === 'chanA' ? 'bg-cyan-600 text-white' : 'text-slate-400 hover:text-slate-200'}"
              >
                {i18n.t('gallery.viewChanA')}
              </button>
              <button
                onclick={() => activeChannel = 'chanB'}
                class="px-2.5 py-1 rounded-lg font-medium transition-colors {activeChannel === 'chanB' ? 'bg-cyan-600 text-white' : 'text-slate-400 hover:text-slate-200'}"
              >
                {i18n.t('gallery.viewChanB')}
              </button>
            </div>
          </div>

          <div class="relative bg-slate-950 rounded-xl overflow-hidden border border-slate-800 min-h-[360px] flex items-center justify-center p-2">
            <img
              src={activeChannel === 'full' ? selectedRecord.image_path : (activeChannel === 'chanA' ? selectedRecord.channel_a : selectedRecord.channel_b)}
              alt="APT Satellite Capture"
              class="max-h-[460px] object-contain rounded-lg shadow-xl"
            />
          </div>

          <div class="flex items-center justify-between pt-2">
            <span class="text-xs text-slate-500 font-mono">Image Resolution: 2080px line scan</span>
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
