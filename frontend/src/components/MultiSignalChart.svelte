<script>
  import { onMount, onDestroy } from 'svelte'
  import * as echarts from 'echarts'
  import { i18n } from '../i18n.svelte.js'
  import { Activity, TrendingUp, Clock, Sparkles, Radio, Layers, X, Info } from '@lucide/svelte'

  let { selectedRecords = [], onDeselect = () => {} } = $props()

  let chartContainer = $state(null)
  let chartInstance = null
  let activeMetric = $state('snr') // 'snr' | 'signal' | 'noise'

  const COLOR_PALETTE = [
    '#38bdf8', // Sky Blue
    '#10b981', // Emerald Green
    '#f59e0b', // Amber Orange
    '#a855f7', // Purple
    '#f43f5e', // Rose Red
    '#3b82f6', // Indigo Blue
    '#06b6d4', // Cyan
    '#ec4899', // Pink
    '#eab308', // Yellow
    '#84cc16', // Lime
    '#6366f1', // Indigo
    '#14b8a6', // Teal
  ]

  function formatTime(sec) {
    if (isNaN(sec) || sec < 0) return '00:00'
    const mins = Math.floor(sec / 60)
    const secs = Math.floor(sec % 60)
    return `${mins.toString().padStart(2, '0')}:${secs.toString().padStart(2, '0')}`
  }

  // Filter records that actually have signal_history data
  let recordsWithData = $derived(
    selectedRecords.filter(r => r && r.signal_history && r.signal_history.length > 0)
  )

  // Compute statistics for each pass
  let passStats = $derived.by(() => {
    return recordsWithData.map((rec, idx) => {
      const history = rec.signal_history || []
      const snrs = history.map(p => p.snr).sort((a, b) => a - b)
      const mid = Math.floor(snrs.length / 2)
      const medianSNR = snrs.length === 0 ? 0 : (snrs.length % 2 !== 0 ? snrs[mid] : (snrs[mid - 1] + snrs[mid]) / 2)

      const goodCount = history.filter(p => p.snr >= 10.0).length
      const ratio = history.length > 0 ? (goodCount / history.length) * 100 : 0

      const firstTime = history[0]?.time_sec ?? 0
      const lastTime = history[history.length - 1]?.time_sec ?? 0
      const totalDuration = Math.max(0, lastTime - firstTime)
      const durationSec = Math.round((ratio / 100) * totalDuration)

      const snrsDesc = [...snrs].reverse()
      const topCount = Math.max(1, Math.ceil(snrsDesc.length * 0.1))
      const top10SNR = snrsDesc.length === 0 ? 0 : (snrsDesc.slice(0, topCount).reduce((a, b) => a + b, 0) / topCount)

      return {
        rec,
        color: COLOR_PALETTE[idx % COLOR_PALETTE.length],
        medianSNR,
        goodRatio: ratio,
        goodDurationSec: durationSec,
        top10SNR,
        totalDuration
      }
    })
  })

  function initChart() {
    if (!chartContainer) return

    if (chartInstance) {
      chartInstance.dispose()
    }

    chartInstance = echarts.init(chartContainer, 'dark', { renderer: 'canvas' })
    updateChartOption()
  }

  function updateChartOption() {
    if (!chartInstance) return

    if (recordsWithData.length === 0) {
      chartInstance.clear()
      return
    }

    // Determine global max duration in seconds
    let maxDuration = 0
    let maxSnrGlobal = 0
    let minDbFSGlobal = 0

    recordsWithData.forEach(r => {
      const hist = r.signal_history || []
      if (hist.length > 0) {
        const lastSec = hist[hist.length - 1].time_sec
        if (lastSec > maxDuration) maxDuration = lastSec
        hist.forEach(p => {
          if (p.snr > maxSnrGlobal) maxSnrGlobal = p.snr
          if (p.noise < minDbFSGlobal) minDbFSGlobal = p.noise
          if (p.signal < minDbFSGlobal) minDbFSGlobal = p.signal
        })
      }
    })

    if (maxDuration <= 0) maxDuration = 600

    const isSNR = activeMetric === 'snr'
    const isSignal = activeMetric === 'signal'
    const isNoise = activeMetric === 'noise'

    const snrAxisMax = maxSnrGlobal > 25 ? Math.ceil(maxSnrGlobal / 5) * 5 : 25

    const series = recordsWithData.map((rec, idx) => {
      const color = COLOR_PALETTE[idx % COLOR_PALETTE.length]
      const labelDate = new Date(rec.timestamp).toLocaleDateString([], { month: 'numeric', day: 'numeric' })
      const labelTime = new Date(rec.timestamp).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
      const seriesName = `${rec.sat_name} (${labelDate} ${labelTime}, ${rec.max_elevation}°)`

      const data = rec.signal_history.map(p => {
        let val = p.snr
        if (isSignal) val = p.signal
        else if (isNoise) val = p.noise
        return [p.time_sec, val]
      })

      return {
        name: seriesName,
        type: 'line',
        smooth: true,
        showSymbol: false,
        data: data,
        itemStyle: { color: color },
        lineStyle: { width: 2, color: color },
        emphasis: {
          lineStyle: { width: 3.5 }
        }
      }
    })

    // Add markLine to the first series if viewing SNR
    if (isSNR && series.length > 0) {
      series[0].markLine = {
        silent: true,
        symbol: 'none',
        data: [
          {
            yAxis: 10,
            lineStyle: {
              color: 'rgba(16, 185, 129, 0.75)',
              type: 'dashed',
              width: 1.5
            },
            label: {
              show: true,
              position: 'insideEndTop',
              formatter: '10 dB (Lock)',
              color: '#10b981',
              fontSize: 10
            }
          }
        ]
      }
    }

    const yAxisConfig = isSNR
      ? {
          type: 'value',
          name: 'SNR (dB)',
          nameTextStyle: { color: '#10b981', fontSize: 11, fontWeight: 'bold' },
          min: 0,
          max: snrAxisMax,
          interval: 5,
          axisLine: { show: false },
          axisLabel: { color: '#10b981', fontSize: 10, formatter: '{value} dB' },
          splitLine: { lineStyle: { color: '#1e293b', type: 'dashed' } }
        }
      : {
          type: 'value',
          name: isSignal ? `${i18n.t('gallery.signalLevel')} (dBFS)` : `${i18n.t('gallery.noiseLevel')} (dBFS)`,
          nameTextStyle: { color: isSignal ? '#38bdf8' : '#f43f5e', fontSize: 11, fontWeight: 'bold' },
          min: -50,
          max: 0,
          interval: 10,
          axisLine: { show: false },
          axisLabel: { color: '#94a3b8', fontSize: 10, formatter: '{value} dBFS' },
          splitLine: { lineStyle: { color: '#1e293b', type: 'dashed' } }
        }

    const option = {
      backgroundColor: 'transparent',
      tooltip: {
        trigger: 'axis',
        axisPointer: {
          type: 'cross',
          crossStyle: { color: '#94a3b8' }
        },
        backgroundColor: 'rgba(15, 23, 42, 0.95)',
        borderColor: '#334155',
        borderWidth: 1,
        textStyle: { color: '#f8fafc', fontSize: 12 },
        formatter: function (params) {
          if (!params || params.length === 0) return ''
          const timeSec = params[0].value[0]
          let html = `<div class="font-mono text-xs text-slate-300 font-bold mb-1 pb-1 border-b border-slate-700 flex items-center justify-between">
            <span>${i18n.t('gallery.timeOffset')}: ${formatTime(timeSec)}</span>
            <span class="text-[10px] text-slate-400 font-normal">(${timeSec.toFixed(1)}s)</span>
          </div>`
          params.forEach(item => {
            const unit = isSNR ? 'dB' : 'dBFS'
            const val = item.value[1]
            html += `<div class="flex items-center justify-between gap-4 text-xs py-0.5">
              <span class="flex items-center gap-1.5 truncate max-w-[240px] text-slate-200">${item.marker} ${item.seriesName}:</span>
              <span class="font-mono font-bold whitespace-nowrap" style="color:${item.color}">${val.toFixed(1)} ${unit}</span>
            </div>`
          })
          return html
        }
      },
      legend: {
        type: 'scroll',
        orient: 'horizontal',
        top: 0,
        textStyle: { color: '#94a3b8', fontSize: 11 },
        pageTextStyle: { color: '#94a3b8' },
        pageIconColor: '#38bdf8',
        pageIconInactiveColor: '#475569'
      },
      grid: {
        top: 45,
        left: 55,
        right: 35,
        bottom: 50,
        containLabel: false
      },
      dataZoom: [
        {
          type: 'slider',
          show: true,
          xAxisIndex: 0,
          start: 0,
          end: 100,
          height: 18,
          bottom: 8,
          borderColor: '#334155',
          backgroundColor: 'rgba(15, 23, 42, 0.6)',
          fillerColor: 'rgba(56, 189, 248, 0.2)',
          handleStyle: { color: '#38bdf8' },
          textStyle: { color: '#94a3b8', fontSize: 9 },
          labelFormatter: (v) => formatTime(v)
        },
        {
          type: 'inside',
          xAxisIndex: 0
        }
      ],
      xAxis: {
        type: 'value',
        min: 0,
        max: Math.ceil(maxDuration),
        axisLine: { lineStyle: { color: '#334155' } },
        axisLabel: {
          color: '#94a3b8',
          fontSize: 10,
          formatter: (value) => formatTime(value)
        },
        splitLine: { lineStyle: { color: '#1e293b', type: 'dotted' } }
      },
      yAxis: yAxisConfig,
      series: series
    }

    chartInstance.setOption(option, true)
  }

  $effect(() => {
    selectedRecords
    activeMetric
    if (chartInstance) {
      updateChartOption()
    }
  })

  function handleResize() {
    if (chartInstance) {
      chartInstance.resize()
    }
  }

  onMount(() => {
    initChart()
    window.addEventListener('resize', handleResize)
  })

  onDestroy(() => {
    window.removeEventListener('resize', handleResize)
    if (chartInstance) {
      chartInstance.dispose()
    }
  })
