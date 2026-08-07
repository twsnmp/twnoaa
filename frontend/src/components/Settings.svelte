<script>
  import { onMount } from 'svelte'
  import { i18n } from '../i18n.svelte.js'
  import { Settings, MapPin, Sliders, Key, Save, RefreshCw, CheckCircle2 } from '@lucide/svelte'

  let cfg = $state({
    observer_lat: 36.028,
    observer_lon: 139.533,
    observer_alt: 0.02,
    observer_city: 'Kitamoto, Saitama',
    active_provider: 'ollama',
    api_key_gemini: '',
    api_key_openai: '',
    api_key_anthropic: '',
    ollama_url: 'http://localhost:11434',
    ollama_model: 'llama3',
    auto_analyze: true,
    simulation_mode: false,
    auto_record: false,
    auto_record_duration: 30,
    tuner_gain: 400,
    sample_rate: 2080000
  })

  let saveMsg = $state('')
  let isFetchingLoc = $state(false)

  async function loadConfig() {
    if (window.go?.main?.App) {
      try {
        const loaded = await window.go.main.App.GetConfig()
        if (loaded) cfg = loaded
      } catch (err) {
        console.error("Failed to load config:", err)
      }
    }
  }

  async function saveConfig() {
    if (!window.go?.main?.App) return
    try {
      saveMsg = await window.go.main.App.SaveConfig(cfg)
      setTimeout(() => saveMsg = '', 3000)
    } catch (err) {
      saveMsg = 'Error: ' + err
    }
  }

  async function autoDetectLocation() {
    if (!window.go?.main?.App) return
    isFetchingLoc = true
    try {
      const loc = await window.go.main.App.FetchIPLocation()
      if (loc) {
        cfg.observer_lat = loc.lat
        cfg.observer_lon = loc.lon
        cfg.observer_city = loc.city
        saveMsg = i18n.currentLocale === 'ja' ? '位置情報を更新しました: ' + loc.city : 'Location updated: ' + loc.city
      }
    } catch (err) {
      saveMsg = i18n.currentLocale === 'ja' ? '位置情報の取得に失敗しました: ' + err : 'Failed to fetch location: ' + err
    } finally {
      isFetchingLoc = false
    }
  }

  onMount(() => {
    loadConfig()
  })
</script>

