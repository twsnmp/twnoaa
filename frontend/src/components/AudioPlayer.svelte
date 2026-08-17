<script>
  import { onDestroy } from 'svelte'
  import { i18n } from '../i18n.svelte.js'
  import { Play, Pause, Square, Volume2, VolumeX, RotateCcw, RotateCw, Music, AlertCircle } from '@lucide/svelte'

  let { audioPath = '', title = '' } = $props()

  let audioElement = $state(null)
  let isPlaying = $state(false)
  let currentTime = $state(0)
  let duration = $state(0)
  let volume = $state(0.8)
  let isMuted = $state(false)
  let loading = $state(false)
  let errorMsg = $state('')
  let audioSrc = $state('')

  let previousPath = ''

  async function loadAudio(path) {
    if (!path) {
      audioSrc = ''
      isPlaying = false
      currentTime = 0
      duration = 0
      errorMsg = ''
      return
    }

    if (path === previousPath && audioSrc) {
      return
    }

    previousPath = path
    loading = true
    errorMsg = ''
    isPlaying = false
    currentTime = 0
    duration = 0

    try {
      if (window.go?.main?.App?.GetAudioBase64) {
        const base64Data = await window.go.main.App.GetAudioBase64(path)
        if (base64Data && base64Data.startsWith('data:audio/')) {
          audioSrc = base64Data
        } else {
          errorMsg = i18n.t('gallery.noAudioAvailable')
          audioSrc = ''
        }
      } else {
        errorMsg = 'Backend API unavailable'
      }
    } catch (err) {
      console.error('Failed to load audio:', err)
      errorMsg = 'Error: ' + (err.message || err)
    } finally {
      loading = false
    }
  }

  $effect(() => {
    if (audioPath) {
      loadAudio(audioPath)
    } else {
      audioSrc = ''
      isPlaying = false
      currentTime = 0
      duration = 0
    }
  })

  function togglePlay() {
    if (!audioElement || !audioSrc) return

    if (isPlaying) {
      audioElement.pause()
      isPlaying = false
    } else {
      audioElement.play().then(() => {
        isPlaying = true
      }).catch(err => {
        console.error('Audio play error:', err)
        errorMsg = 'Playback error: ' + err.message
        isPlaying = false
      })
    }
  }

  function stop() {
    if (!audioElement) return
    audioElement.pause()
    audioElement.currentTime = 0
    currentTime = 0
    isPlaying = false
  }

  function handleSeek(e) {
    if (!audioElement || !duration) return
    const newTime = parseFloat(e.target.value)
    audioElement.currentTime = newTime
    currentTime = newTime
  }

  function skip(sec) {
    if (!audioElement) return
    const newTime = Math.min(Math.max(0, audioElement.currentTime + sec), duration || 0)
    audioElement.currentTime = newTime
    currentTime = newTime
  }

  function handleVolumeChange(e) {
    const val = parseFloat(e.target.value)
    volume = val
    if (audioElement) {
      audioElement.volume = isMuted ? 0 : val
    }
  }

  function toggleMute() {
    isMuted = !isMuted
    if (audioElement) {
      audioElement.volume = isMuted ? 0 : volume
    }
  }

  function handleTimeUpdate() {
    if (audioElement) {
      currentTime = audioElement.currentTime
    }
  }

  function handleLoadedMetadata() {
    if (audioElement) {
      duration = audioElement.duration || 0
      audioElement.volume = isMuted ? 0 : volume
    }
  }

  function handleEnded() {
    isPlaying = false
    currentTime = 0
    if (audioElement) {
      audioElement.currentTime = 0
    }
  }

  function formatTime(sec) {
    if (isNaN(sec) || sec < 0) return '00:00'
    const m = Math.floor(sec / 60)
    const s = Math.floor(sec % 60)
    return `${m.toString().padStart(2, '0')}:${s.toString().padStart(2, '0')}`
  }

  onDestroy(() => {
    if (audioElement) {
      audioElement.pause()
      audioElement.src = ''
    }
  })
</script>

