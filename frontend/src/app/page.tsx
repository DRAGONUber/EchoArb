// src/app/page.tsx
'use client';

import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { useWebSocket } from '@/hooks/useWebSocket';
import { api, getWebSocketURL, Tick } from '@/lib/api';

type SourceFilter = 'ALL' | 'KALSHI' | 'POLYMARKET';

function formatTime(timestamp: string): string {
  const date = new Date(timestamp);
  if (Number.isNaN(date.getTime())) return timestamp;
  return date.toLocaleTimeString('en-US', { hour12: false });
}

function formatPrice(price: number | null | undefined): string {
  if (price === null || price === undefined || price === 0) return '—';
  return `${(price * 100).toFixed(0)}¢`;
}

function formatSpread(bid: number | null | undefined, ask: number | null | undefined): string {
  if (!bid || !ask) return '—';
  return `${((ask - bid) * 100).toFixed(1)}¢`;
}

function formatNumber(value: number | null | undefined): string {
  if (value === null || value === undefined || value === 0) return '—';
  if (value >= 1000000) return `${(value / 1000000).toFixed(1)}M`;
  if (value >= 1000) return `${(value / 1000).toFixed(1)}K`;
  return value.toLocaleString();
}

function formatDollars(value: number | null | undefined): string {
  if (value === null || value === undefined || value === 0) return '—';
  if (value >= 1000000) return `$${(value / 1000000).toFixed(1)}M`;
  if (value >= 1000) return `$${(value / 1000).toFixed(1)}K`;
  return `$${value.toLocaleString()}`;
}

function formatSize(value: number | null | undefined): string {
  if (value === null || value === undefined || value === 0) return '—';
  if (value >= 1000) return `${(value / 1000).toFixed(1)}K`;
  return value.toFixed(1);
}

function truncate(text: string, len: number): string {
  return text.length > len ? text.slice(0, len) + '…' : text;
}

// Stats Card Component
function StatCard({ label, value, subValue, color, icon }: {
  label: string;
  value: string | number;
  subValue?: string;
  color: 'blue' | 'green' | 'purple' | 'orange';
  icon: React.ReactNode;
}) {
  const colors = {
    blue: 'from-blue-500/20 to-blue-500/5 border-blue-500/30',
    green: 'from-green-500/20 to-green-500/5 border-green-500/30',
    purple: 'from-purple-500/20 to-purple-500/5 border-purple-500/30',
    orange: 'from-orange-500/20 to-orange-500/5 border-orange-500/30',
  };

  return (
    <div className={`bg-gradient-to-br ${colors[color]} border rounded-xl p-4`}>
      <div className="flex items-center gap-3">
        <div className="text-2xl opacity-60">{icon}</div>
        <div>
          <div className="text-xs uppercase tracking-wide text-zinc-400">{label}</div>
          <div className="text-2xl font-bold text-white">{value}</div>
          {subValue && <div className="text-xs text-zinc-500">{subValue}</div>}
        </div>
      </div>
    </div>
  );
}

