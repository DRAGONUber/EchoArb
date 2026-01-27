// src/components/LatencyDisplay.tsx
'use client';

import { useEffect, useState } from 'react';

interface LatencyData {
  source: string;
  avgLatency: number;
  p95Latency: number;
  lastUpdate: Date;
}

interface LatencyDisplayProps {
  latencies: LatencyData[];
}

export default function LatencyDisplay({ latencies }: LatencyDisplayProps) {
  const [currentTime, setCurrentTime] = useState(new Date());

  useEffect(() => {
    const interval = setInterval(() => setCurrentTime(new Date()), 1000);
    return () => clearInterval(interval);
  }, []);

  const getLatencyColor = (latency: number): string => {
    if (latency < 100) return 'text-green-600';
    if (latency < 500) return 'text-yellow-600';
    if (latency < 1000) return 'text-orange-600';
    return 'text-red-600';
  };

  const getLatencyStatus = (latency: number): string => {
    if (latency < 100) return 'Excellent';
    if (latency < 500) return 'Good';
    if (latency < 1000) return 'Fair';
    return 'Poor';
  };

  const getTimeSinceUpdate = (lastUpdate: Date): string => {
    const seconds = Math.floor((currentTime.getTime() - lastUpdate.getTime()) / 1000);
    if (seconds < 60) return `${seconds}s ago`;
    const minutes = Math.floor(seconds / 60);
    if (minutes < 60) return `${minutes}m ago`;
    const hours = Math.floor(minutes / 60);
    return `${hours}h ago`;
  };

  return (
    <div className="bg-white rounded-lg shadow p-6">
      <h3 className="text-lg font-semibold mb-4">Platform Latency</h3>

      {latencies.length === 0 ? (
        <div className="text-gray-500 text-center py-4">
          No latency data available
        </div>
      ) : (
        <div className="space-y-4">
          {latencies.map((data) => (
            <div key={data.source} className="border-b last:border-b-0 pb-4 last:pb-0">
              <div className="flex items-center justify-between mb-2">
                <div className="font-medium">{data.source}</div>
                <div className="text-sm text-gray-500">
                  {getTimeSinceUpdate(data.lastUpdate)}
                </div>
              </div>

              <div className="grid grid-cols-3 gap-4 text-sm">
                <div>
                  <div className="text-gray-500">Avg Latency</div>
                  <div className={`font-semibold ${getLatencyColor(data.avgLatency)}`}>
                    {data.avgLatency.toFixed(0)}ms
                  </div>
                </div>
                <div>
                  <div className="text-gray-500">P95 Latency</div>
                  <div className={`font-semibold ${getLatencyColor(data.p95Latency)}`}>
                    {data.p95Latency.toFixed(0)}ms
                  </div>
                </div>
                <div>
                  <div className="text-gray-500">Status</div>
                  <div className={`font-semibold ${getLatencyColor(data.avgLatency)}`}>
                    {getLatencyStatus(data.avgLatency)}
                  </div>
                </div>
              </div>

              {/* Latency bar visualization */}
              <div className="mt-2">
                <div className="h-2 bg-gray-200 rounded-full overflow-hidden">
                  <div
                    className={`h-full rounded-full ${
                      data.avgLatency < 100
                        ? 'bg-green-500'
                        : data.avgLatency < 500
                        ? 'bg-yellow-500'
                        : data.avgLatency < 1000
                        ? 'bg-orange-500'
                        : 'bg-red-500'
                    }`}
                    style={{
                      width: `${Math.min((data.avgLatency / 1000) * 100, 100)}%`,
                    }}
                  />
                </div>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
