<script>
  import catLogoBase64 from '../../assets/catLogo.js';

  let { mode = 'loading', type = 'info', size = 'md' } = $props();

  const logoUrl = catLogoBase64;

  const sizeClasses = $derived.by(() => {
    switch (size) {
      case 'sm':
        return {
          box: 'h-16 w-16',
          img: 'p-1',
          ring1: 'h-24 w-24',
          ring2: 'h-28 w-28',
          glow: 'h-20 w-20',
        };
      case 'lg':
        return {
          box: 'h-28 w-28',
          img: 'p-2',
          ring1: 'h-40 w-40',
          ring2: 'h-48 w-48',
          glow: 'h-36 w-36',
        };
      case 'md':
      default:
        return {
          box: 'h-20 w-20',
          img: 'p-1.5',
          ring1: 'h-32 w-32',
          ring2: 'h-36 w-36',
          glow: 'h-28 w-28',
        };
    }
  });

  const glowColorClass = $derived.by(() => {
    if (mode === 'loading') {
      return 'bg-gradient-to-tr from-cyan-500 to-blue-500 shadow-cyan-500/40';
    }
    switch (type) {
      case 'danger':
        return 'bg-gradient-to-tr from-rose-500 to-red-600 shadow-rose-500/40';
      case 'warning':
        return 'bg-gradient-to-tr from-amber-500 to-orange-500 shadow-amber-500/40';
      case 'info':
      default:
        return 'bg-gradient-to-tr from-cyan-500 to-teal-500 shadow-cyan-500/40';
    }
  });

  const borderColorClass = $derived.by(() => {
    if (mode === 'loading') {
      return 'border-cyan-400/50 dark:border-cyan-400/60 shadow-cyan-500/30';
    }
    switch (type) {
      case 'danger':
        return 'border-rose-400/50 dark:border-rose-400/60 shadow-rose-500/30';
      case 'warning':
        return 'border-amber-400/50 dark:border-amber-400/60 shadow-amber-500/30';
      case 'info':
      default:
        return 'border-cyan-400/50 dark:border-cyan-400/60 shadow-cyan-500/30';
    }
  });
</script>

<div class="relative flex items-center justify-center select-none py-2">
  {#if mode === 'loading'}
    <!-- Radar Pulse Waves -->
    <div class="absolute {sizeClasses.ring1} rounded-full border border-cyan-400/30 dark:border-cyan-400/40 animate-cat-radar pointer-events-none"></div>
    <div class="absolute {sizeClasses.ring1} rounded-full border border-teal-400/20 dark:border-teal-400/30 animate-cat-radar-delayed pointer-events-none"></div>

    <!-- Orbit Rings with Tech Dots -->
    <div class="absolute {sizeClasses.ring1} rounded-full border border-dashed border-cyan-500/30 dark:border-cyan-400/40 animate-spin-slow pointer-events-none">
      <div class="absolute -top-1 left-1/2 -translate-x-1/2 h-2 w-2 rounded-full bg-cyan-400 shadow-sm shadow-cyan-400"></div>
    </div>
    <div class="absolute {sizeClasses.ring2} rounded-full border border-dotted border-blue-500/25 dark:border-blue-400/30 animate-spin-reverse-slow pointer-events-none">
      <div class="absolute -bottom-1 left-1/2 -translate-x-1/2 h-1.5 w-1.5 rounded-full bg-blue-400 shadow-sm shadow-blue-400"></div>
    </div>
  {/if}

  <!-- Ambient Glow -->
  <div
    class="absolute {sizeClasses.glow} rounded-full {glowColorClass} opacity-40 blur-xl animate-cat-pulse-glow pointer-events-none"
  ></div>

  <!-- Cat Box Container -->
  <div
    class="relative flex {sizeClasses.box} items-center justify-center rounded-2xl overflow-hidden border-2 {borderColorClass} bg-gradient-to-b from-slate-900 via-slate-900 to-slate-950 shadow-xl transition-all duration-300 {mode === 'loading' ? 'animate-cat-bounce' : 'animate-cat-float'}"
  >
    <!-- Scanning Light Bar for loading mode -->
    {#if mode === 'loading'}
      <div class="absolute left-0 right-0 h-1 bg-gradient-to-r from-transparent via-cyan-400 to-transparent animate-scan-beam pointer-events-none z-10"></div>
    {/if}

    <!-- Cat Logo Image -->
    <img
      src={logoUrl}
      alt="Cat Logo"
      class="h-full w-full object-contain {sizeClasses.img} filter drop-shadow-[0_4px_10px_rgba(6,182,212,0.35)] relative z-0"
    />
  </div>
</div>
