<script>
  import { onMount, onDestroy } from 'svelte'
  import * as echarts from 'echarts'
  import { i18n } from '../i18n.svelte.js'
  import { Activity, TrendingUp, Clock, Sparkles } from '@lucide/svelte'

  let { signalHistory = [] } = $props()

  let chartContainer = $state(null)
  let chartInstance = null

  function formatTime(sec) {
    const mins = Math.floor(sec / 60)
    const secs = Math.floor(sec % 60)
    return `${mins.toString().padStart(2, '0')}:${secs.toString().padStart(2, '0')}`
  }

  function initChart() {
    if (!chartContainer) return

    if (chartInstance) {
      chartInstance.dispose()
    }

    chartInstance = echarts.init(chartContainer, 'dark', { renderer: 'canvas' })
    updateChartOption()
  }

  function updateChartOption() {
    if (!chartInstance || !signalHistory || signalHistory.length === 0) return

    const timeData = signalHistory.map(p => formatTime(p.time_sec))
    const signalData = signalHistory.map(p => p.signal)
    const noiseData = signalHistory.map(p => p.noise)
    const snrData = signalHistory.map(p => p.snr)

    const maxRecordedSnr = snrData.length > 0 ? Math.max(...snrData) : 0
    const snrAxisMax = maxRecordedSnr > 25 ? Math.ceil(maxRecordedSnr / 5) * 5 : 25

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
          let html = `<div class="font-mono text-xs text-slate-300 font-bold mb-1">${params[0].axisValue}</div>`
          params.forEach(item => {
            let unit = item.seriesName.includes('SNR') ? 'dB' : 'dBFS'
            html += `<div class="flex items-center justify-between gap-4 text-xs">
              <span class="flex items-center gap-1.5">${item.marker} ${item.seriesName}:</span>
              <span class="font-mono font-bold" style="color:${item.color}">${item.value.toFixed(1)} ${unit}</span>
            </div>`
          })
          return html
        }
      },
      legend: {
        data: [i18n.t('gallery.signalLevel'), i18n.t('gallery.noiseLevel'), i18n.t('gallery.snr')],
        textStyle: { color: '#94a3b8', fontSize: 11 },
        top: 0
      },
      grid: {
        top: 40,
        left: 45,
        right: 45,
        bottom: 35,
        containLabel: false
      },
      xAxis: {
        type: 'category',
        data: timeData,
        axisLine: { lineStyle: { color: '#334155' } },
        axisLabel: { color: '#94a3b8', fontSize: 10, interval: 'auto' },
        axisTick: { alignWithLabel: true }
      },
      yAxis: [
        {
          type: 'value',
          name: 'dBFS',
          nameTextStyle: { color: '#64748b', fontSize: 10 },
          min: -50,
          max: 0,
          interval: 10,
          axisLine: { show: false },
          axisLabel: { color: '#94a3b8', fontSize: 10, formatter: '{value}' },
          splitLine: { lineStyle: { color: '#1e293b', type: 'dashed' } }
        },
        {
          type: 'value',
          name: 'SNR (dB)',
          nameTextStyle: { color: '#10b981', fontSize: 10 },
          min: 0,
          max: snrAxisMax,
          interval: 5,
          axisLine: { show: false },
          axisLabel: { color: '#10b981', fontSize: 10, formatter: '{value} dB' },
          splitLine: { show: false }
        }
      ],
      series: [
        {
          name: i18n.t('gallery.signalLevel'),
          type: 'line',
          smooth: true,
          showSymbol: false,
          data: signalData,
          itemStyle: { color: '#38bdf8' },
          lineStyle: { width: 2.5, color: '#38bdf8' },
          areaStyle: {
            color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
              { offset: 0, color: 'rgba(56, 189, 248, 0.35)' },
              { offset: 1, color: 'rgba(56, 189, 248, 0.02)' }
            ])
          }
        },
        {
          name: i18n.t('gallery.noiseLevel'),
          type: 'line',
          smooth: true,
          showSymbol: false,
          data: noiseData,
          itemStyle: { color: '#f43f5e' },
          lineStyle: { width: 1.8, color: '#f43f5e', type: 'dashed' },
          areaStyle: {
            color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
              { offset: 0, color: 'rgba(244, 63, 94, 0.15)' },
              { offset: 1, color: 'rgba(244, 63, 94, 0.01)' }
            ])
          }
        },
        {
          name: i18n.t('gallery.snr'),
          type: 'line',
          yAxisIndex: 1,
          smooth: true,
          showSymbol: false,
          data: snrData,
          itemStyle: { color: '#10b981' },
          lineStyle: { width: 2, color: '#10b981' },
          markLine: {
            silent: true,
            symbol: 'none',
            data: [
              {
                yAxis: 10,
                lineStyle: {
                  color: 'rgba(16, 185, 129, 0.65)',
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
      ]
    }

    chartInstance.setOption(option)
  }

  $effect(() => {
    signalHistory
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

  // Calculate statistics
  let medianSNR = $derived.by(() => {
    if (!signalHistory || signalHistory.length === 0) return 0
    const snrs = signalHistory.map(p => p.snr).sort((a, b) => a - b)
    const mid = Math.floor(snrs.length / 2)
    return snrs.length % 2 !== 0 ? snrs[mid] : (snrs[mid - 1] + snrs[mid]) / 2
  })

  let goodSignalStats = $derived.by(() => {
    if (!signalHistory || signalHistory.length === 0) {
      return { count: 0, ratio: 0, durationSec: 0 }
    }
    const goodCount = signalHistory.filter(p => p.snr >= 10.0).length
    const ratio = (goodCount / signalHistory.length) * 100

    // Compute actual duration in seconds using time_sec
    const firstTime = signalHistory[0]?.time_sec ?? 0
    const lastTime = signalHistory[signalHistory.length - 1]?.time_sec ?? 0
    const totalDuration = Math.max(0, lastTime - firstTime)
    const durationSec = (ratio / 100) * totalDuration

    return {
      count: goodCount,
      ratio: ratio,
      durationSec: Math.round(durationSec)
    }
  })

  let top10SNR = $derived.by(() => {
    if (!signalHistory || signalHistory.length === 0) return 0
    const snrs = signalHistory.map(p => p.snr).sort((a, b) => b - a)
    const topCount = Math.max(1, Math.ceil(snrs.length * 0.1))
    const topSlice = snrs.slice(0, topCount)
    const sum = topSlice.reduce((acc, v) => acc + v, 0)
    return sum / topCount
  })
</script>

<div class="space-y-4">
  {#if !signalHistory || signalHistory.length === 0}
    <div class="bg-slate-950/80 border border-slate-800 rounded-xl p-8 text-center space-y-2">
      <Activity class="w-8 h-8 text-slate-600 mx-auto" />
      <p class="text-xs text-slate-400 font-medium">{i18n.t('gallery.noSignalData')}</p>
    </div>
  {:else}
    <!-- Summary Stats Bar -->
    <div class="grid grid-cols-3 gap-3">
      <div class="bg-slate-950/80 border border-slate-800 rounded-xl p-3 flex items-center justify-between">
        <div>
          <span class="text-[10px] text-slate-400 uppercase font-semibold block flex items-center gap-1">
            <TrendingUp class="w-3 h-3 text-emerald-400" />
            {i18n.t('gallery.medianSNR')}
          </span>
          <span class="text-sm font-bold text-emerald-400 font-mono mt-0.5 block">+{medianSNR.toFixed(1)} <span class="text-[10px] font-normal text-slate-400">dB</span></span>
        </div>
      </div>

      <div class="bg-slate-950/80 border border-slate-800 rounded-xl p-3 flex items-center justify-between">
        <div>
          <span class="text-[10px] text-slate-400 uppercase font-semibold block flex items-center gap-1">
            <Clock class="w-3 h-3 text-cyan-400" />
            {i18n.t('gallery.goodDuration')}
          </span>
          <span class="text-sm font-bold text-cyan-300 font-mono mt-0.5 block">{formatTime(goodSignalStats.durationSec)} <span class="text-[10px] font-normal text-slate-400">({goodSignalStats.ratio.toFixed(0)}%)</span></span>
        </div>
      </div>

      <div class="bg-slate-950/80 border border-slate-800 rounded-xl p-3 flex items-center justify-between">
        <div>
          <span class="text-[10px] text-slate-400 uppercase font-semibold block flex items-center gap-1">
            <Sparkles class="w-3 h-3 text-amber-400" />
            {i18n.t('gallery.top10SNR')}
          </span>
          <span class="text-sm font-bold text-amber-300 font-mono mt-0.5 block">+{top10SNR.toFixed(1)} <span class="text-[10px] font-normal text-slate-400">dB</span></span>
        </div>
      </div>
    </div>

    <!-- ECharts Container -->
    <div class="bg-slate-950/80 border border-slate-800 rounded-xl p-3 relative">
      <div bind:this={chartContainer} class="w-full h-[280px]"></div>
    </div>
  {/if}
</div>
