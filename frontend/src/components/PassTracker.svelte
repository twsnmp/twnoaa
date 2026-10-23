<script>
  import { onMount, onDestroy } from 'svelte'
  import { i18n } from '../i18n.svelte.js'
  import { Satellite, Calendar, RefreshCw, Radio } from '@lucide/svelte'

  let { onSelectPass = () => {} } = $props()

  let satStates = $state([])
  let upcomingPasses = $state([])
  let activeUpcomingPasses = $derived(
    upcomingPasses.filter(p => new Date(p.los).getTime() > Date.now())
  )
  let isLoading = $state(false)
  let autoRecordEnabled = $state(false)
  let autoRecordBanner = $state('')
  let timer
  let bannerTimer
  let pollCount = 0

  function showBanner(msg, autoHideMs = 0) {
    if (bannerTimer) clearTimeout(bannerTimer)
    autoRecordBanner = msg
    if (autoHideMs > 0) {
      bannerTimer = setTimeout(() => {
        autoRecordBanner = ''
      }, autoHideMs)
    }
  }

  async function loadData() {
    if (window.go?.main?.App) {
      try {
        satStates = await window.go.main.App.GetSatelliteStates()
        upcomingPasses = await window.go.main.App.GetUpcomingPasses(24)
        const cfg = await window.go.main.App.GetConfig()
        if (cfg) autoRecordEnabled = cfg.auto_record
      } catch (err) {
        console.error("Failed to load satellite states:", err)
      }
    }
  }

  async function refreshTLEs() {
    isLoading = true
    try {
      if (window.go?.main?.App) {
        await window.go.main.App.RefreshTLEs()
      }
      await loadData()
    } catch (err) {
      console.error("Failed to refresh TLEs:", err)
    } finally {
      isLoading = false
    }
  }

  function formatTime(isoStr) {
    if (!isoStr) return ''
    const d = new Date(isoStr)
    return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' })
  }

  onMount(() => {
    loadData()
    // Poll satellite states every 3 seconds and update pass schedule every 15 seconds
    timer = setInterval(async () => {
      if (window.go?.main?.App) {
        try {
          satStates = await window.go.main.App.GetSatelliteStates()
          pollCount++
          if (pollCount % 5 === 0) {
            upcomingPasses = await window.go.main.App.GetUpcomingPasses(24)
          }
        } catch (err) {
          console.error("Failed to update satellite states:", err)
        }
      }
    }, 3000)

    if (window.runtime?.EventsOn) {
      window.runtime.EventsOn('auto_record_started', (data) => {
        showBanner(`衛星通過検出: ${data.sat_name} の自動録音・デコード (${data.duration}秒) を開始しました`)
      })

      window.runtime.EventsOn('auto_record_ended', (data) => {
        const satName = data?.sat_name || ''
        showBanner(`${satName ? satName + ': ' : ''}${i18n.t('tracker.autoRecordComplete')}`, 4000)
        loadData()
      })

      window.runtime.EventsOn('decode_error', (errMsg) => {
        showBanner(`${i18n.t('tracker.autoRecordFailed')}: ${errMsg}`, 5000)
      })
    }
  })

  onDestroy(() => {
    if (timer) clearInterval(timer)
    if (bannerTimer) clearTimeout(bannerTimer)
  })
</script>