<div class="p-6 space-y-6 overflow-y-auto max-h-[calc(100vh-65px)]">
  <div class="flex items-center justify-between">
    <div>
      <h2 class="text-xl font-bold text-slate-100 flex items-center gap-2">
        <Settings class="w-5 h-5 text-cyan-400" />
        {i18n.t('settings.title')}
      </h2>
      <p class="text-xs text-slate-400 mt-1">{i18n.t('settings.subtitle')}</p>
    </div>

    <button
      onclick={saveConfig}
      class="px-5 py-2.5 bg-gradient-to-r from-cyan-600 to-emerald-600 hover:from-cyan-500 hover:to-emerald-500 text-white rounded-xl text-xs font-bold shadow-lg shadow-cyan-600/30 flex items-center gap-2 transition-all"
    >
      <Save class="w-4 h-4" />
      {i18n.t('settings.save')}
    </button>
  </div>

  {#if saveMsg}
    <div class="p-3 bg-emerald-950/40 border border-emerald-800/40 rounded-xl text-xs text-emerald-300 font-medium flex items-center gap-2">
      <CheckCircle2 class="w-4 h-4 text-emerald-400" />
      {saveMsg}
    </div>
  {/if}

  <div class="grid grid-cols-1 md:grid-cols-2 gap-6">
    <!-- Observer Location Card -->
    <div class="bg-slate-900/90 border border-slate-800 rounded-2xl p-5 shadow-lg space-y-4">
      <div class="flex items-center justify-between border-b border-slate-800 pb-3">
        <h3 class="text-base font-bold text-slate-100 flex items-center gap-2">
          <MapPin class="w-4 h-4 text-cyan-400" />
          {i18n.t('settings.observerTitle')}
        </h3>

        <button
          onclick={autoDetectLocation}
          disabled={isFetchingLoc}
          class="px-3 py-1.5 bg-slate-950 hover:bg-slate-800 border border-slate-800 rounded-lg text-[11px] font-semibold text-cyan-400 flex items-center gap-1.5 transition-colors disabled:opacity-50"
        >
          <RefreshCw class="w-3 h-3 {isFetchingLoc ? 'animate-spin' : ''}" />
          {i18n.t('settings.autoLocation')}
        </button>
      </div>

      <div>
        <label for="city-input" class="text-xs font-semibold text-slate-400 uppercase tracking-wider block mb-1">{i18n.t('settings.observerCity')}</label>
        <input
          id="city-input"
          type="text"
          bind:value={cfg.observer_city}
          class="w-full bg-slate-950 border border-slate-800 rounded-xl px-3.5 py-2 text-xs font-medium text-slate-200 focus:outline-none focus:border-cyan-500"
        />
      </div>

      <div class="grid grid-cols-3 gap-3">
        <div>
          <label for="lat-input" class="text-xs font-semibold text-slate-400 block mb-1">{i18n.t('settings.lat')}</label>
          <input
            id="lat-input"
            type="number"
            step="0.001"
            bind:value={cfg.observer_lat}
            class="w-full bg-slate-950 border border-slate-800 rounded-xl px-3 py-2 text-xs font-mono text-cyan-400 focus:outline-none focus:border-cyan-500"
          />
        </div>

        <div>
          <label for="lon-input" class="text-xs font-semibold text-slate-400 block mb-1">{i18n.t('settings.lon')}</label>
          <input
            id="lon-input"
            type="number"
            step="0.001"
            bind:value={cfg.observer_lon}
            class="w-full bg-slate-950 border border-slate-800 rounded-xl px-3 py-2 text-xs font-mono text-cyan-400 focus:outline-none focus:border-cyan-500"
          />
        </div>

        <div>
          <label for="alt-input" class="text-xs font-semibold text-slate-400 block mb-1">{i18n.t('settings.alt')}</label>
          <input
            id="alt-input"
            type="number"
            step="0.01"
            bind:value={cfg.observer_alt}
            class="w-full bg-slate-950 border border-slate-800 rounded-xl px-3 py-2 text-xs font-mono text-cyan-400 focus:outline-none focus:border-cyan-500"
          />
        </div>
      </div>
    </div>

    <!-- SDR Settings Card -->
    <div class="bg-slate-900/90 border border-slate-800 rounded-2xl p-5 shadow-lg space-y-4">
      <h3 class="text-base font-bold text-slate-100 flex items-center gap-2 border-b border-slate-800 pb-3">
        <Sliders class="w-4 h-4 text-cyan-400" />
        {i18n.t('settings.sdrConfig')}
      </h3>

      <label for="sim-checkbox" class="flex items-center gap-3 p-3 bg-slate-950 rounded-xl border border-slate-800 cursor-pointer">
        <input
          id="sim-checkbox"
          type="checkbox"
          bind:checked={cfg.simulation_mode}
          class="rounded bg-slate-900 border-slate-700 text-cyan-500 focus:ring-cyan-500"
        />
        <div>
          <span class="text-xs font-bold text-slate-200 block">{i18n.t('settings.simMode')}</span>
          <span class="text-[10px] text-slate-500 block">{i18n.t('settings.simDesc')}</span>
        </div>
      </label>

      <label for="auto-record-checkbox" class="flex items-center gap-3 p-3 bg-slate-950 rounded-xl border border-slate-800 cursor-pointer">
        <input
          id="auto-record-checkbox"
          type="checkbox"
          bind:checked={cfg.auto_record}
          class="rounded bg-slate-900 border-slate-700 text-cyan-500 focus:ring-cyan-500"
        />
        <div>
          <span class="text-xs font-bold text-slate-200 block">{i18n.t('settings.autoRecord')}</span>
          <span class="text-[10px] text-slate-500 block">{i18n.t('settings.autoRecordDesc')}</span>
        </div>
      </label>

      {#if cfg.auto_record}
        <div>
          <label for="auto-record-duration" class="text-xs font-semibold text-slate-400 uppercase tracking-wider block mb-1">{i18n.t('settings.autoRecordDuration')}</label>
          <input
            id="auto-record-duration"
            type="number"
            bind:value={cfg.auto_record_duration}
            min="10"
            max="600"
            step="5"
            class="w-full bg-slate-950 border border-slate-800 rounded-xl px-3.5 py-2 text-xs font-mono text-cyan-400 focus:outline-none focus:border-cyan-500"
          />
        </div>
      {/if}

      <div>
        <label for="default-gain-input" class="text-xs font-semibold text-slate-400 uppercase tracking-wider block mb-1">{i18n.t('settings.defaultGain')}</label>
        <input
          id="default-gain-input"
          type="number"
          bind:value={cfg.tuner_gain}
          class="w-full bg-slate-950 border border-slate-800 rounded-xl px-3.5 py-2 text-xs font-mono text-cyan-400 focus:outline-none focus:border-cyan-500"
        />
      </div>
    </div>

    <!-- LLM Provider Configuration Card -->
    <div class="md:col-span-2 bg-slate-900/90 border border-slate-800 rounded-2xl p-5 shadow-lg space-y-5">
      <h3 class="text-base font-bold text-slate-100 flex items-center gap-2 border-b border-slate-800 pb-3">
        <Key class="w-4 h-4 text-cyan-400" />
        {i18n.t('settings.llmConfig')}
      </h3>

      <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
        <div>
          <label for="provider-select" class="text-xs font-semibold text-slate-400 uppercase tracking-wider block mb-1">{i18n.t('settings.activeProvider')}</label>
          <select
            id="provider-select"
            bind:value={cfg.active_provider}
            class="w-full bg-slate-950 border border-slate-800 rounded-xl px-3.5 py-2 text-xs font-bold text-slate-200 focus:outline-none focus:border-cyan-500"
          >
            <option value="ollama">Ollama (Local LLM)</option>
            <option value="gemini">Google Gemini</option>
            <option value="openai">OpenAI (GPT-4o / GPT-4o-mini)</option>
            <option value="anthropic">Anthropic Claude</option>
          </select>
        </div>

        <div class="flex items-center">
          <label for="auto-analyze-checkbox" class="flex items-center gap-3 p-3 bg-slate-950 rounded-xl border border-slate-800 cursor-pointer w-full">
            <input
              id="auto-analyze-checkbox"
              type="checkbox"
              bind:checked={cfg.auto_analyze}
              class="rounded bg-slate-900 border-slate-700 text-cyan-500 focus:ring-cyan-500"
            />
            <div>
              <span class="text-xs font-bold text-slate-200 block">{i18n.t('ai.autoAnalyze')}</span>
              <span class="text-[10px] text-slate-500 block">{i18n.t('settings.autoAnalyzeDesc')}</span>
            </div>
          </label>
        </div>
      </div>

      <!-- Dynamic LLM Fields: Show only settings relevant to active_provider -->
      <div class="grid grid-cols-1 md:grid-cols-2 gap-4 pt-2 border-t border-slate-800/80">
        {#if cfg.active_provider === 'ollama'}
          <div>
            <label for="ollama-url-input" class="text-xs font-semibold text-slate-400 block mb-1">{i18n.t('settings.ollamaUrl')}</label>
            <input
              id="ollama-url-input"
              type="text"
              bind:value={cfg.ollama_url}
              class="w-full bg-slate-950 border border-slate-800 rounded-xl px-3.5 py-2 text-xs font-mono text-slate-300 focus:outline-none focus:border-cyan-500"
            />
          </div>

          <div>
            <label for="ollama-model-input" class="text-xs font-semibold text-slate-400 block mb-1">{i18n.t('settings.ollamaModel')}</label>
            <input
              id="ollama-model-input"
              type="text"
              bind:value={cfg.ollama_model}
              class="w-full bg-slate-950 border border-slate-800 rounded-xl px-3.5 py-2 text-xs font-mono text-slate-300 focus:outline-none focus:border-cyan-500"
            />
          </div>
        {:else if cfg.active_provider === 'gemini'}
          <div class="md:col-span-2">
            <label for="gemini-key-input" class="text-xs font-semibold text-slate-400 block mb-1">{i18n.t('settings.geminiKey')}</label>
            <input
              id="gemini-key-input"
              type="password"
              bind:value={cfg.api_key_gemini}
              placeholder="AIzaSy..."
              class="w-full bg-slate-950 border border-slate-800 rounded-xl px-3.5 py-2 text-xs font-mono text-cyan-400 focus:outline-none focus:border-cyan-500"
            />
          </div>
        {:else if cfg.active_provider === 'openai'}
          <div class="md:col-span-2">
            <label for="openai-key-input" class="text-xs font-semibold text-slate-400 block mb-1">{i18n.t('settings.openaiKey')}</label>
            <input
              id="openai-key-input"
              type="password"
              bind:value={cfg.api_key_openai}
              placeholder="sk-..."
              class="w-full bg-slate-950 border border-slate-800 rounded-xl px-3.5 py-2 text-xs font-mono text-cyan-400 focus:outline-none focus:border-cyan-500"
            />
          </div>
        {:else if cfg.active_provider === 'anthropic'}
          <div class="md:col-span-2">
            <label for="anthropic-key-input" class="text-xs font-semibold text-slate-400 block mb-1">{i18n.t('settings.anthropicKey')}</label>
            <input
              id="anthropic-key-input"
              type="password"
              bind:value={cfg.api_key_anthropic}
              placeholder="sk-ant-..."
              class="w-full bg-slate-950 border border-slate-800 rounded-xl px-3.5 py-2 text-xs font-mono text-cyan-400 focus:outline-none focus:border-cyan-500"
            />
          </div>
        {/if}
      </div>
    </div>
  </div>
</div>
