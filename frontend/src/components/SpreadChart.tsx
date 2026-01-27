// src/components/SpreadChart.tsx
'use client';

import { LineChart, Line, XAxis, YAxis, CartesianGrid, Tooltip, Legend, ResponsiveContainer } from 'recharts';
import { format } from 'date-fns';

interface SpreadDataPoint {
  timestamp: string;
  kalshi_prob?: number | null;
  poly_prob?: number | null;
  manifold_prob?: number | null;
  max_spread: number;
}

interface SpreadChartProps {
  data: SpreadDataPoint[];
  pairId: string;
  description: string;
}

export default function SpreadChart({ data, pairId, description }: SpreadChartProps) {
  if (data.length === 0) {
    return (
      <div className="bg-white rounded-lg shadow p-6">
        <h3 className="text-lg font-semibold mb-4">{description}</h3>
        <div className="text-gray-500 text-center py-8">
          No data available yet
        </div>
      </div>
    );
  }

  const chartData = data.map((point) => ({
    ...point,
    time: new Date(point.timestamp).getTime(),
    kalshi: point.kalshi_prob ? point.kalshi_prob * 100 : null,
    poly: point.poly_prob ? point.poly_prob * 100 : null,
    manifold: point.manifold_prob ? point.manifold_prob * 100 : null,
    spread: point.max_spread * 100,
  }));

  return (
    <div className="bg-white rounded-lg shadow p-6">
      <h3 className="text-lg font-semibold mb-4">{description}</h3>

      <ResponsiveContainer width="100%" height={300}>
        <LineChart data={chartData}>
          <CartesianGrid strokeDasharray="3 3" />
          <XAxis
            dataKey="time"
            type="number"
            domain={['dataMin', 'dataMax']}
            tickFormatter={(timestamp) => format(new Date(timestamp), 'HH:mm:ss')}
          />
          <YAxis
            label={{ value: 'Probability (%)', angle: -90, position: 'insideLeft' }}
            domain={[0, 100]}
          />
          <Tooltip
            labelFormatter={(timestamp) => format(new Date(timestamp as number), 'HH:mm:ss')}
            formatter={(value: number) => `${value.toFixed(1)}%`}
          />
          <Legend />
          <Line
            type="monotone"
            dataKey="kalshi"
            stroke="#3b82f6"
            name="Kalshi"
            dot={false}
            strokeWidth={2}
            connectNulls
          />
          <Line
            type="monotone"
            dataKey="poly"
            stroke="#10b981"
            name="Polymarket"
            dot={false}
            strokeWidth={2}
            connectNulls
          />
          <Line
            type="monotone"
            dataKey="manifold"
            stroke="#f59e0b"
            name="Manifold"
            dot={false}
            strokeWidth={2}
            connectNulls
          />
          <Line
            type="monotone"
            dataKey="spread"
            stroke="#ef4444"
            name="Max Spread"
            dot={false}
            strokeWidth={2}
            strokeDasharray="5 5"
          />
        </LineChart>
      </ResponsiveContainer>

      <div className="mt-4 grid grid-cols-4 gap-4 text-sm">
        <div>
          <div className="text-gray-500">Latest Kalshi</div>
          <div className="font-semibold text-blue-600">
            {chartData[chartData.length - 1]?.kalshi !== null
              ? `${chartData[chartData.length - 1]?.kalshi?.toFixed(1)}%`
              : 'N/A'}
          </div>
        </div>
        <div>
          <div className="text-gray-500">Latest Poly</div>
          <div className="font-semibold text-green-600">
            {chartData[chartData.length - 1]?.poly !== null
              ? `${chartData[chartData.length - 1]?.poly?.toFixed(1)}%`
              : 'N/A'}
          </div>
        </div>
        <div>
          <div className="text-gray-500">Latest Manifold</div>
          <div className="font-semibold text-amber-600">
            {chartData[chartData.length - 1]?.manifold !== null
              ? `${chartData[chartData.length - 1]?.manifold?.toFixed(1)}%`
              : 'N/A'}
          </div>
        </div>
        <div>
          <div className="text-gray-500">Latest Spread</div>
          <div className="font-semibold text-red-600">
            {chartData[chartData.length - 1]?.spread?.toFixed(2)}%
          </div>
        </div>
      </div>
    </div>
  );
}
