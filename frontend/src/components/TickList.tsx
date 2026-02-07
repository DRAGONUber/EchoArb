// src/components/TickList.tsx
'use client';

import { Tick } from '@/lib/api';

interface TickListProps {
  ticks: Tick[];
}

function formatTimestamp(timestamp: string): string {
  const date = new Date(timestamp);
  if (Number.isNaN(date.getTime())) {
    return timestamp;
  }
  return date.toLocaleTimeString();
}

function formatPrice(price: number | null | undefined): string {
  if (price === null || price === undefined || price === 0) return '—';
  return `${(price * 100).toFixed(0)}¢`;
}

function formatSpread(bid: number | null | undefined, ask: number | null | undefined): string {
  if (!bid || !ask) return '—';
  const spread = (ask - bid) * 100;
  return `${spread.toFixed(1)}¢`;
}

function formatNumber(value: number | null | undefined): string {
  if (value === null || value === undefined || value === 0) return '—';
  if (value >= 1000000) return `${(value / 1000000).toFixed(1)}M`;
  if (value >= 1000) return `${(value / 1000).toFixed(1)}K`;
  return value.toLocaleString();
}

function formatTradeSize(size: number | null | undefined): string {
  if (size === null || size === undefined || size === 0) return '—';
  return size.toFixed(2);
}

function truncateText(text: string, maxLength: number): string {
  if (text.length <= maxLength) return text;
  return text.substring(0, maxLength) + '...';
}

export default function TickList({ ticks }: TickListProps) {
  const latestTicks = ticks.slice(0, 25);

  const getSourceColor = (source: Tick['source']) =>
    source === 'KALSHI' ? 'text-blue-600 bg-blue-50' : 'text-green-600 bg-green-50';

  return (
    <div className="bg-white rounded-lg shadow p-6">
      <h3 className="text-lg font-semibold mb-4">Live Market Ticks</h3>
      {latestTicks.length === 0 ? (
        <div className="text-gray-500 text-center py-4">No ticks yet</div>
      ) : (
        <div className="overflow-x-auto">
          <table className="min-w-full text-sm">
            <thead>
              <tr className="text-left text-gray-500 border-b text-xs uppercase tracking-wide">
                <th className="py-3 pr-3">Source</th>
                <th className="py-3 pr-3">Market Name</th>
                <th className="py-3 pr-3 text-right">Price</th>
                <th className="py-3 pr-3 text-right">Bid</th>
                <th className="py-3 pr-3 text-right">Ask</th>
                <th className="py-3 pr-3 text-right">Spread</th>
                <th className="py-3 pr-3 text-right">Volume</th>
                <th className="py-3 pr-3 text-right">Open Int.</th>
                <th className="py-3 pr-3 text-right">Trade Size</th>
                <th className="py-3 pr-3">Time</th>
                <th className="py-3 text-right">Latency</th>
              </tr>
            </thead>
            <tbody>
              {latestTicks.map((tick, index) => (
                <tr
                  key={`${tick.contract_id}-${tick.timestamp}-${index}`}
                  className="border-b last:border-b-0 hover:bg-gray-50 transition-colors"
                >
                  <td className="py-2 pr-3">
                    <span className={`px-2 py-1 rounded text-xs font-medium ${getSourceColor(tick.source)}`}>
                      {tick.source}
                    </span>
                  </td>
                  <td className="py-2 pr-3 text-gray-700 max-w-xs" title={tick.market_name || tick.contract_id}>
                    {tick.market_name ? (
                      <span className="block truncate text-sm">{truncateText(tick.market_name, 45)}</span>
                    ) : (
                      <span className="font-mono text-xs text-gray-500">{truncateText(tick.contract_id, 25)}</span>
                    )}
                  </td>
                  <td className="py-2 pr-3 text-right font-semibold text-gray-900">
                    {(tick.price * 100).toFixed(1)}%
                  </td>
                  <td className="py-2 pr-3 text-right font-mono text-xs text-gray-600">
                    {formatPrice(tick.yes_bid)}
                  </td>
                  <td className="py-2 pr-3 text-right font-mono text-xs text-gray-600">
                    {formatPrice(tick.yes_ask)}
                  </td>
                  <td className="py-2 pr-3 text-right font-mono text-xs">
                    <span className={tick.yes_bid && tick.yes_ask ? 'text-orange-600' : 'text-gray-400'}>
                      {formatSpread(tick.yes_bid, tick.yes_ask)}
                    </span>
                  </td>
                  <td className="py-2 pr-3 text-right text-gray-600">
                    {formatNumber(tick.volume)}
                  </td>
                  <td className="py-2 pr-3 text-right text-gray-600">
                    {formatNumber(tick.open_interest)}
                  </td>
                  <td className="py-2 pr-3 text-right font-mono text-xs text-gray-600">
                    {formatTradeSize(tick.trade_size)}
                  </td>
                  <td className="py-2 pr-3 text-gray-600 text-xs">
                    {formatTimestamp(tick.timestamp)}
                  </td>
                  <td className="py-2 text-right text-xs">
                    <span className={tick.latency_ms !== undefined && tick.latency_ms < 100 ? 'text-green-600' : 'text-gray-600'}>
                      {tick.latency_ms !== undefined ? `${tick.latency_ms}ms` : '—'}
                    </span>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
      <div className="mt-4 text-xs text-gray-400">
        Showing {latestTicks.length} most recent ticks
      </div>
    </div>
  );
}