<div class="h-[calc(100vh-65px)] flex flex-col p-5 space-y-4 overflow-hidden">
  <div class="flex items-center justify-between flex-shrink-0">
    <div>
      <h2 class="text-xl font-bold text-slate-100 flex items-center gap-2">
        <Satellite class="w-5 h-5 text-cyan-400" />
        {i18n.t('tracker.liveStatus')}
        {#if autoRecordEnabled}
          <span class="text-[11px] font-semibold px-2.5 py-1 rounded-lg bg-emerald-500/10 text-emerald-400 border border-emerald-500/30 flex items-center gap-1.5 ml-2">
            <span class="w-2 h-2 rounded-full bg-emerald-400 animate-ping"></span>
            {i18n.t('tracker.autoRecordActive')}
          </span>
        {/if}
      </h2>
      <p class="text-xs text-slate-400 mt-0.5">METEOR-M2 3 / M2 4 (137.900 MHz LRPT 稼働中) • NOAA 15 / 18 / 19 (2025年運用終了・停波済)</p>
    </div>

    <button
      onclick={refreshTLEs}
      disabled={isLoading}
      class="px-3.5 py-1.5 bg-slate-800 hover:bg-slate-700 border border-slate-700 rounded-xl text-xs font-semibold text-slate-200 flex items-center gap-2 transition-all disabled:opacity-50"
    >
      <RefreshCw class="w-3.5 h-3.5 text-cyan-400 {isLoading ? 'animate-spin' : ''}" />
      {i18n.t('tracker.refresh')}
    </button>
  </div>

  {#if autoRecordBanner}
    <div class="p-3 bg-emerald-950/40 border border-emerald-800/40 rounded-xl text-xs text-emerald-300 font-medium flex items-center gap-2 flex-shrink-0 animate-pulse">
      <Radio class="w-4 h-4 text-emerald-400" />
      {autoRecordBanner}
    </div>
  {/if}

  <!-- Realtime Satellite Cards -->
  <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-5 gap-3 flex-shrink-0">
    {#each satStates as sat}
      <div class="bg-slate-900/90 border {sat.is_active ? 'border-emerald-500/50 shadow-lg shadow-emerald-500/10' : sat.operational === false ? 'border-slate-800/60 opacity-60' : 'border-slate-800'} rounded-xl p-3 relative overflow-hidden transition-all">
        {#if sat.is_active}
          <div class="absolute top-0 right-0 bg-emerald-500/20 text-emerald-400 text-[9px] font-bold uppercase tracking-wider px-2 py-0.5 rounded-bl-xl border-l border-b border-emerald-500/30 flex items-center gap-1">
            <span class="w-1.5 h-1.5 rounded-full bg-emerald-400 animate-ping"></span>
            {i18n.t('tracker.activePass')}
          </div>
        {:else if sat.operational === false}
          <div class="absolute top-0 right-0 bg-slate-800 text-slate-400 text-[9px] font-semibold px-2 py-0.5 rounded-bl-xl border-l border-b border-slate-700">
            停波済 (EOL)
          </div>
        {:else if sat.mode}
          <div class="absolute top-0 right-0 bg-cyan-500/15 text-cyan-300 text-[9px] font-mono font-bold px-2 py-0.5 rounded-bl-xl border-l border-b border-cyan-500/30">
            {sat.mode}
          </div>
        {/if}

        <div class="flex items-center gap-2 mb-2">
          <div class="p-1.5 rounded-lg {sat.is_active ? 'bg-emerald-500/10 text-emerald-400 border border-emerald-500/30' : 'bg-slate-800 text-slate-400'}">
            <Radio class="w-4 h-4" />
          </div>
          <div class="min-w-0">
            <h3 class="text-xs font-bold text-slate-100 truncate">{sat.name}</h3>
            <p class="text-[10px] font-mono text-cyan-400">{(sat.frequency / 1e6).toFixed(4)} MHz</p>
          </div>
        </div>

        <div class="grid grid-cols-2 gap-2 text-xs">
          <div class="bg-slate-950/60 p-2 rounded-lg border border-slate-800/80">
            <span class="text-slate-400 block text-[9px] uppercase font-semibold">{i18n.t('tracker.elevation')}</span>
            <span class="text-xs font-bold font-mono {sat.elevation >= 5 ? 'text-emerald-400' : 'text-slate-200'}">
              {sat.elevation}°
            </span>
          </div>

          <div class="bg-slate-950/60 p-2 rounded-lg border border-slate-800/80">
            <span class="text-slate-400 block text-[9px] uppercase font-semibold">{i18n.t('tracker.azimuth')}</span>
            <span class="text-xs font-bold font-mono text-slate-200">
              {sat.azimuth}°
            </span>
          </div>

          <div class="bg-slate-950/60 p-2 rounded-lg border border-slate-800/80">
            <span class="text-slate-400 block text-[9px] uppercase font-semibold">{i18n.t('tracker.doppler')}</span>
            <span class="text-xs font-bold font-mono text-amber-400">
              {sat.doppler_hz > 0 ? '+' : ''}{sat.doppler_hz} Hz
            </span>
          </div>

          <div class="bg-slate-950/60 p-2 rounded-lg border border-slate-800/80">
            <span class="text-slate-400 block text-[9px] uppercase font-semibold">{i18n.t('tracker.position')}</span>
            <span class="text-[10px] font-mono text-slate-300">
              {sat.latitude}°, {sat.longitude}°
            </span>
          </div>
        </div>

        {#if sat.is_active}
          <button
            onclick={() => onSelectPass(sat.name, sat.frequency)}
            class="w-full mt-2.5 py-1.5 bg-emerald-600 hover:bg-emerald-500 text-white rounded-lg text-xs font-bold transition-colors shadow-md shadow-emerald-600/30 flex items-center justify-center gap-1.5"
          >
            <Radio class="w-3.5 h-3.5" />
            {i18n.t('tracker.tuneNow')}
          </button>
        {/if}
      </div>
    {/each}
  </div>

  <!-- Upcoming Passes Table -->
  <div class="bg-slate-900/90 border border-slate-800 rounded-2xl p-4 shadow-lg flex-1 flex flex-col min-h-0 overflow-hidden">
    <h3 class="text-sm font-bold text-slate-100 flex items-center gap-2 flex-shrink-0 mb-3">
      <Calendar class="w-4 h-4 text-cyan-400" />
      {i18n.t('tracker.upcomingPasses')}
    </h3>

    <div class="flex-1 overflow-y-auto overflow-x-auto rounded-xl border border-slate-800/80 bg-slate-950/40">
      <table class="w-full text-left text-xs">
        <thead class="bg-slate-950 text-slate-400 uppercase font-semibold text-[10px] border-b border-slate-800 sticky top-0 z-10">
          <tr>
            <th class="py-2.5 px-3.5">{i18n.t('tracker.satName')}</th>
            <th class="py-2.5 px-3.5">{i18n.t('tracker.freq')}</th>
            <th class="py-2.5 px-3.5">{i18n.t('tracker.aos')}</th>
            <th class="py-2.5 px-3.5">{i18n.t('tracker.los')}</th>
            <th class="py-2.5 px-3.5">{i18n.t('tracker.maxElev')}</th>
            <th class="py-2.5 px-3.5">{i18n.t('tracker.duration')}</th>
            <th class="py-2.5 px-3.5 text-right">{i18n.t('tracker.action')}</th>
          </tr>
        </thead>
        <tbody class="divide-y divide-slate-800/60 font-mono text-slate-300">
          {#each activeUpcomingPasses as pass}
            <tr class="hover:bg-slate-800/40 transition-colors">
              <td class="py-1.5 px-3.5 font-bold text-slate-100 font-sans">{pass.sat_name}</td>
              <td class="py-1.5 px-3.5 text-cyan-400">{(pass.frequency / 1e6).toFixed(4)} MHz</td>
              <td class="py-1.5 px-3.5 text-emerald-400">{formatTime(pass.aos)}</td>
              <td class="py-1.5 px-3.5 text-rose-400">{formatTime(pass.los)}</td>
              <td class="py-1.5 px-3.5">
                <span class="px-2 py-0.5 rounded bg-cyan-500/10 text-cyan-300 font-bold border border-cyan-500/20 text-[11px]">
                  {pass.max_elev}° ({pass.max_elev_az}°)
                </span>
              </td>
              <td class="py-1.5 px-3.5 text-slate-400">{Math.round(pass.duration_sec / 60)} min</td>
              <td class="py-1.5 px-3.5 text-right">
                <button
                  onclick={() => onSelectPass(pass.sat_name, pass.frequency)}
                  class="px-2 py-0.5 bg-slate-800 hover:bg-cyan-600 text-slate-300 hover:text-white rounded-lg text-[11px] font-sans font-medium transition-colors"
                >
                  {i18n.t('tracker.tune')}
                </button>
              </td>
            </tr>
          {/each}

          {#if activeUpcomingPasses.length === 0}
            <tr>
              <td colspan="7" class="py-6 text-center text-slate-500 font-sans">
                {i18n.t('tracker.noPasses')}
              </td>
            </tr>
          {/if}
        </tbody>
      </table>
    </div>
  </div>
</div>
