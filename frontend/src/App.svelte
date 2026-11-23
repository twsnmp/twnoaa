<script>
  import { onMount, onDestroy } from 'svelte'
  import { AlertTriangle, X } from '@lucide/svelte'
  import { i18n } from './i18n.svelte.js'
  import Header from './components/Header.svelte'
  import PassTracker from './components/PassTracker.svelte'
  import Globe from './components/Globe.svelte'
  import SDRControl from './components/SDRControl.svelte'
  import ImageGallery from './components/ImageGallery.svelte'
  import AIAnalysis from './components/AIAnalysis.svelte'
  import Settings from './components/Settings.svelte'
  import APRSMonitor from './components/APRSMonitor.svelte'

  let activeTab = $state('tracker')
  let selectedFreq = $state(137900000)
  let selectedSat = $state('METEOR-M2 4')
  let selectedRecordForAI = $state(null)
  let sdrAlert = $state({ active: false, fail_count: 0, message: '' })

  onMount(async () => {
    if (window.go?.main?.App?.GetSDRAlert) {
      try {
        const alert = await window.go.main.App.GetSDRAlert()
        if (alert) sdrAlert = alert
      } catch (err) {
        console.error('Failed to get SDR alert:', err)
      }
    }
    if (window.runtime?.EventsOn) {
      window.runtime.EventsOn('sdr_alert', (alert) => {
        if (alert) sdrAlert = alert
      })
    }
  })

  onDestroy(() => {
    if (window.runtime?.EventsOff) {
      window.runtime.EventsOff('sdr_alert')
    }
  })

  async function dismissSDRAlert() {
    sdrAlert = { active: false, fail_count: 0, message: '' }
    if (window.go?.main?.App?.ClearSDRAlert) {
      try {
        await window.go.main.App.ClearSDRAlert()
      } catch (err) {
        console.error('Failed to clear SDR alert:', err)
      }
    }
  }

  function handleSelectPass(satName, freq) {
    selectedSat = satName
    selectedFreq = freq
    activeTab = 'sdr'
  }

  function handleRunAI(record) {
    selectedRecordForAI = record
    activeTab = 'ai'
  }

  function handleDecoded(record) {
    selectedRecordForAI = record
    activeTab = 'gallery'
  }
</script>

<div class="h-screen w-screen flex flex-col bg-slate-950 text-slate-100 overflow-hidden">
  <Header {activeTab} onTabChange={(tab) => activeTab = tab} />

  {#if sdrAlert?.active}
    <div class="bg-rose-950/90 border-b border-rose-500/60 px-6 py-2.5 flex items-center justify-between gap-4 text-rose-100 shadow-lg">
      <div class="flex items-center gap-3 min-w-0">
        <AlertTriangle class="w-5 h-5 text-rose-400 shrink-0 animate-pulse" />
        <div class="min-w-0">
          <div class="text-xs font-bold text-rose-200 flex items-center gap-2">
            <span>{i18n.t('sdrAlert.title')}</span>
            {#if sdrAlert.message}
              <span class="font-mono text-[11px] px-1.5 py-0.5 rounded bg-rose-900/80 text-rose-300 truncate max-w-md" title={sdrAlert.message}>
                {sdrAlert.message}
              </span>
            {/if}
          </div>
          <p class="text-[11px] text-rose-200/90 mt-0.5">
            {i18n.t('sdrAlert.desc', { count: sdrAlert.fail_count || 1 })}
          </p>
        </div>
      </div>
      <button
        onclick={dismissSDRAlert}
        class="px-2.5 py-1 rounded-md text-xs font-semibold bg-rose-900/70 hover:bg-rose-800 text-rose-100 border border-rose-500/40 flex items-center gap-1 shrink-0 transition-colors"
      >
        <X class="w-3.5 h-3.5" />
        {i18n.t('sdrAlert.dismiss')}
      </button>
    </div>
  {/if}

  <main class="flex-1 overflow-hidden relative">
    {#if activeTab === 'tracker'}
      <PassTracker onSelectPass={handleSelectPass} />
    {:else if activeTab === 'globe'}
      <Globe onSelectPass={handleSelectPass} />
    {:else if activeTab === 'sdr'}
      <SDRControl initialFreq={selectedFreq} initialSat={selectedSat} onDecoded={handleDecoded} />
    {:else if activeTab === 'aprs'}
      <APRSMonitor />
    {:else if activeTab === 'gallery'}
      <ImageGallery onRunAI={handleRunAI} />
    {:else if activeTab === 'ai'}
      <AIAnalysis record={selectedRecordForAI} />
    {:else if activeTab === 'settings'}
      <Settings />
    {/if}
  </main>
</div>
