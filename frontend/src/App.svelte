<script>
  import Header from './components/Header.svelte'
  import PassTracker from './components/PassTracker.svelte'
  import SDRControl from './components/SDRControl.svelte'
  import ImageGallery from './components/ImageGallery.svelte'
  import AIAnalysis from './components/AIAnalysis.svelte'
  import Settings from './components/Settings.svelte'

  let activeTab = $state('tracker')
  let selectedFreq = $state(137100000)
  let selectedSat = $state('NOAA 19')
  let selectedRecordForAI = $state(null)

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

  <main class="flex-1 overflow-hidden relative">
    {#if activeTab === 'tracker'}
      <PassTracker onSelectPass={handleSelectPass} />
    {:else if activeTab === 'sdr'}
      <SDRControl initialFreq={selectedFreq} initialSat={selectedSat} onDecoded={handleDecoded} />
    {:else if activeTab === 'gallery'}
      <ImageGallery onRunAI={handleRunAI} />
    {:else if activeTab === 'ai'}
      <AIAnalysis record={selectedRecordForAI} />
    {:else if activeTab === 'settings'}
      <Settings />
    {/if}
  </main>
</div>
