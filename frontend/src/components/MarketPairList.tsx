// src/components/MarketPairList.tsx
'use client';

import { formatProbability, formatSpread, getSpreadColor } from '@/lib/api';

interface SpreadResult {
  pair_id: string;
  description: string;
  kalshi_prob: number | null;
  poly_prob: number | null;
  manifold_prob: number | null;
  max_spread: number;
  max_spread_pair: string;
  data_completeness: number;
  timestamp: string;
}

interface MarketPairListProps {
  spreads: SpreadResult[];
}

export default function MarketPairList({ spreads }: MarketPairListProps) {
  if (spreads.length === 0) {
    return (
      <div className="bg-white rounded-lg shadow p-6">
        <h3 className="text-lg font-semibold mb-4">Market Pairs</h3>
        <div className="text-gray-500 text-center py-8">
          No market pairs configured yet
        </div>
      </div>
    );
  }

  // Sort by max spread descending
  const sortedSpreads = [...spreads].sort((a, b) => b.max_spread - a.max_spread);

  return (
    <div className="bg-white rounded-lg shadow p-6">
      <div className="flex items-center justify-between mb-4">
        <h3 className="text-lg font-semibold">Market Pairs</h3>
        <div className="text-sm text-gray-500">{spreads.length} pairs</div>
      </div>

      <div className="space-y-4">
        {sortedSpreads.map((spread) => (
          <div
            key={spread.pair_id}
            className="border rounded-lg p-4 hover:shadow-md transition-shadow"
          >
            {/* Header */}
            <div className="flex items-start justify-between mb-3">
              <div className="flex-1">
                <h4 className="font-medium text-gray-900">{spread.description}</h4>
                <div className="text-xs text-gray-500 mt-1">{spread.pair_id}</div>
              </div>
              <div className="ml-4">
                <div
                  className={`text-2xl font-bold ${getSpreadColor(spread.max_spread)}`}
                >
                  {formatSpread(spread.max_spread)}
                </div>
                <div className="text-xs text-gray-500 text-right">
                  {spread.max_spread_pair}
                </div>
              </div>
            </div>

            {/* Probabilities Grid */}
            <div className="grid grid-cols-2 gap-3">
              {/* Kalshi */}
              <div className="bg-blue-50 rounded p-2">
                <div className="text-xs text-gray-600 mb-1">Kalshi</div>
                <div className="text-lg font-semibold text-blue-700">
                  {formatProbability(spread.kalshi_prob)}
                </div>
              </div>

              {/* Polymarket */}
              <div className="bg-green-50 rounded p-2">
                <div className="text-xs text-gray-600 mb-1">Polymarket</div>
                <div className="text-lg font-semibold text-green-700">
                  {formatProbability(spread.poly_prob)}
                </div>
              </div>
            </div>

            {/* Footer */}
            <div className="mt-3 flex items-center justify-between text-xs text-gray-500">
              <div className="flex items-center gap-2">
                <div className="flex items-center gap-1">
                  <div
                    className={`w-2 h-2 rounded-full ${
                      spread.data_completeness === 1.0
                        ? 'bg-green-500'
                        : 'bg-yellow-500'
                    }`}
                  />
                  <span>
                    {spread.data_completeness === 1.0 ? 'Both sources' : 'Partial data'}
                  </span>
                </div>
              </div>
              <div>
                Updated {new Date(spread.timestamp).toLocaleTimeString()}
              </div>
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}
