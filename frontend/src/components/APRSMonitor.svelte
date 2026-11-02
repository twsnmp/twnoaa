<script>
  import { onMount, onDestroy } from 'svelte'
  import { i18n } from '../i18n.svelte.js'
  import { Radio, Search, Trash2, MapPin, MessageSquare, Activity, Terminal, ExternalLink } from '@lucide/svelte'

  let packets = $state([])
  let searchQuery = $state('')
  let showRawTNC = $state(false)
  let selectedPacket = $state(null)
  let cancelEvent = null

  let filteredPackets = $derived.by(() => {
    if (!searchQuery.trim()) return packets
    const q = searchQuery.toLowerCase()
    return packets.filter(p =>
      p.source_call?.toLowerCase().includes(q) ||
      p.dest_call?.toLowerCase().includes(q) ||
      p.path_display?.toLowerCase().includes(q) ||
      p.comment?.toLowerCase().includes(q) ||
      p.message?.toLowerCase().includes(q) ||
      p.raw_tnc?.toLowerCase().includes(q)
    )
  })

  async function loadPackets() {
    try {
      if (window.go?.main?.App?.GetRecentAPRSPackets) {
        const pkts = await window.go.main.App.GetRecentAPRSPackets()
        if (Array.isArray(pkts)) {
          packets = pkts.reverse() // Newest first
        }
      }
    } catch (e) {
      console.error('Failed to load APRS packets:', e)
    }
  }

  async function handleClear() {
    try {
      if (window.go?.main?.App?.ClearRecentAPRSPackets) {
        await window.go.main.App.ClearRecentAPRSPackets()
        packets = []
        selectedPacket = null
      }
    } catch (e) {
      console.error('Failed to clear APRS packets:', e)
    }
  }

  function formatTime(ts) {
    if (!ts) return ''
    const d = new Date(ts)
    return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' })
  }

  onMount(() => {
    loadPackets()

    if (window.runtime?.EventsOn) {
      cancelEvent = window.runtime.EventsOn('aprs_packet', (pkt) => {
        if (pkt) {
          packets = [pkt, ...packets]
        }
      })
    }
  })

  onDestroy(() => {
    if (cancelEvent) cancelEvent()
  })
</script>

