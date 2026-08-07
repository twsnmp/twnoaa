<script>
  import { onMount, onDestroy } from 'svelte'
  import { i18n } from '../i18n.svelte.js'
  import { Satellite, Compass, Activity, Calendar, RefreshCw, CheckCircle2, Radio } from '@lucide/svelte'

  let { onSelectPass = () => {} } = $props()

  let satStates = $state([])
  let upcomingPasses = $state([])
  let isLoading = $state(false)
  let timer

  async function loadData() {
    if (window.go?.main?.App) {
      try {
        satStates = await window.go.main.App.GetSatelliteStates()
        upcomingPasses = await window.go.main.App.GetUpcomingPasses(24)
      } catch (err) {
        console.error("Failed to load satellite states:", err)
      }
    }
  }

  async function refreshTLEs() {
    isLoading = true
    await loadData()
    isLoading = false
  }

  onMount(() => {
    loadData()
    timer = setInterval(loadData, 3000)
  })

  onDestroy(() => {
    if (timer) clearInterval(timer)
  })
</script>

<div class="p-6 space-y-6 overflow-y-auto max-h-[calc(100vh-65px)]">
  <div class="flex items-center justify-between">
    <div>
      <h2 class="text-xl font-bold text-slate-100 flex items-center gap-2">
        <Satellite class="w-5 h-5 text-cyan-400" />
        {i18n.t('tracker.liveStatus')}
      </h2>
      <p class="text-xs text-slate-400 mt-1">NOAA 15 (137.620 MHz) • NOAA 18 (137.9125 MHz) • NOAA 19 (137.100 MHz)</p>
    </div>

    <button
      onclick={refreshTLEs}
      disabled={isLoading}
      class="px-4 py-2 bg-slate-800 hover:bg-slate-700 border border-slate-700 rounded-xl text-xs font-semibold text-slate-200 flex items-center gap-2 transition-all disabled:opacity-50"
    >
      <RefreshCw class="w-3.5 h-3.5 text-cyan-400 {isLoading ? 'animate-spin' : ''}" />
      {i18n.t('tracker.refresh')}
    </button>
  </div>

  <!-- Realtime Satellite Cards -->
  <div class="grid grid-cols-1 md:grid-cols-3 gap-4">
    {#each satStates as sat}
      <div class="bg-slate-900/90 border {sat.is_active ? 'border-emerald-500/50 shadow-lg shadow-emerald-500/10' : 'border-slate-800'} rounded-2xl p-5 relative overflow-hidden transition-all">
        {#if sat.is_active}
          <div class="absolute top-0 right-0 bg-emerald-500/20 text-emerald-400 text-[10px] font-bold uppercase tracking-wider px-3 py-1 rounded-bl-xl border-l border-b border-emerald-500/30 flex items-center gap-1.5">
            <span class="w-2 h-2 rounded-full bg-emerald-400 animate-ping"></span>
            ACTIVE PASS
          </div>
        {/if}

        <div class="flex items-center gap-3 mb-4">
          <div class="p-3 rounded-xl {sat.is_active ? 'bg-emerald-500/10 text-emerald-400 border border-emerald-500/30' : 'bg-slate-800 text-slate-400'}">
            <Radio class="w-6 h-6" />
          </div>
          <div>
            <h3 class="text-base font-bold text-slate-100">{sat.name}</h3>
            <p class="text-xs font-mono text-cyan-400">{(sat.frequency / 1e6).toFixed(4)} MHz</p>
          </div>
        </div>

        <div class="grid grid-cols-2 gap-3 text-xs">
          <div class="bg-slate-950/60 p-2.5 rounded-xl border border-slate-800/80">
            <span class="text-slate-400 block text-[10px] uppercase font-semibold">{i18n.t('tracker.elevation')}</span>
            <span class="text-sm font-bold font-mono {sat.elevation >= 10 ? 'text-emerald-400' : 'text-slate-200'}">
              {sat.elevation}°
            </span>
          </div>

          <div class="bg-slate-950/60 p-2.5 rounded-xl border border-slate-800/80">
            <span class="text-slate-400 block text-[10px] uppercase font-semibold">{i18n.t('tracker.azimuth')}</span>
            <span class="text-sm font-bold font-mono text-slate-200">
              {sat.azimuth}°
            </span>
          </div>

          <div class="bg-slate-950/60 p-2.5 rounded-xl border border-slate-800/80">
            <span class="text-slate-400 block text-[10px] uppercase font-semibold">{i18n.t('tracker.doppler')}</span>
            <span class="text-sm font-bold font-mono text-amber-400">
              {sat.doppler_hz > 0 ? '+' : ''}{sat.doppler_hz} Hz
            </span>
          </div>

          <div class="bg-slate-950/60 p-2.5 rounded-xl border border-slate-800/80">
            <span class="text-slate-400 block text-[10px] uppercase font-semibold">Position</span>
            <span class="text-[11px] font-mono text-slate-300">
              {sat.latitude}°, {sat.longitude}°
            </span>
          </div>
        </div>

        {#if sat.is_active}
          <button
            onclick={() => onSelectPass(sat.name, sat.frequency)}
            class="w-full mt-4 py-2 bg-emerald-600 hover:bg-emerald-500 text-white rounded-xl text-xs font-bold transition-colors shadow-md shadow-emerald-600/30 flex items-center justify-center gap-1.5"
          >
            <Radio class="w-3.5 h-3.5" />
            Tune & Receive Now
          </button>
        {/if}
      </div>
    {/each}
  </div>

  <!-- Upcoming Passes Table -->
  <div class="bg-slate-900/90 border border-slate-800 rounded-2xl p-5 shadow-lg space-y-4">
    <h3 class="text-base font-bold text-slate-100 flex items-center gap-2">
      <Calendar class="w-4 h-4 text-cyan-400" />
      {i18n.t('tracker.upcomingPasses')}
    </h3>

    <div class="overflow-x-auto">
      <table class="w-full text-left text-xs">
        <thead class="bg-slate-950 text-slate-400 uppercase font-semibold text-[10px] border-b border-slate-800">
          <tr>
            <th class="py-3 px-4">{i18n.t('tracker.satName')}</th>
            <th class="py-3 px-4">{i18n.t('tracker.freq')}</th>
            <th class="py-3 px-4">{i18n.t('tracker.aos')}</th>
            <th class="py-3 px-4">{i18n.t('tracker.los')}</th>
            <th class="py-3 px-4">{i18n.t('tracker.maxElev')}</th>
            <th class="py-3 px-4">{i18n.t('tracker.duration')}</th>
            <th class="py-3 px-4 text-right">Action</th>
          </tr>
        </thead>
        <tbody class="divide-y divide-slate-800/60 font-mono text-slate-300">
          {#each upcomingPasses as pass}
            <tr class="hover:bg-slate-800/40 transition-colors">
              <td class="py-3 px-4 font-bold text-slate-100 font-sans">{pass.sat_name}</td>
              <td class="py-3 px-4 text-cyan-400">{(pass.frequency / 1e6).toFixed(4)} MHz</td>
              <td class="py-3 px-4 text-emerald-400">{new Date(pass.aos).toLocaleTimeString()}</td>
              <td class="py-3 px-4 text-rose-400">{new Date(pass.los).toLocaleTimeString()}</td>
              <td class="py-3 px-4">
                <span class="px-2 py-0.5 rounded-md bg-cyan-500/10 text-cyan-300 font-bold border border-cyan-500/20">
                  {pass.max_elev}° ({pass.max_elev_az}°)
                </span>
              </td>
              <td class="py-3 px-4 text-slate-400">{Math.round(pass.duration_sec / 60)} min</td>
              <td class="py-3 px-4 text-right">
                <button
                  onclick={() => onSelectPass(pass.sat_name, pass.frequency)}
                  class="px-2.5 py-1 bg-slate-800 hover:bg-cyan-600 text-slate-300 hover:text-white rounded-lg text-[11px] font-sans font-medium transition-colors"
                >
                  Tune Receiver
                </button>
              </td>
            </tr>
          {/each}

          {#if upcomingPasses.length === 0}
            <tr>
              <td colspan="7" class="py-8 text-center text-slate-500 font-sans">
                No upcoming satellite passes detected in the next 24 hours.
              </td>
            </tr>
          {/if}
        </tbody>
      </table>
    </div>
  </div>
</div>
