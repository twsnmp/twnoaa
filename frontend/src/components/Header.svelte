<script>
  import { i18n } from '../i18n.svelte.js'
  import { Radio, Satellite, Image, Bot, Settings, Globe } from '@lucide/svelte'

  let { activeTab = 'tracker', onTabChange = () => {}, isStreaming = false } = $props()

  function toggleLang() {
    i18n.setLocale(i18n.currentLocale === 'ja' ? 'en' : 'ja')
  }
</script>

<header class="bg-slate-950 border-b border-slate-800 px-6 py-3.5 flex items-center justify-between shadow-lg">
  <div class="flex items-center gap-3">
    <div class="p-2 bg-cyan-500/10 border border-cyan-500/30 rounded-xl text-cyan-400">
      <Satellite class="w-6 h-6 animate-pulse" />
    </div>
    <div>
      <h1 class="text-xl font-bold text-slate-100 tracking-tight flex items-center gap-2">
        twnoaa
        <span class="text-xs px-2 py-0.5 rounded-full bg-cyan-500/20 text-cyan-300 font-mono border border-cyan-500/30">
          v1.0.0
        </span>
      </h1>
      <p class="text-xs text-slate-400 font-medium">NOAA Weather Satellite Receiver & AI Engine</p>
    </div>
  </div>

  <nav class="flex items-center gap-1.5 bg-slate-900/80 p-1.5 rounded-xl border border-slate-800">
    <button
      onclick={() => onTabChange('tracker')}
      class="px-3.5 py-1.5 rounded-lg text-xs font-semibold flex items-center gap-2 transition-all {activeTab === 'tracker' ? 'bg-cyan-600 text-white shadow-md shadow-cyan-600/30' : 'text-slate-400 hover:text-slate-200 hover:bg-slate-800/60'}"
    >
      <Satellite class="w-4 h-4" />
      {i18n.t('tabs.tracker')}
    </button>

    <button
      onclick={() => onTabChange('sdr')}
      class="px-3.5 py-1.5 rounded-lg text-xs font-semibold flex items-center gap-2 transition-all {activeTab === 'sdr' ? 'bg-cyan-600 text-white shadow-md shadow-cyan-600/30' : 'text-slate-400 hover:text-slate-200 hover:bg-slate-800/60'}"
    >
      <Radio class="w-4 h-4" />
      {i18n.t('tabs.sdr')}
      {#if isStreaming}
        <span class="w-2 h-2 rounded-full bg-emerald-400 animate-ping"></span>
      {/if}
    </button>

    <button
      onclick={() => onTabChange('gallery')}
      class="px-3.5 py-1.5 rounded-lg text-xs font-semibold flex items-center gap-2 transition-all {activeTab === 'gallery' ? 'bg-cyan-600 text-white shadow-md shadow-cyan-600/30' : 'text-slate-400 hover:text-slate-200 hover:bg-slate-800/60'}"
    >
      <Image class="w-4 h-4" />
      {i18n.t('tabs.gallery')}
    </button>

    <button
      onclick={() => onTabChange('ai')}
      class="px-3.5 py-1.5 rounded-lg text-xs font-semibold flex items-center gap-2 transition-all {activeTab === 'ai' ? 'bg-cyan-600 text-white shadow-md shadow-cyan-600/30' : 'text-slate-400 hover:text-slate-200 hover:bg-slate-800/60'}"
    >
      <Bot class="w-4 h-4" />
      {i18n.t('tabs.ai')}
    </button>

    <button
      onclick={() => onTabChange('settings')}
      class="px-3.5 py-1.5 rounded-lg text-xs font-semibold flex items-center gap-2 transition-all {activeTab === 'settings' ? 'bg-cyan-600 text-white shadow-md shadow-cyan-600/30' : 'text-slate-400 hover:text-slate-200 hover:bg-slate-800/60'}"
    >
      <Settings class="w-4 h-4" />
      {i18n.t('tabs.settings')}
    </button>
  </nav>

  <div class="flex items-center gap-3">
    <button
      onclick={toggleLang}
      class="px-3 py-1.5 bg-slate-900 border border-slate-800 hover:border-slate-700 rounded-lg text-xs font-semibold text-slate-300 flex items-center gap-1.5 transition-colors"
    >
      <Globe class="w-3.5 h-3.5 text-cyan-400" />
      {i18n.currentLocale.toUpperCase()}
    </button>
  </div>
</header>