export default function Home() {
  const [ticks, setTicks] = useState<Tick[]>([]);
  const [ticksPerSecond, setTicksPerSecond] = useState(0);
  const [filter, setFilter] = useState<SourceFilter>('ALL');
  const [totalTicks, setTotalTicks] = useState({ kalshi: 0, poly: 0 });
  const tickTimesRef = useRef<number[]>([]);

  // WebSocket handler
  const handleWebSocketMessage = useCallback((message: any) => {
    if (message.type === 'tick') {
      const newTick: Tick = {
        source: message.source,
        contract_id: message.contract_id,
        price: message.price,
        timestamp: message.timestamp,
        latency_ms: message.latency_ms,
        yes_bid: message.yes_bid,
        yes_ask: message.yes_ask,
        volume: message.volume,
        open_interest: message.open_interest,
        trade_size: message.trade_size,
        market_name: message.market_name,
      };
      setTicks((prev) => [newTick, ...prev].slice(0, 200));
      setTotalTicks((prev) => ({
        kalshi: prev.kalshi + (message.source === 'KALSHI' ? 1 : 0),
        poly: prev.poly + (message.source === 'POLYMARKET' ? 1 : 0),
      }));

      const now = Date.now();
      tickTimesRef.current = [...tickTimesRef.current, now].filter((ts) => now - ts <= 1000);
      setTicksPerSecond(tickTimesRef.current.length);
    }
  }, []);

  const { connected } = useWebSocket({
    url: getWebSocketURL('/ws/spreads'),
    onMessage: handleWebSocketMessage,
    reconnectInterval: 3000,
    maxReconnectAttempts: 10,
  });

  // Fetch initial ticks
  useEffect(() => {
    async function fetchInitialData() {
      try {
        const ticksData = await api.getTicks();
        setTicks(ticksData);
        const kalshiCount = ticksData.filter((t) => t.source === 'KALSHI').length;
        const polyCount = ticksData.filter((t) => t.source === 'POLYMARKET').length;
        setTotalTicks({ kalshi: kalshiCount, poly: polyCount });
      } catch (error) {
        console.error('Failed to fetch initial data:', error);
      }
    }
    fetchInitialData();
    const interval = setInterval(fetchInitialData, 5000); // Refresh every 5 seconds
    return () => clearInterval(interval);
  }, []);

  // Update ticks per second
  useEffect(() => {
    const interval = setInterval(() => {
      const now = Date.now();
      tickTimesRef.current = tickTimesRef.current.filter((ts) => now - ts <= 1000);
      setTicksPerSecond(tickTimesRef.current.length);
    }, 500);
    return () => clearInterval(interval);
  }, []);

  // Filter ticks
  const filteredTicks = useMemo(() => {
    if (filter === 'ALL') return ticks;
    return ticks.filter((t) => t.source === filter);
  }, [ticks, filter]);

  // Calculate stats
  const stats = useMemo(() => {
    const avgLatency = ticks.length > 0
      ? ticks.reduce((sum, t) => sum + (t.latency_ms || 0), 0) / ticks.length
      : 0;
    const withBidAsk = ticks.filter((t) => t.yes_bid && t.yes_ask);
    const avgSpread = withBidAsk.length > 0
      ? withBidAsk.reduce((sum, t) => sum + ((t.yes_ask! - t.yes_bid!) * 100), 0) / withBidAsk.length
      : 0;
    return { avgLatency, avgSpread };
  }, [ticks]);

  return (
    <div className="min-h-screen bg-[#0a0a0f]">
      {/* Header */}
      <header className="border-b border-zinc-800 bg-[#0a0a0f]/80 backdrop-blur-sm sticky top-0 z-50">
        <div className="max-w-[1600px] mx-auto px-6 py-4">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-4">
              <h1 className="text-2xl font-bold text-white">
                Echo<span className="text-blue-500">Arb</span>
              </h1>
              <div className="hidden sm:block text-sm text-zinc-500">
                Live prediction market data
              </div>
            </div>
            <div className="flex items-center gap-6">
              <div className="flex items-center gap-2 text-sm">
                <div className={`w-2 h-2 rounded-full ${connected ? 'bg-green-500 pulse-glow' : 'bg-red-500'}`} />
                <span className={connected ? 'text-green-400' : 'text-red-400'}>
                  {connected ? 'Live' : 'Disconnected'}
                </span>
              </div>
              <div className="text-sm font-mono text-zinc-400">
                <span className="text-white font-bold">{ticksPerSecond}</span> ticks/s
              </div>
            </div>
          </div>
        </div>
      </header>

      <main className="max-w-[1600px] mx-auto px-6 py-6">
        {/* Stats Grid */}
        <div className="grid grid-cols-2 lg:grid-cols-4 gap-4 mb-6">
          <StatCard
            label="Kalshi Ticks"
            value={totalTicks.kalshi}
            color="blue"
            icon="📊"
          />
          <StatCard
            label="Polymarket Ticks"
            value={totalTicks.poly}
            color="green"
            icon="📈"
          />
          <StatCard
            label="Avg Latency"
            value={`${stats.avgLatency.toFixed(0)}ms`}
            subValue={stats.avgLatency < 100 ? 'Excellent' : stats.avgLatency < 500 ? 'Good' : 'Slow'}
            color="purple"
            icon="⚡"
          />
          <StatCard
            label="Avg Spread"
            value={`${stats.avgSpread.toFixed(1)}¢`}
            color="orange"
            icon="📉"
          />
        </div>

        {/* Filter Tabs */}
        <div className="flex items-center gap-2 mb-4">
          {(['ALL', 'KALSHI', 'POLYMARKET'] as SourceFilter[]).map((f) => (
            <button
              key={f}
              onClick={() => setFilter(f)}
              className={`px-4 py-2 text-sm font-medium rounded-lg transition-all ${
                filter === f
                  ? 'bg-blue-600 text-white'
                  : 'bg-zinc-800/50 text-zinc-400 hover:bg-zinc-800 hover:text-white'
              }`}
            >
              {f === 'ALL' ? 'All Sources' : f}
              {f !== 'ALL' && (
                <span className="ml-2 text-xs opacity-60">
                  {f === 'KALSHI' ? totalTicks.kalshi : totalTicks.poly}
                </span>
              )}
            </button>
          ))}
          <div className="flex-1" />
          <div className="text-xs text-zinc-500">
            Showing {filteredTicks.length} ticks
          </div>
        </div>

        {/* Tick Table */}
        <div className="bg-zinc-900/50 border border-zinc-800 rounded-xl overflow-hidden">
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-zinc-800 text-zinc-400 text-xs uppercase tracking-wider">
                  <th className="text-left py-3 px-3 font-medium sticky left-0 bg-zinc-900/90">Source</th>
                  <th className="text-left py-3 px-3 font-medium">Market</th>
                  <th className="text-right py-3 px-3 font-medium">Price</th>
                  <th className="text-right py-3 px-3 font-medium">Yes Bid</th>
                  <th className="text-right py-3 px-3 font-medium">Yes Ask</th>
                  <th className="text-right py-3 px-3 font-medium">Spread</th>
                  <th className="text-right py-3 px-3 font-medium">No Bid</th>
                  <th className="text-right py-3 px-3 font-medium">No Ask</th>
                  <th className="text-right py-3 px-3 font-medium">Bid Size</th>
                  <th className="text-right py-3 px-3 font-medium">Ask Size</th>
                  <th className="text-right py-3 px-3 font-medium">Volume</th>
                  <th className="text-right py-3 px-3 font-medium">Open Int</th>
                  <th className="text-right py-3 px-3 font-medium">$ Volume</th>
                  <th className="text-right py-3 px-3 font-medium">$ Open Int</th>
                  <th className="text-right py-3 px-3 font-medium">Trade Size</th>
                  <th className="text-center py-3 px-3 font-medium">Side</th>
                  <th className="text-right py-3 px-3 font-medium">Fee (bps)</th>
                  <th className="text-center py-3 px-3 font-medium">Event</th>
                  <th className="text-left py-3 px-3 font-medium">Time</th>
                  <th className="text-right py-3 px-3 font-medium">Latency</th>
                </tr>
              </thead>
              <tbody>
                {filteredTicks.slice(0, 50).map((tick, idx) => (
                  <tr
                    key={`${tick.contract_id}-${tick.timestamp}-${idx}`}
                    className="border-b border-zinc-800/50 hover:bg-zinc-800/30 transition-colors"
                  >
                    <td className="py-2 px-3 sticky left-0 bg-zinc-900/90">
                      <span className={`inline-flex items-center px-2 py-0.5 rounded text-xs font-medium ${
                        tick.source === 'KALSHI'
                          ? 'bg-blue-500/20 text-blue-400 border border-blue-500/30'
                          : 'bg-green-500/20 text-green-400 border border-green-500/30'
                      }`}>
                        {tick.source === 'KALSHI' ? 'KAL' : 'POLY'}
                      </span>
                    </td>
                    <td className="py-2 px-3 max-w-[250px]" title={tick.market_name || tick.contract_id}>
                      {tick.market_name ? (
                        <span className="text-zinc-200 truncate block text-xs">
                          {truncate(tick.market_name, 40)}
                        </span>
                      ) : (
                        <span className="text-zinc-500 font-mono text-xs">
                          {truncate(tick.contract_id, 20)}
                        </span>
                      )}
                    </td>
                    <td className="py-2 px-3 text-right font-mono font-bold text-white">
                      {(tick.price * 100).toFixed(1)}%
                    </td>
                    <td className="py-2 px-3 text-right font-mono text-zinc-400 text-xs">
                      {formatPrice(tick.yes_bid)}
                    </td>
                    <td className="py-2 px-3 text-right font-mono text-zinc-400 text-xs">
                      {formatPrice(tick.yes_ask)}
                    </td>
                    <td className="py-2 px-3 text-right font-mono text-xs">
                      <span className={tick.yes_bid && tick.yes_ask ? 'text-orange-400' : 'text-zinc-600'}>
                        {formatSpread(tick.yes_bid, tick.yes_ask)}
                      </span>
                    </td>
                    <td className="py-2 px-3 text-right font-mono text-zinc-500 text-xs">
                      {formatPrice(tick.no_bid)}
                    </td>
                    <td className="py-2 px-3 text-right font-mono text-zinc-500 text-xs">
                      {formatPrice(tick.no_ask)}
                    </td>
                    <td className="py-2 px-3 text-right font-mono text-zinc-400 text-xs">
                      {formatSize(tick.bid_size)}
                    </td>
                    <td className="py-2 px-3 text-right font-mono text-zinc-400 text-xs">
                      {formatSize(tick.ask_size)}
                    </td>
                    <td className="py-2 px-3 text-right text-zinc-400 text-xs">
                      {formatNumber(tick.volume)}
                    </td>
                    <td className="py-2 px-3 text-right text-zinc-400 text-xs">
                      {formatNumber(tick.open_interest)}
                    </td>
                    <td className="py-2 px-3 text-right text-zinc-400 text-xs">
                      {formatDollars(tick.dollar_volume)}
                    </td>
                    <td className="py-2 px-3 text-right text-zinc-400 text-xs">
                      {formatDollars(tick.dollar_open_interest)}
                    </td>
                    <td className="py-2 px-3 text-right font-mono text-zinc-400 text-xs">
                      {formatSize(tick.trade_size)}
                    </td>
                    <td className="py-2 px-3 text-center text-xs">
                      {tick.trade_side ? (
                        <span className={`px-1.5 py-0.5 rounded ${
                          tick.trade_side.toUpperCase().includes('BUY') || tick.trade_side.toUpperCase() === 'YES'
                            ? 'bg-green-500/20 text-green-400'
                            : 'bg-red-500/20 text-red-400'
                        }`}>
                          {tick.trade_side}
                        </span>
                      ) : '—'}
                    </td>
                    <td className="py-2 px-3 text-right font-mono text-zinc-500 text-xs">
                      {tick.fee_rate_bps ? tick.fee_rate_bps.toFixed(0) : '—'}
                    </td>
                    <td className="py-2 px-3 text-center text-xs">
                      {tick.event_type ? (
                        <span className="px-1.5 py-0.5 rounded bg-purple-500/20 text-purple-400 text-xs">
                          {tick.event_type}
                        </span>
                      ) : '—'}
                    </td>
                    <td className="py-2 px-3 text-zinc-500 font-mono text-xs whitespace-nowrap">
                      {formatTime(tick.timestamp)}
                    </td>
                    <td className="py-2 px-3 text-right font-mono text-xs">
                      <span className={
                        tick.latency_ms === undefined ? 'text-zinc-600' :
                        tick.latency_ms < 100 ? 'text-green-400' :
                        tick.latency_ms < 500 ? 'text-yellow-400' : 'text-red-400'
                      }>
                        {tick.latency_ms !== undefined ? `${tick.latency_ms}ms` : '—'}
                      </span>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
          {filteredTicks.length === 0 && (
            <div className="text-center py-12 text-zinc-500">
              <div className="text-4xl mb-2">📭</div>
              <div>No ticks yet. Waiting for data...</div>
            </div>
          )}
        </div>
      </main>

      {/* Footer */}
      <footer className="border-t border-zinc-800 mt-12">
        <div className="max-w-[1600px] mx-auto px-6 py-4">
          <div className="flex items-center justify-between text-xs text-zinc-500">
            <span>EchoArb © 2025</span>
            <span>Kalshi + Polymarket Real-Time Feed</span>
          </div>
        </div>
      </footer>
    </div>
  );
}