<div class="bg-slate-950/80 border border-slate-800/80 rounded-2xl p-4 shadow-xl space-y-3">
  <!-- Hidden HTML5 Audio Element -->
  {#if audioSrc}
    <audio
      bind:this={audioElement}
      src={audioSrc}
      ontimeupdate={handleTimeUpdate}
      onloadedmetadata={handleLoadedMetadata}
      onended={handleEnded}
      preload="metadata"
    ></audio>
  {/if}

  <!-- Header / Meta Info -->
  <div class="flex items-center justify-between gap-3 pb-2 border-b border-slate-800/60">
    <div class="flex items-center gap-2.5 overflow-hidden">
      <div class="p-2 rounded-xl bg-cyan-950/60 border border-cyan-500/30 text-cyan-400">
        <Music class="w-4 h-4" />
      </div>
      <div class="overflow-hidden">
        <h4 class="text-xs font-bold text-slate-200 truncate">
          {title || i18n.t('gallery.audioTitle')}
        </h4>
        <p class="text-[10px] text-slate-400 font-mono truncate">
          {audioPath ? audioPath.split(/[/\\]/).pop() : i18n.t('gallery.noAudioAvailable')}
        </p>
      </div>
    </div>

    <!-- Animated Waveform indicator when playing -->
    {#if isPlaying}
      <div class="flex items-end gap-0.5 h-4 px-2 py-0.5 bg-cyan-950/40 rounded-lg border border-cyan-500/20">
        <span class="w-1 bg-cyan-400 rounded-full animate-pulse h-3"></span>
        <span class="w-1 bg-cyan-300 rounded-full animate-bounce h-4"></span>
        <span class="w-1 bg-cyan-400 rounded-full animate-pulse h-2"></span>
        <span class="w-1 bg-cyan-300 rounded-full animate-bounce h-3.5"></span>
      </div>
    {/if}
  </div>

  {#if loading}
    <div class="py-6 text-center text-xs text-cyan-400 animate-pulse flex items-center justify-center gap-2">
      <span>{i18n.t('gallery.loadingAudio')}</span>
    </div>
  {:else if errorMsg}
    <div class="py-4 px-3 bg-red-950/30 border border-red-500/30 rounded-xl text-xs text-red-300 flex items-center gap-2">
      <AlertCircle class="w-4 h-4 shrink-0 text-red-400" />
      <span>{errorMsg}</span>
    </div>
  {:else if !audioSrc}
    <div class="py-6 text-center text-xs text-slate-500">
      {i18n.t('gallery.noAudioAvailable')}
    </div>
  {:else}
    <!-- Seek Bar & Time -->
    <div class="space-y-1">
      <input
        type="range"
        min="0"
        max={duration || 100}
        step="0.1"
        value={currentTime}
        oninput={handleSeek}
        class="w-full h-1.5 bg-slate-800 rounded-lg appearance-none cursor-pointer accent-cyan-400 hover:accent-cyan-300 transition-all"
      />
      <div class="flex items-center justify-between text-[11px] font-mono text-slate-400">
        <span>{formatTime(currentTime)}</span>
        <span>{formatTime(duration)}</span>
      </div>
    </div>

    <!-- Controls Row -->
    <div class="flex flex-wrap items-center justify-between gap-3 pt-1">
      <!-- Main Playback Buttons -->
      <div class="flex items-center gap-2">
        <button
          type="button"
          onclick={() => skip(-10)}
          disabled={!audioSrc}
          title={i18n.t('gallery.skipBackward')}
          class="p-2 rounded-xl bg-slate-900 hover:bg-slate-800 text-slate-300 border border-slate-800 hover:border-slate-700 transition-all active:scale-95 disabled:opacity-40 cursor-pointer"
        >
          <RotateCcw class="w-3.5 h-3.5" />
        </button>

        <button
          type="button"
          onclick={togglePlay}
          disabled={!audioSrc}
          title={isPlaying ? i18n.t('gallery.pauseAudio') : i18n.t('gallery.playAudio')}
          class="px-4 py-2 rounded-xl font-semibold text-xs transition-all flex items-center gap-2 shadow-lg active:scale-95 disabled:opacity-40 cursor-pointer {isPlaying ? 'bg-amber-600 hover:bg-amber-500 text-white shadow-amber-600/30' : 'bg-cyan-600 hover:bg-cyan-500 text-white shadow-cyan-600/30'}"
        >
          {#if isPlaying}
            <Pause class="w-4 h-4 fill-current" />
            <span>{i18n.t('gallery.pauseAudio')}</span>
          {:else}
            <Play class="w-4 h-4 fill-current" />
            <span>{i18n.t('gallery.playAudio')}</span>
          {/if}
        </button>

        <button
          type="button"
          onclick={stop}
          disabled={!audioSrc || (currentTime === 0 && !isPlaying)}
          title={i18n.t('gallery.stopAudio')}
          class="p-2 rounded-xl bg-slate-900 hover:bg-red-950/60 text-slate-400 hover:text-red-300 border border-slate-800 hover:border-red-500/30 transition-all active:scale-95 disabled:opacity-40 cursor-pointer"
        >
          <Square class="w-4 h-4 fill-current" />
        </button>

        <button
          type="button"
          onclick={() => skip(10)}
          disabled={!audioSrc}
          title={i18n.t('gallery.skipForward')}
          class="p-2 rounded-xl bg-slate-900 hover:bg-slate-800 text-slate-300 border border-slate-800 hover:border-slate-700 transition-all active:scale-95 disabled:opacity-40 cursor-pointer"
        >
          <RotateCw class="w-3.5 h-3.5" />
        </button>
      </div>

      <!-- Volume Controls -->
      <div class="flex items-center gap-2 bg-slate-900/90 px-3 py-1.5 rounded-xl border border-slate-800">
        <button
          type="button"
          onclick={toggleMute}
          title={isMuted ? i18n.t('gallery.unmute') : i18n.t('gallery.mute')}
          class="text-slate-400 hover:text-cyan-300 transition-colors cursor-pointer"
        >
          {#if isMuted || volume === 0}
            <VolumeX class="w-4 h-4 text-red-400" />
          {:else}
            <Volume2 class="w-4 h-4 text-cyan-400" />
          {/if}
        </button>

        <input
          type="range"
          min="0"
          max="1"
          step="0.05"
          value={isMuted ? 0 : volume}
          oninput={handleVolumeChange}
          title={`${i18n.t('gallery.volume')}: ${Math.round((isMuted ? 0 : volume) * 100)}%`}
          class="w-20 h-1 bg-slate-700 rounded-lg appearance-none cursor-pointer accent-cyan-400 hover:accent-cyan-300 transition-all"
        />

        <span class="text-[10px] font-mono text-slate-400 min-w-[28px] text-right">
          {Math.round((isMuted ? 0 : volume) * 100)}%
        </span>
      </div>
    </div>
  {/if}
</div>
