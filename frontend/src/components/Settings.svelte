<script>
  import { onMount } from 'svelte'
  import { i18n } from '../i18n.svelte.js'
  import { Settings, MapPin, Sliders, Key, Save, RefreshCw, CheckCircle2, Radio, Trash2, X, Info } from '@lucide/svelte'
  import { GetVersion } from '../../wailsjs/go/main/App'

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
    tuner_gain_auto: false,
    bias_tee: false,
    sample_rate: 2080000
  })

  let saveMsg = $state('')
  let isFetchingLoc = $state(false)
  let rawStorageInfo = $state(null)
  let isLoadingStorage = $state(false)
  let showDeleteAllModal = $state(false)
  let isDeletingAllRaw = $state(false)
  let appVersion = $state('v0.1.0')

  async function loadVersion() {
    try {
      if (typeof GetVersion === 'function') {
        const v = await GetVersion()
        if (v) appVersion = v
      } else if (window.go?.main?.App?.GetVersion) {
        const v = await window.go.main.App.GetVersion()
        if (v) appVersion = v
      }
    } catch (err) {
      console.error("Failed to load version:", err)
    }
  }

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

  async function loadRawStorageInfo() {
    if (!window.go?.main?.App?.GetRawDataStorageInfo) return
    isLoadingStorage = true
    try {
      rawStorageInfo = await window.go.main.App.GetRawDataStorageInfo()
    } catch (err) {
      console.error("Failed to get raw storage info:", err)
    } finally {
      isLoadingStorage = false
    }
  }

  async function confirmDeleteAllRaw() {
    if (!window.go?.main?.App?.DeleteAllRawData) return
    isDeletingAllRaw = true
    showDeleteAllModal = false
    try {
      const msg = await window.go.main.App.DeleteAllRawData()
      saveMsg = msg || i18n.t('settings.allRawDataDeleted')
      await loadRawStorageInfo()
      setTimeout(() => saveMsg = '', 3000)
    } catch (err) {
      saveMsg = 'Error: ' + err
    } finally {
      isDeletingAllRaw = false
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
    loadRawStorageInfo()
    loadVersion()
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

      <label for="auto-gain-checkbox" class="flex items-center gap-3 p-3 bg-slate-950 rounded-xl border border-slate-800 cursor-pointer">
        <input
          id="auto-gain-checkbox"
          type="checkbox"
          bind:checked={cfg.tuner_gain_auto}
          class="rounded bg-slate-900 border-slate-700 text-cyan-500 focus:ring-cyan-500"
        />
        <div>
          <span class="text-xs font-bold text-slate-200 block">{i18n.t('settings.autoGainSetting')}</span>
          <span class="text-[10px] text-slate-500 block">{i18n.t('settings.autoGainSettingDesc')}</span>
        </div>
      </label>

      <label for="bias-tee-checkbox" class="flex items-center gap-3 p-3 bg-slate-950 rounded-xl border border-slate-800 cursor-pointer">
        <input
          id="bias-tee-checkbox"
          type="checkbox"
          bind:checked={cfg.bias_tee}
          class="rounded bg-slate-900 border-slate-700 text-cyan-500 focus:ring-cyan-500"
        />
        <div>
          <span class="text-xs font-bold text-slate-200 block">{i18n.t('settings.biasTee')}</span>
          <span class="text-[10px] text-slate-500 block">{i18n.t('settings.biasTeeDesc')}</span>
        </div>
      </label>

      <label for="save-raw-iq-checkbox" class="flex items-center gap-3 p-3 bg-slate-950 rounded-xl border border-slate-800 cursor-pointer">
        <input
          id="save-raw-iq-checkbox"
          type="checkbox"
          bind:checked={cfg.save_raw_iq}
          class="rounded bg-slate-900 border-slate-700 text-cyan-500 focus:ring-cyan-500"
        />
        <div>
          <span class="text-xs font-bold text-slate-200 block">{i18n.t('settings.saveRawIQ')}</span>
          <span class="text-[10px] text-slate-500 block">{i18n.t('settings.saveRawIQDesc')}</span>
        </div>
      </label>

      <!-- Raw IQ Storage Management Box -->
      <div class="p-3.5 bg-slate-950/80 rounded-xl border border-slate-800 space-y-3">
        <div class="flex items-center justify-between">
          <div class="flex items-center gap-2">
            <Radio class="w-4 h-4 text-indigo-400" />
            <div>
              <span class="text-xs font-bold text-slate-200 block">{i18n.t('settings.rawStorage')}</span>
              <span class="text-[10px] text-slate-500 block">{i18n.t('settings.rawStorageDesc')}</span>
            </div>
          </div>
          <button
            type="button"
            onclick={loadRawStorageInfo}
            disabled={isLoadingStorage}
            class="p-1.5 text-slate-400 hover:text-slate-200 hover:bg-slate-800 rounded-lg transition-colors cursor-pointer"
            title={i18n.t('settings.refreshStorage')}
          >
            <RefreshCw class="w-3.5 h-3.5 {isLoadingStorage ? 'animate-spin text-cyan-400' : ''}" />
          </button>
        </div>

        <div class="flex items-center justify-between bg-slate-900/90 px-3 py-2 rounded-lg border border-slate-800/80">
          <div>
            <span class="text-[10px] text-slate-500 uppercase tracking-wider block">{i18n.t('settings.rawStorageTotal')}</span>
            <div class="flex items-baseline gap-2 mt-0.5">
              <span class="text-sm font-bold font-mono text-indigo-300">
                {rawStorageInfo ? rawStorageInfo.total_size_formatted : '0 B'}
              </span>
              {#if rawStorageInfo && rawStorageInfo.file_count > 0}
                <span class="text-[10px] font-mono text-slate-400">
                  ({i18n.t('settings.rawFilesCount', { count: rawStorageInfo.file_count })})
                </span>
              {/if}
            </div>
          </div>

          {#if rawStorageInfo && rawStorageInfo.file_count > 0}
            <button
              type="button"
              onclick={() => showDeleteAllModal = true}
              disabled={isDeletingAllRaw}
              class="px-2.5 py-1.5 bg-rose-950/50 hover:bg-rose-900/70 text-rose-300 border border-rose-500/30 rounded-lg text-xs font-medium flex items-center gap-1.5 transition-all shadow-sm cursor-pointer disabled:opacity-50"
            >
              <Trash2 class="w-3.5 h-3.5" />
              <span>{i18n.t('settings.deleteAllRawData')}</span>
            </button>
          {:else}
            <span class="text-[11px] text-slate-500 italic">
              {i18n.t('settings.noRawData')}
            </span>
          {/if}
        </div>
      </div>

      <div>
        <div class="flex items-center justify-between mb-1">
          <label for="default-gain-input" class="text-xs font-semibold text-slate-400 uppercase tracking-wider">{i18n.t('settings.defaultGain')}</label>
          <span class="text-xs font-mono text-cyan-400 font-bold">
            {cfg.tuner_gain_auto ? i18n.t('sdr.autoGainBadge') : (cfg.tuner_gain / 10).toFixed(1) + ' dB'}
          </span>
        </div>
        <input
          id="default-gain-input"
          type="number"
          bind:value={cfg.tuner_gain}
          disabled={cfg.tuner_gain_auto}
          class="w-full bg-slate-950 border border-slate-800 rounded-xl px-3.5 py-2 text-xs font-mono text-cyan-400 focus:outline-none focus:border-cyan-500 disabled:opacity-50 disabled:cursor-not-allowed"
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

    <!-- Application Info -->
    <div class="bg-slate-900/60 border border-slate-800 rounded-2xl p-5 shadow-sm space-y-3">
      <div class="flex items-center gap-2.5 text-slate-300">
        <Info class="w-4 h-4 text-cyan-400" />
        <h3 class="text-xs font-bold uppercase tracking-wider">{i18n.t('settings.appInfo')}</h3>
      </div>
      <div class="flex items-center justify-between text-xs text-slate-400 pt-2 border-t border-slate-800/60">
        <span>{i18n.t('settings.version')}</span>
        <span class="font-mono text-cyan-300 bg-cyan-950/40 border border-cyan-800/40 px-2.5 py-1 rounded-md">{appVersion}</span>
      </div>
    </div>
  </div>
</div>

<!-- Purge All Raw IQ Confirmation Modal -->
{#if showDeleteAllModal && rawStorageInfo}
  <div class="fixed inset-0 z-50 bg-slate-950/80 backdrop-blur-sm flex items-center justify-center p-4">
    <div class="bg-slate-900 border border-slate-800 rounded-2xl p-6 max-w-md w-full shadow-2xl space-y-4">
      <div class="flex items-center justify-between">
        <div class="flex items-center gap-3 text-rose-400">
          <div class="p-2.5 bg-rose-500/10 border border-rose-500/20 rounded-xl">
            <Trash2 class="w-5 h-5 text-rose-400" />
          </div>
          <div>
            <h3 class="text-sm font-bold text-slate-100">{i18n.t('settings.deleteAllRawData')}</h3>
            <p class="text-xs text-slate-400 font-mono mt-0.5">{rawStorageInfo.total_size_formatted} ({rawStorageInfo.file_count} files)</p>
          </div>
        </div>
        <button
          onclick={() => showDeleteAllModal = false}
          class="p-1 text-slate-400 hover:text-slate-200 rounded-lg hover:bg-slate-800 transition-all cursor-pointer"
        >
          <X class="w-4 h-4" />
        </button>
      </div>

      <p class="text-xs text-slate-300 leading-relaxed whitespace-pre-line">
        {i18n.t('settings.deleteAllRawDataConfirm', { size: rawStorageInfo.total_size_formatted })}
      </p>

      <div class="flex items-center justify-end gap-2 pt-2">
        <button
          type="button"
          onclick={() => showDeleteAllModal = false}
          class="px-4 py-2 bg-slate-800 hover:bg-slate-700 text-slate-300 rounded-xl text-xs font-semibold transition-all cursor-pointer"
        >
          {i18n.t('gallery.cancel')}
        </button>
        <button
          type="button"
          onclick={confirmDeleteAllRaw}
          disabled={isDeletingAllRaw}
          class="px-4 py-2 bg-rose-600 hover:bg-rose-500 text-white rounded-xl text-xs font-semibold shadow-lg shadow-rose-600/30 transition-all flex items-center gap-1.5 cursor-pointer disabled:opacity-50"
        >
          <Trash2 class="w-4 h-4" />
          {i18n.t('settings.deleteAllRawData')}
        </button>
      </div>
    </div>
  </div>
{/if}
