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

export default function TickList({ ticks }: TickListProps) {
  const latestTicks = ticks.slice(0, 20);

  const getSourceColor = (source: Tick['source']) =>
    source === 'KALSHI' ? 'text-blue-600' : 'text-green-600';

  return (
    <div className="bg-white rounded-lg shadow p-6">
      <h3 className="text-lg font-semibold mb-4">Latest Ticks</h3>
      {latestTicks.length === 0 ? (
        <div className="text-gray-500 text-center py-4">No ticks yet</div>
      ) : (
        <div className="overflow-x-auto">
          <table className="min-w-full text-sm">
            <thead>
              <tr className="text-left text-gray-500 border-b">
                <th className="py-2 pr-4">Source</th>
                <th className="py-2 pr-4">Market</th>
                <th className="py-2 pr-4">Price</th>
                <th className="py-2 pr-4">Timestamp</th>
                <th className="py-2">Latency</th>
              </tr>
            </thead>
            <tbody>
              {latestTicks.map((tick, index) => (
                <tr key={`${tick.contract_id}-${tick.timestamp}-${index}`} className="border-b last:border-b-0">
                  <td className={`py-2 pr-4 font-medium ${getSourceColor(tick.source)}`}>
                    {tick.source}
                  </td>
                  <td className="py-2 pr-4 text-gray-700">{tick.contract_id}</td>
                  <td className="py-2 pr-4 text-gray-900">{(tick.price * 100).toFixed(2)}%</td>
                  <td className="py-2 pr-4 text-gray-600">{formatTimestamp(tick.timestamp)}</td>
                  <td className="py-2 text-gray-600">
                    {tick.latency_ms !== undefined ? `${tick.latency_ms}ms` : '—'}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}
