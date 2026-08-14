<script>
  import { onMount, onDestroy } from 'svelte'
  import * as echarts from 'echarts'
  import { i18n } from '../i18n.svelte.js'
  import { Activity, Radio, Volume2, TrendingUp } from '@lucide/svelte'

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
          axisLine: { show: false },
          axisLabel: { color: '#94a3b8', fontSize: 10, formatter: '{value}' },
          splitLine: { lineStyle: { color: '#1e293b', type: 'dashed' } }
        },
        {
          type: 'value',
          name: 'SNR (dB)',
          nameTextStyle: { color: '#10b981', fontSize: 10 },
          min: 0,
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
          markPoint: {
            data: [
              { type: 'max', name: 'Max SNR', symbolSize: 35, itemStyle: { color: '#059669' } }
            ],
            label: { fontSize: 9, color: '#ffffff' }
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
  let peakSignal = $derived(
    signalHistory.length > 0 ? Math.max(...signalHistory.map(p => p.signal)) : -100
  )
  let avgNoise = $derived(
    signalHistory.length > 0 ? (signalHistory.reduce((acc, p) => acc + p.noise, 0) / signalHistory.length) : -100
  )
  let maxSNR = $derived(
    signalHistory.length > 0 ? Math.max(...signalHistory.map(p => p.snr)) : 0
  )
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
            <Radio class="w-3 h-3 text-cyan-400" />
            {i18n.t('gallery.signalLevel')} (Peak)
          </span>
          <span class="text-sm font-bold text-cyan-300 font-mono mt-0.5 block">{peakSignal.toFixed(1)} <span class="text-[10px] font-normal text-slate-400">dBFS</span></span>
        </div>
      </div>

      <div class="bg-slate-950/80 border border-slate-800 rounded-xl p-3 flex items-center justify-between">
        <div>
          <span class="text-[10px] text-slate-400 uppercase font-semibold block flex items-center gap-1">
            <Volume2 class="w-3 h-3 text-rose-400" />
            {i18n.t('gallery.noiseLevel')} (Avg)
          </span>
          <span class="text-sm font-bold text-rose-300 font-mono mt-0.5 block">{avgNoise.toFixed(1)} <span class="text-[10px] font-normal text-slate-400">dBFS</span></span>
        </div>
      </div>

      <div class="bg-slate-950/80 border border-slate-800 rounded-xl p-3 flex items-center justify-between">
        <div>
          <span class="text-[10px] text-slate-400 uppercase font-semibold block flex items-center gap-1">
            <TrendingUp class="w-3 h-3 text-emerald-400" />
            {i18n.t('gallery.snr')} (Max)
          </span>
          <span class="text-sm font-bold text-emerald-400 font-mono mt-0.5 block">+{maxSNR.toFixed(1)} <span class="text-[10px] font-normal text-slate-400">dB</span></span>
        </div>
      </div>
    </div>

    <!-- ECharts Container -->
    <div class="bg-slate-950/80 border border-slate-800 rounded-xl p-3 relative">
      <div bind:this={chartContainer} class="w-full h-[280px]"></div>
    </div>
  {/if}
</div>