<div class="h-full flex flex-col bg-slate-950 p-6 overflow-hidden">
  <!-- Top bar -->
  <div class="flex flex-wrap items-center justify-between gap-4 mb-5 pb-4 border-b border-slate-800">
    <div class="flex items-center gap-3">
      <div class="p-2.5 bg-amber-500/10 border border-amber-500/30 rounded-xl text-amber-400">
        <Radio class="w-6 h-6 animate-pulse" />
      </div>
      <div>
        <h2 class="text-lg font-bold text-slate-100 flex items-center gap-2">
          ISS APRS パケットモニター
          <span class="text-xs font-mono font-normal px-2.5 py-0.5 rounded-full bg-amber-500/20 text-amber-300 border border-amber-500/30">
            437.825 / 145.825 MHz AFSK 1200
          </span>
        </h2>
        <p class="text-xs text-slate-400 mt-0.5">
          ARISS デジピータ経由で中継された世界中のパケット通信をリアルタイムにデコード・監視します
        </p>
      </div>
    </div>

    <!-- Actions -->
    <div class="flex items-center gap-2">
      <div class="relative">
        <Search class="w-4 h-4 text-slate-500 absolute left-3 top-2.5" />
        <input
          type="text"
          bind:value={searchQuery}
          placeholder="コールサイン / 本文を検索..."
          class="pl-9 pr-3 py-1.5 text-xs bg-slate-900 border border-slate-800 rounded-xl text-slate-200 focus:outline-none focus:border-amber-500/60 w-48 sm:w-64"
        />
      </div>

      <button
        onclick={() => showRawTNC = !showRawTNC}
        class="px-3 py-1.5 rounded-xl border text-xs font-semibold flex items-center gap-1.5 transition-all {showRawTNC ? 'bg-amber-600 text-white border-amber-500' : 'bg-slate-900 border-slate-800 text-slate-300 hover:border-slate-700'}"
      >
        <Terminal class="w-3.5 h-3.5" />
        {showRawTNC ? 'パース済み' : '生TNC形式'}
      </button>

      <button
        onclick={handleClear}
        class="px-3 py-1.5 rounded-xl border border-rose-900/60 bg-rose-950/40 text-rose-300 hover:bg-rose-900/60 text-xs font-semibold flex items-center gap-1.5 transition-all"
        title="パケット履歴をクリア"
      >
        <Trash2 class="w-3.5 h-3.5" />
        クリア
      </button>
    </div>
  </div>

  <!-- Content Table & Inspector -->
  <div class="flex-1 min-h-0 flex gap-4 overflow-hidden">
    <!-- Packet List Table -->
    <div class="flex-1 bg-slate-900/80 border border-slate-800 rounded-2xl overflow-hidden flex flex-col shadow-xl">
      <div class="bg-slate-950 px-4 py-2.5 border-b border-slate-800 flex items-center justify-between text-xs text-slate-400 font-semibold uppercase tracking-wider">
        <span>受信パケット ({filteredPackets.length} 件)</span>
        <span class="text-[11px] font-mono text-emerald-400 flex items-center gap-1.5">
          <span class="w-2 h-2 rounded-full bg-emerald-400 animate-ping"></span>
          受信待機中
        </span>
      </div>

      <div class="flex-1 overflow-y-auto">
        {#if filteredPackets.length === 0}
          <div class="h-full flex flex-col items-center justify-center p-8 text-center text-slate-500">
            <Radio class="w-12 h-12 text-slate-700 mb-3 animate-pulse" />
            <p class="text-sm font-semibold text-slate-400">APRS パケットがまだありません</p>
            <p class="text-xs text-slate-500 mt-1 max-w-sm">
              SDR で 437.825 MHz / 145.825 MHz（ISS APRS プリセット）にチューニングして受信を開始するか、ISSの通過録音を実施してください。
            </p>
          </div>
        {:else if showRawTNC}
          <!-- Raw TNC console view -->
          <div class="p-3 font-mono text-xs space-y-1 bg-black/40 text-amber-200/90 select-text">
            {#each filteredPackets as pkt}
              <div class="p-1.5 rounded hover:bg-white/5 cursor-pointer flex items-start gap-2 border-b border-slate-900">
                <span class="text-slate-500 shrink-0 font-normal">[{formatTime(pkt.timestamp)}]</span>
                <span class="font-bold text-amber-400 shrink-0">{pkt.source_call}</span>
                <span class="text-slate-400">&gt;</span>
                <span class="text-cyan-400 shrink-0">{pkt.dest_call}{pkt.path_display ? ',' + pkt.path_display : ''}:</span>
                <span class="text-slate-200 break-all">{pkt.raw_payload}</span>
              </div>
            {/each}
          </div>
        {:else}
          <!-- Structured Table view -->
          <table class="w-full text-left text-xs">
            <thead class="bg-slate-950/60 text-slate-400 font-semibold border-b border-slate-800/80 sticky top-0">
              <tr>
                <th class="py-2.5 px-3">時刻</th>
                <th class="py-2.5 px-3">送信元</th>
                <th class="py-2.5 px-3">中継経路</th>
                <th class="py-2.5 px-3">タイプ</th>
                <th class="py-2.5 px-3">位置</th>
                <th class="py-2.5 px-3">メッセージ / コメント</th>
                <th class="py-2.5 px-3 text-right">RSSI</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-slate-800/50">
              {#each filteredPackets as pkt}
                <tr
                  onclick={() => selectedPacket = pkt}
                  class="hover:bg-slate-800/50 cursor-pointer transition-colors {selectedPacket === pkt ? 'bg-amber-500/10' : ''}"
                >
                  <td class="py-2.5 px-3 font-mono text-slate-400 shrink-0">
                    {formatTime(pkt.timestamp)}
                  </td>
                  <td class="py-2.5 px-3 font-bold font-mono text-amber-400">
                    {pkt.source_call}
                  </td>
                  <td class="py-2.5 px-3 text-slate-400 font-mono text-[11px]">
                    <span class="px-1.5 py-0.5 rounded bg-slate-800 border border-slate-700/60">
                      {pkt.path_display || 'Direct'}
                    </span>
                  </td>
                  <td class="py-2.5 px-3">
                    {#if pkt.payload_type === 'position'}
                      <span class="px-2 py-0.5 rounded-full bg-emerald-500/20 text-emerald-300 border border-emerald-500/30 text-[10px] font-bold">
                        位置情報
                      </span>
                    {:else if pkt.payload_type === 'message'}
                      <span class="px-2 py-0.5 rounded-full bg-cyan-500/20 text-cyan-300 border border-cyan-500/30 text-[10px] font-bold">
                        MSG
                      </span>
                    {:else if pkt.payload_type === 'status'}
                      <span class="px-2 py-0.5 rounded-full bg-purple-500/20 text-purple-300 border border-purple-500/30 text-[10px] font-bold">
                        ステータス
                      </span>
                    {:else}
                      <span class="px-2 py-0.5 rounded-full bg-slate-800 text-slate-300 text-[10px]">
                        {pkt.payload_type}
                      </span>
                    {/if}
                  </td>
                  <td class="py-2.5 px-3 font-mono text-slate-300 text-[11px]">
                    {#if pkt.has_location}
                      <span class="flex items-center gap-1 text-emerald-400">
                        <MapPin class="w-3.5 h-3.5 shrink-0" />
                        {pkt.latitude.toFixed(2)}°, {pkt.longitude.toFixed(2)}°
                      </span>
                    {:else}
                      <span class="text-slate-600">-</span>
                    {/if}
                  </td>
                  <td class="py-2.5 px-3 text-slate-200 max-w-xs truncate">
                    {pkt.message || pkt.comment || pkt.raw_payload}
                  </td>
                  <td class="py-2.5 px-3 text-right font-mono text-slate-400 text-[11px]">
                    {pkt.rssi ? pkt.rssi.toFixed(1) + ' dBFS' : '-'}
                  </td>
                </tr>
              {/each}
            </tbody>
          </table>
        {/if}
      </div>
    </div>

    <!-- Packet Detail Inspector Panel -->
    {#if selectedPacket}
      <div class="w-80 bg-slate-900/90 border border-slate-800 rounded-2xl p-4 flex flex-col gap-3 shadow-xl overflow-y-auto">
        <div class="flex items-center justify-between pb-3 border-b border-slate-800">
          <div class="flex items-center gap-2">
            <div class="p-1.5 rounded-lg bg-amber-500/20 text-amber-400">
              <Activity class="w-4 h-4" />
            </div>
            <h3 class="text-sm font-bold text-slate-100">パケット詳細</h3>
          </div>
          <button
            onclick={() => selectedPacket = null}
            class="text-xs text-slate-500 hover:text-slate-300"
          >
            閉じる
          </button>
        </div>

        <div class="space-y-2.5 text-xs">
          <div>
            <span class="text-[10px] text-slate-400 uppercase font-semibold block">送信元コールサイン</span>
            <span class="text-base font-bold font-mono text-amber-400">{selectedPacket.source_call}</span>
          </div>

          <div>
            <span class="text-[10px] text-slate-400 uppercase font-semibold block">宛先 / 経路 (Path)</span>
            <span class="font-mono text-slate-200 block">{selectedPacket.dest_call}</span>
            <span class="font-mono text-[11px] text-slate-400 block mt-0.5">{selectedPacket.path_display || 'Direct (中継なし)'}</span>
          </div>

          {#if selectedPacket.has_location}
            <div class="p-3 rounded-xl bg-emerald-950/40 border border-emerald-500/30 space-y-1">
              <span class="text-[10px] text-emerald-400 uppercase font-bold flex items-center gap-1">
                <MapPin class="w-3.5 h-3.5" />
                位置座標 (GPS)
              </span>
              <div class="font-mono text-emerald-200 text-sm font-bold">
                {selectedPacket.latitude.toFixed(4)}°N, {selectedPacket.longitude.toFixed(4)}°E
              </div>
              <a
                href="https://www.google.com/maps?q={selectedPacket.latitude},{selectedPacket.longitude}"
                target="_blank"
                class="text-[11px] text-emerald-400 hover:underline flex items-center gap-1 mt-1 font-semibold"
              >
                Google Maps で開く <ExternalLink class="w-3 h-3" />
              </a>
            </div>
          {/if}

          <div>
            <span class="text-[10px] text-slate-400 uppercase font-semibold block">メッセージ / コメント</span>
            <div class="p-2.5 rounded-xl bg-slate-950 border border-slate-800 text-slate-200 font-mono text-xs break-all">
              {selectedPacket.message || selectedPacket.comment || '(本文なし)'}
            </div>
          </div>

          <div>
            <span class="text-[10px] text-slate-400 uppercase font-semibold block">生 TNC フレーム</span>
            <div class="p-2.5 rounded-xl bg-black/60 border border-slate-800 text-amber-300/90 font-mono text-[11px] break-all select-all">
              {selectedPacket.raw_tnc}
            </div>
          </div>
        </div>
      </div>
    {/if}
  </div>
</div>