</script>

<div class="space-y-4">
  {#if recordsWithData.length === 0}
    <div class="bg-slate-950/80 border border-slate-800 rounded-xl p-8 text-center space-y-2">
      <Activity class="w-8 h-8 text-slate-600 mx-auto" />
      <p class="text-xs text-slate-400 font-medium">{i18n.t('gallery.noPassSelected')}</p>
    </div>
  {:else}
    <!-- Controls Header -->
    <div class="flex flex-wrap items-center justify-between gap-3 bg-slate-950/80 border border-slate-800 rounded-xl p-3">
      <div class="flex items-center gap-2">
        <Layers class="w-4 h-4 text-cyan-400" />
        <span class="text-xs font-bold text-slate-200">
          {i18n.t('gallery.compareSignals')}
          <span class="text-[11px] font-mono text-cyan-400 font-normal ml-1">({recordsWithData.length} passes)</span>
        </span>
      </div>

      <!-- Metric Toggle -->
      <div class="flex items-center gap-1 bg-slate-900 p-1 rounded-lg border border-slate-800 text-xs">
        <button
          type="button"
          onclick={() => activeMetric = 'snr'}
          class="px-2.5 py-1 rounded-md font-medium transition-colors cursor-pointer {activeMetric === 'snr' ? 'bg-emerald-600 text-white shadow-sm' : 'text-slate-400 hover:text-slate-200'}"
        >
          {i18n.t('gallery.metricSNR')}
        </button>
        <button
          type="button"
          onclick={() => activeMetric = 'signal'}
          class="px-2.5 py-1 rounded-md font-medium transition-colors cursor-pointer {activeMetric === 'signal' ? 'bg-cyan-600 text-white shadow-sm' : 'text-slate-400 hover:text-slate-200'}"
        >
          {i18n.t('gallery.metricSignal')}
        </button>
        <button
          type="button"
          onclick={() => activeMetric = 'noise'}
          class="px-2.5 py-1 rounded-md font-medium transition-colors cursor-pointer {activeMetric === 'noise' ? 'bg-rose-600 text-white shadow-sm' : 'text-slate-400 hover:text-slate-200'}"
        >
          {i18n.t('gallery.metricNoise')}
        </button>
      </div>
    </div>

    <!-- Multi-Series ECharts Container -->
    <div class="bg-slate-950/80 border border-slate-800 rounded-xl p-3 relative shadow-inner">
      <div bind:this={chartContainer} class="w-full h-[320px]"></div>
    </div>

    <!-- Comparison Summary Table -->
    <div class="bg-slate-950/80 border border-slate-800 rounded-xl p-3.5 space-y-2.5">
      <div class="flex items-center justify-between">
        <h4 class="text-xs font-bold text-slate-300 uppercase tracking-wider flex items-center gap-1.5">
          <TrendingUp class="w-3.5 h-3.5 text-cyan-400" />
          {i18n.t('gallery.comparisonSummary')}
        </h4>
        <span class="text-[10px] text-slate-500">{i18n.t('gallery.zoomHint')}</span>
      </div>

      <div class="overflow-x-auto">
        <table class="w-full text-left text-xs border-collapse font-sans">
          <thead>
            <tr class="border-b border-slate-800 text-[10px] text-slate-400 uppercase">
              <th class="py-2 px-2 font-semibold">Pass</th>
              <th class="py-2 px-2 font-semibold">Frequency</th>
              <th class="py-2 px-2 font-semibold">Max Elev</th>
              <th class="py-2 px-2 font-semibold text-emerald-400">{i18n.t('gallery.medianSNR')}</th>
              <th class="py-2 px-2 font-semibold text-cyan-400">{i18n.t('gallery.goodDuration')}</th>
              <th class="py-2 px-2 font-semibold text-amber-400">{i18n.t('gallery.top10SNR')}</th>
              <th class="py-2 px-2 text-right"></th>
            </tr>
          </thead>
          <tbody class="divide-y divide-slate-800/60 font-mono text-[11px]">
            {#each passStats as stat}
              <tr class="hover:bg-slate-900/60 transition-colors">
                <td class="py-2 px-2 font-sans font-medium text-slate-200 flex items-center gap-2">
                  <span class="w-2.5 h-2.5 rounded-full inline-block shrink-0 shadow-sm" style="background-color: {stat.color}"></span>
                  <div>
                    <span class="font-bold block">{stat.rec.sat_name}</span>
                    <span class="text-[10px] text-slate-500 block font-mono">{new Date(stat.rec.timestamp).toLocaleString()}</span>
                  </div>
                </td>
                <td class="py-2 px-2 text-cyan-400">
                  {(stat.rec.frequency / 1e6).toFixed(4)} MHz
                </td>
                <td class="py-2 px-2 text-slate-300">
                  {stat.rec.max_elevation}°{stat.rec.max_elevation_az ? ` (${stat.rec.max_elevation_az}°)` : ''}
                </td>
                <td class="py-2 px-2 font-bold text-emerald-400">
                  +{stat.medianSNR.toFixed(1)} dB
                </td>
                <td class="py-2 px-2 text-cyan-300">
                  {formatTime(stat.goodDurationSec)} <span class="text-[10px] text-slate-400 font-normal">({stat.goodRatio.toFixed(0)}%)</span>
                </td>
                <td class="py-2 px-2 font-bold text-amber-300">
                  +{stat.top10SNR.toFixed(1)} dB
                </td>
                <td class="py-2 px-2 text-right">
                  <button
                    type="button"
                    onclick={() => onDeselect(stat.rec.id)}
                    class="p-1 text-slate-500 hover:text-red-400 hover:bg-red-500/10 rounded-md transition-all cursor-pointer"
                    title="Remove from comparison"
                  >
                    <X class="w-3.5 h-3.5" />
                  </button>
                </td>
              </tr>
            {/each}
          </tbody>
        </table>
      </div>
    </div>
  {/if}
</div>
