<script>
  import { onMount } from 'svelte'
  import * as echarts from 'echarts'
  import 'echarts-gl'
  import { i18n } from '../i18n.svelte.js'
  import { Satellite, Radio, Compass, ArrowUpRight, Activity } from '@lucide/svelte'

  // Local offline images imported for packaging
  import earthImg from '../assets/globe/earth.jpg'
  import bathymetryImg from '../assets/globe/bathymetry.jpg'
  import starfieldImg from '../assets/globe/starfield.jpg'
  import nightImg from '../assets/globe/night.jpg'
  import cloudsImg from '../assets/globe/clouds.png'

  let { onSelectPass = () => {} } = $props()

  let chartContainer = $state(null)
  let chart = null

  let satStates = $state([])
  let observerConfig = $state({ lat: 35.68, lon: 139.76, city: 'Local Station' })
  let selectedSat = $state(null)
  let updateTimer = null

  // Satellite distinct colors
  const satColors = {
    'NOAA 15': '#3b82f6', // Blue
    'NOAA 18': '#10b981', // Emerald
    'NOAA 19': '#f59e0b', // Amber
  }

  async function loadData() {
    try {
      if (window.go && window.go.main && window.go.main.App) {
        const cfg = await window.go.main.App.GetConfig()
        if (cfg) {
          observerConfig = {
            lat: cfg.observer_lat || 35.68,
            lon: cfg.observer_lon || 139.76,
            city: cfg.observer_city || 'Local Station'
          }
        }

        const states = await window.go.main.App.GetSatelliteStates()
        if (states && states.length > 0) {
          satStates = states
          if (!selectedSat) {
            selectedSat = states[0]
          } else {
            // Keep selected satellite state fresh
            const found = states.find(s => s.name === selectedSat.name)
            if (found) selectedSat = found
          }
        }
      }
    } catch (e) {
      console.error("Failed to load satellite state for 3D Globe:", e)
    }
  }

  let lastUpdateTime = 0

  function updateGlobe() {
    if (!chart) return

    const now = Date.now()
    // Throttle 3D globe option re-renders (at most once every 10s)
    // Constantly calling setOption with 3D WebGL scene can cause CPU spikes and view jitter.
    if (lastUpdateTime !== 0 && now - lastUpdateTime < 10000) {
      return
    }
    lastUpdateTime = now

    const baseLon = observerConfig.lon
    const baseLat = observerConfig.lat

    // Preserve user manual rotation/zoom if active, default to no auto-rotation
    let viewControlConfig = {
      autoRotate: false,
      autoRotateSpeed: 0,
      targetCoord: [baseLon, baseLat],
      distance: 180,
      minDistance: 110,
      maxDistance: 320
    }

    const currentOption = chart.getOption()
    if (currentOption && currentOption.globe && currentOption.globe[0]) {
      const currentViewControl = currentOption.globe[0].viewControl
      if (currentViewControl) {
        viewControlConfig = {
          ...viewControlConfig,
          autoRotate: currentViewControl.autoRotate !== undefined ? currentViewControl.autoRotate : false,
          distance: currentViewControl.distance !== undefined ? currentViewControl.distance : viewControlConfig.distance,
          alpha: currentViewControl.alpha !== undefined ? currentViewControl.alpha : currentViewControl.alpha,
          beta: currentViewControl.beta !== undefined ? currentViewControl.beta : currentViewControl.beta,
          targetCoord: currentViewControl.targetCoord || [baseLon, baseLat]
        }
      }
    }

    const scatterData = []
    const lineData = []

    // 1. Observer / Ground Station Beacon Point
    scatterData.push({
      name: `Station: ${observerConfig.city}`,
      value: [baseLon, baseLat, 0],
      symbolSize: 20,
      itemStyle: {
        color: '#06b6d4', // Cyan
        borderColor: '#ffffff',
        borderWidth: 2,
        shadowBlur: 25,
        shadowColor: '#06b6d4'
      }
    })

    // 2. Weather Satellites
    satStates.forEach(sat => {
      const color = satColors[sat.name] || '#a855f7'
      const isActive = sat.is_active // Elevation >= 5°
      const altitudeScale = 350000 // Display altitude on globe (meters approx scale)

      scatterData.push({
        name: sat.name,
        satDetail: sat,
        value: [sat.longitude, sat.latitude, altitudeScale],
        symbolSize: isActive ? 18 : 12,
        itemStyle: {
          color: isActive ? '#ef4444' : color,
          borderColor: isActive ? '#ffffff' : color,
          borderWidth: isActive ? 2 : 1,
          shadowBlur: isActive ? 20 : 8,
          shadowColor: color
        }
      })

      // Link line between Ground Station and Satellite
      lineData.push({
        coords: [
          [sat.longitude, sat.latitude, altitudeScale],
          [baseLon, baseLat, 0]
        ],
        lineStyle: {
          color: isActive ? '#10b981' : color,
          opacity: isActive ? 0.85 : 0.25,
          width: isActive ? 3 : 1,
          type: isActive ? 'solid' : 'dashed'
        }
      })
    })

    const option = {
      backgroundColor: '#020617', // Slate 950
      tooltip: {
        show: true,
        formatter: function (params) {
          if (params.data && params.data.satDetail) {
            const sat = params.data.satDetail
            return `
              <div style="font-family: sans-serif; padding: 4px 8px;">
                <strong style="color: #38bdf8; font-size: 14px;">${sat.name}</strong><br/>
                <span style="color: #94a3b8;">Frequency:</span> ${(sat.frequency / 1e6).toFixed(3)} MHz<br/>
                <span style="color: #94a3b8;">Elevation:</span> <b style="color: ${sat.elevation >= 5 ? '#34d399' : '#f1f5f9'};">${sat.elevation}°</b><br/>
                <span style="color: #94a3b8;">Azimuth:</span> ${sat.azimuth}°<br/>
                <span style="color: #94a3b8;">Doppler:</span> ${sat.doppler_hz} Hz<br/>
                <span style="color: #94a3b8;">Position:</span> ${sat.latitude.toFixed(2)}°, ${sat.longitude.toFixed(2)}°
              </div>
            `
          }
          return params.name
        }
      },
      globe: {
        baseTexture: earthImg,
        heightTexture: bathymetryImg,
        displacementScale: 0.08,
        shading: 'lambert',
        environment: starfieldImg,
        light: {
          ambient: {
            intensity: 0.2
          },
          main: {
            intensity: 1.4,
            shadow: true
          }
        },
        layers: [
          {
            type: 'blend',
            blendTo: 'emission',
            texture: nightImg
          },
          {
            type: 'overlay',
            texture: cloudsImg,
            shading: 'lambert',
            distance: 5
          }
        ],
        viewControl: viewControlConfig
      },
      series: [
        {
          type: 'scatter3D',
          coordinateSystem: 'globe',
          blendMode: 'lighter',
          zlevel: 10,
          label: {
            show: true,
            formatter: '{b}',
            position: 'top',
            textStyle: {
              color: '#f8fafc',
              fontSize: 11,
              fontWeight: 'bold',
              backgroundColor: 'rgba(15, 23, 42, 0.85)',
              padding: [3, 6],
              borderRadius: 4,
              borderColor: 'rgba(56, 189, 248, 0.3)',
              borderWidth: 1
            }
          },
          data: scatterData
        },
        {
          type: 'lines3D',
          coordinateSystem: 'globe',
          blendMode: 'lighter',
          zlevel: 9,
          effect: {
            show: false
          },
          lineStyle: {
            width: 1.5,
            opacity: 0.4
          },
          data: lineData
        }
      ]
    }

    chart.setOption(option)
  }

  onMount(() => {
    chart = echarts.init(chartContainer)

    loadData().then(() => {
      updateGlobe()
    })

    // Click satellite on 3D globe to select
    chart.on('click', (params) => {
      if (params.data && params.data.satDetail) {
        selectedSat = params.data.satDetail
      }
    })

    const handleResize = () => {
      chart && chart.resize()
    }
    window.addEventListener('resize', handleResize)

    // Interval fetch (every 2s)
    updateTimer = setInterval(async () => {
      await loadData()
      updateGlobe()
    }, 2000)

    return () => {
      window.removeEventListener('resize', handleResize)
      if (updateTimer) clearInterval(updateTimer)
      chart && chart.dispose()
    }
  })
