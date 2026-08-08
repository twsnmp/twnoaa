<script>
  import { onMount } from 'svelte'
  import { i18n } from '../i18n.svelte.js'
  import { Image as ImageIcon, Bot, Sparkles, Trash2, AlertTriangle, X } from '@lucide/svelte'

  let { onRunAI = () => {} } = $props()

  let records = $state([])
  let selectedRecord = $state(null)
  let activeChannel = $state('full')
  let currentImageBase64 = $state('')
  let recordToDelete = $state(null)
  let viewMode = $state('stretch') // 'stretch' | 'fit'
  let showZoomModal = $state(false)

  async function loadRecords() {
    if (window.go?.main?.App) {
      try {
        const fetched = await window.go.main.App.GetPassRecords()
        records = (fetched && Array.isArray(fetched)) ? [...fetched] : []
        if (records.length > 0) {
          if (!selectedRecord || !records.find(r => r.id === selectedRecord.id)) {
            selectedRecord = records[0]
          }
        } else {
          selectedRecord = null
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
  <div class="flex items-center justify-between">
    <div>
      <h2 class="text-xl font-bold text-slate-100 flex items-center gap-2">
        <ImageIcon class="w-5 h-5 text-cyan-400" />
        {i18n.t('gallery.title')}
      </h2>
      <p class="text-xs text-slate-400 mt-1">{i18n.t('gallery.subtitle')}</p>
    </div>
  </div>

  {#if records.length === 0}
    <div class="bg-slate-900/90 border border-slate-800 rounded-2xl p-12 text-center space-y-3">
      <ImageIcon class="w-12 h-12 text-slate-600 mx-auto" />
      <h3 class="text-base font-semibold text-slate-300">{i18n.t('gallery.noImages')}</h3>
      <p class="text-xs text-slate-500 max-w-md mx-auto">
        {i18n.t('gallery.empty')}
      </p>
    </div>
  {:else}
    <div class="grid grid-cols-1 lg:grid-cols-3 gap-6">
      <div class="bg-slate-900/90 border border-slate-800 rounded-2xl p-4 shadow-lg space-y-3">
        <h3 class="text-xs font-bold text-slate-400 uppercase tracking-wider px-2">{i18n.t('gallery.decodedPasses')} ({records.length})</h3>

        <div class="space-y-2 max-h-[500px] overflow-y-auto pr-1">
          {#each records as rec}
            <div
              role="button"
              tabindex="0"
              onclick={() => selectedRecord = rec}
              onkeydown={(e) => e.key === 'Enter' && (selectedRecord = rec)}
              class="w-full p-3 rounded-xl border text-left transition-all flex items-center justify-between group cursor-pointer {selectedRecord?.id === rec.id ? 'bg-cyan-950/40 border-cyan-500/50 text-slate-100' : 'bg-slate-950/60 border-slate-800 text-slate-400 hover:border-slate-700'}"
            >
              <div>
                <span class="text-xs font-bold text-slate-200 block">{rec.sat_name}</span>
                <span class="text-[10px] text-cyan-400 font-mono block">{(rec.frequency / 1e6).toFixed(4)} MHz</span>
                <span class="text-[10px] text-slate-500 block">{new Date(rec.timestamp).toLocaleString()}</span>
              </div>

              <div class="flex items-center gap-1.5">
                {#if rec.ai_analysis}
                  <span class="p-1.5 bg-emerald-500/20 text-emerald-400 rounded-lg border border-emerald-500/30" title={i18n.t('gallery.aiReady')}>
                    <Sparkles class="w-3.5 h-3.5" />
                  </span>
                {/if}

                <button
                  type="button"
                  onclick={(e) => promptDelete(rec, e)}
                  class="p-1.5 text-slate-500 hover:text-red-400 hover:bg-red-500/10 rounded-lg border border-transparent hover:border-red-500/20 transition-all"
                  title={i18n.t('gallery.delete')}
                >
                  <Trash2 class="w-3.5 h-3.5" />
                </button>
              </div>
            </div>
          {/each}
        </div>
      </div>

      {#if selectedRecord}
        <div class="lg:col-span-2 bg-slate-900/90 border border-slate-800 rounded-2xl p-5 shadow-lg space-y-4">
          <div class="flex flex-wrap items-center justify-between gap-4 border-b border-slate-800 pb-3">
            <div>
              <h3 class="text-lg font-bold text-slate-100 flex items-center gap-2 whitespace-nowrap">
                {selectedRecord.sat_name}
                <span class="text-xs font-mono text-cyan-400 font-normal">({(selectedRecord.frequency / 1e6).toFixed(4)} MHz)</span>
              </h3>
              <p class="text-xs text-slate-400">{new Date(selectedRecord.timestamp).toLocaleString()} • {i18n.t('gallery.maxElevLabel')} {selectedRecord.max_elevation}°</p>
            </div>

            <div class="flex items-center gap-3">
              <div class="flex items-center gap-1 bg-slate-950 p-1 rounded-xl border border-slate-800 text-xs">
                <button
                  onclick={() => viewMode = 'stretch'}
                  class="px-2.5 py-1 rounded-lg font-medium transition-colors {viewMode === 'stretch' ? 'bg-indigo-600 text-white' : 'text-slate-400 hover:text-slate-200'}"
                  title={i18n.t('gallery.stretchY')}
                >
                  {i18n.t('gallery.stretchY')}
                </button>
                <button
                  onclick={() => viewMode = 'fit'}
                  class="px-2.5 py-1 rounded-lg font-medium transition-colors {viewMode === 'fit' ? 'bg-indigo-600 text-white' : 'text-slate-400 hover:text-slate-200'}"
                  title={i18n.t('gallery.fitAspect')}
                >
                  {i18n.t('gallery.fitAspect')}
                </button>
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

              <button
                type="button"
                onclick={(e) => promptDelete(selectedRecord, e)}
                class="px-3 py-1.5 bg-red-500/10 hover:bg-red-500/20 text-red-400 rounded-xl border border-red-500/30 text-xs font-medium flex items-center gap-1.5 transition-all"
                title={i18n.t('gallery.delete')}
              >
                <Trash2 class="w-3.5 h-3.5" />
                {i18n.t('gallery.delete')}
              </button>
            </div>
          </div>

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