</script>

<div class="h-full w-full relative flex bg-slate-950 overflow-hidden">
  <!-- 3D Globe Canvas Container -->
  <div bind:this={chartContainer} class="flex-1 h-full w-full cursor-grab active:cursor-grabbing"></div>

  <!-- Floating HUD Panel -->
  <div class="absolute top-4 left-4 z-10 flex flex-col gap-3 max-w-sm pointer-events-auto">
    <!-- Header Badge -->
    <div class="bg-slate-900/90 backdrop-blur-md border border-slate-800 p-4 rounded-2xl shadow-2xl">
      <div class="flex items-center justify-between gap-3 mb-2">
        <div class="flex items-center gap-2">
          <div class="p-2 bg-cyan-500/10 border border-cyan-500/30 rounded-xl text-cyan-400">
            <Satellite class="w-5 h-5 animate-pulse" />
          </div>
          <div>
            <h2 class="text-sm font-bold text-slate-100">{i18n.t('globe.title')}</h2>
            <p class="text-xs text-slate-400">{i18n.t('globe.subtitle')}</p>
          </div>
        </div>
      </div>

      <!-- Ground Station Info -->
      <div class="mt-3 pt-3 border-t border-slate-800/80 flex items-center justify-between text-xs text-slate-300">
        <span class="text-slate-400 font-medium">{i18n.t('globe.station')}:</span>
        <span class="font-mono text-cyan-400 font-semibold">{observerConfig.city} ({observerConfig.lat.toFixed(2)}°, {observerConfig.lon.toFixed(2)}°)</span>
      </div>
    </div>

    <!-- Live Satellites Quick List -->
    <div class="bg-slate-900/90 backdrop-blur-md border border-slate-800 p-3.5 rounded-2xl shadow-2xl flex flex-col gap-2">
      <h3 class="text-xs font-bold text-slate-400 tracking-wider uppercase px-1">{i18n.t('globe.activeSats')}</h3>

      <div class="flex flex-col gap-1.5">
        {#each satStates as sat}
          <button
            onclick={() => selectedSat = sat}
            class="w-full text-left p-2.5 rounded-xl border transition-all flex items-center justify-between {selectedSat?.name === sat.name ? 'bg-cyan-500/15 border-cyan-500/40 text-cyan-200' : 'bg-slate-950/50 border-slate-800/80 text-slate-300 hover:bg-slate-800/50'}"
          >
            <div class="flex items-center gap-2">
              <span class="w-2.5 h-2.5 rounded-full" style="background-color: {satColors[sat.name] || '#a855f7'}"></span>
              <span class="text-xs font-bold">{sat.name}</span>
            </div>

            <div class="flex items-center gap-3">
              <span class="text-[11px] font-mono text-slate-400">{(sat.frequency / 1e6).toFixed(3)} MHz</span>
              {#if sat.is_active}
                <span class="px-2 py-0.5 rounded-full bg-emerald-500/20 text-emerald-400 border border-emerald-500/30 text-[10px] font-bold animate-pulse">
                  {sat.elevation}° ELEV
                </span>
              {:else}
                <span class="text-[11px] font-mono text-slate-400">{sat.elevation}°</span>
              {/if}
            </div>
          </button>
        {/each}
      </div>
    </div>
  </div>

  <!-- Selected Satellite Detail Card (Bottom Right) -->
  {#if selectedSat}
    <div class="absolute bottom-6 right-6 z-10 w-80 bg-slate-900/95 backdrop-blur-md border border-slate-800 p-4 rounded-2xl shadow-2xl pointer-events-auto">
      <div class="flex items-center justify-between mb-3">
        <div class="flex items-center gap-2">
          <div class="w-3 h-3 rounded-full" style="background-color: {satColors[selectedSat.name] || '#a855f7'}"></div>
          <h3 class="text-base font-bold text-slate-100">{selectedSat.name}</h3>
        </div>
        {#if selectedSat.is_active}
          <span class="px-2.5 py-1 rounded-full bg-emerald-500/20 text-emerald-300 border border-emerald-500/40 text-xs font-bold flex items-center gap-1.5">
            <span class="w-2 h-2 rounded-full bg-emerald-400 animate-ping"></span>
            {i18n.t('tracker.activePass')}
          </span>
        {:else}
          <span class="px-2 py-0.5 rounded-full bg-slate-800 text-slate-400 text-xs font-medium">
            {i18n.t('status.standby')}
          </span>
        {/if}
      </div>

      <div class="grid grid-cols-2 gap-2 text-xs">
        <div class="bg-slate-950/60 p-2.5 rounded-xl border border-slate-800/80">
          <div class="flex items-center gap-1.5 text-slate-400 mb-1">
            <Radio class="w-3.5 h-3.5 text-cyan-400" />
            <span>{i18n.t('tracker.freq')}</span>
          </div>
          <span class="font-mono font-bold text-slate-200">{(selectedSat.frequency / 1e6).toFixed(3)} MHz</span>
        </div>

        <div class="bg-slate-950/60 p-2.5 rounded-xl border border-slate-800/80">
          <div class="flex items-center gap-1.5 text-slate-400 mb-1">
            <ArrowUpRight class="w-3.5 h-3.5 text-emerald-400" />
            <span>{i18n.t('tracker.elevation')}</span>
          </div>
          <span class="font-mono font-bold text-slate-200">{selectedSat.elevation}°</span>
        </div>

        <div class="bg-slate-950/60 p-2.5 rounded-xl border border-slate-800/80">
          <div class="flex items-center gap-1.5 text-slate-400 mb-1">
            <Compass class="w-3.5 h-3.5 text-indigo-400" />
            <span>{i18n.t('tracker.azimuth')}</span>
          </div>
          <span class="font-mono font-bold text-slate-200">{selectedSat.azimuth}°</span>
        </div>

        <div class="bg-slate-950/60 p-2.5 rounded-xl border border-slate-800/80">
          <div class="flex items-center gap-1.5 text-slate-400 mb-1">
            <Activity class="w-3.5 h-3.5 text-amber-400" />
            <span>{i18n.t('tracker.doppler')}</span>
          </div>
          <span class="font-mono font-bold text-slate-200">{selectedSat.doppler_hz > 0 ? '+' : ''}{selectedSat.doppler_hz} Hz</span>
        </div>
      </div>

      <div class="mt-3 pt-3 border-t border-slate-800/80 flex items-center justify-between text-xs font-mono text-slate-400">
        <span>Lat: {selectedSat.latitude.toFixed(2)}°</span>
        <span>Lon: {selectedSat.longitude.toFixed(2)}°</span>
        <span>Alt: {selectedSat.altitude.toFixed(0)} km</span>
      </div>

      <button
        onclick={() => onSelectPass(selectedSat.name, selectedSat.frequency)}
        class="mt-3 w-full py-2 bg-cyan-600 hover:bg-cyan-500 text-white font-semibold rounded-xl text-xs flex items-center justify-center gap-2 shadow-lg shadow-cyan-600/30 transition-all"
      >
        <Radio class="w-4 h-4" />
        {i18n.t('tracker.tuneNow')}
      </button>
    </div>
  {/if}
</div>
