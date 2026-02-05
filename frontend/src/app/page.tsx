// src/app/page.tsx
'use client';

import { useCallback, useEffect, useRef, useState } from 'react';
import { useWebSocket } from '@/hooks/useWebSocket';
import { api, getWebSocketURL, Tick } from '@/lib/api';
import LatencyDisplay from '@/components/LatencyDisplay';
import TickList from '@/components/TickList';

export default function Home() {
  const [ticks, setTicks] = useState<Tick[]>([]);
  const [ticksPerSecond, setTicksPerSecond] = useState(0);
  const [latencies, setLatencies] = useState([
    { source: 'KALSHI', avgLatency: 0, p95Latency: 0, lastUpdate: new Date() },
    { source: 'POLYMARKET', avgLatency: 0, p95Latency: 0, lastUpdate: new Date() },
  ]);
  const tickTimesRef = useRef<number[]>([]);

  const updateLatency = useCallback((source: string, latency: number) => {
    setLatencies((prev) =>
      prev.map((item) =>
        item.source === source
          ? {
              ...item,
              avgLatency: latency,
              p95Latency: latency * 1.2, // Estimate
              lastUpdate: new Date(),
            }
          : item
      )
    );
  }, []);

  const handleWebSocketMessage = useCallback(
    (message: any) => {
      if (message.type === 'tick') {
        const newTick: Tick = {
          source: message.source,
          contract_id: message.contract_id,
          price: message.price,
          timestamp: message.timestamp,
          latency_ms: message.latency_ms,
        };
        setTicks((prev) => [newTick, ...prev].slice(0, 100));

        const now = Date.now();
        tickTimesRef.current = [...tickTimesRef.current, now].filter((ts) => now - ts <= 1000);
        setTicksPerSecond(tickTimesRef.current.length);

        if (typeof message.latency_ms === 'number') {
          updateLatency(message.source, message.latency_ms);
        }
      }
    },
    [updateLatency]
  );

  // WebSocket connection
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
      } catch (error) {
        console.error('Failed to fetch initial data:', error);
      }
    }
  }, [updateLatency]);

  // 3. Initialize WebSocket with the stable handler
  const { connected } = useWebSocket({
    url: getWebSocketURL('/ws/spreads'),
    onMessage: handleWebSocketMessage,
    reconnectInterval: 3000,
    maxReconnectAttempts: 10,
  });

  // Tick rate updater
  useEffect(() => {
    const interval = setInterval(() => {
      const now = Date.now();
      tickTimesRef.current = tickTimesRef.current.filter((ts) => now - ts <= 1000);
      setTicksPerSecond(tickTimesRef.current.length);
    }, 1000);
    return () => clearInterval(interval);
  }, []);

  return (
    <main className="min-h-screen bg-gray-50">
      {/* Header */}
      <header className="bg-white shadow-sm">
        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-4">
          <div className="flex items-center justify-between">
            <div>
              <h1 className="text-3xl font-bold text-gray-900">EchoArb</h1>
              <p className="text-sm text-gray-500 mt-1">
                Real-time prediction market tick tape
              </p>
            </div>
            <div className="flex items-center gap-4">
              <div className="flex items-center gap-2">
                <div
                  className={`w-3 h-3 rounded-full ${
                    connected ? 'bg-green-500 animate-pulse' : 'bg-red-500'
                  }`}
                />
                <span className="text-sm text-gray-600">
                  {connected ? 'Connected' : 'Disconnected'}
                </span>
              </div>
              <div className="text-sm text-gray-500">{ticksPerSecond} ticks/sec</div>
            </div>
          </div>
        </div>
      </header>

      {/* Main Content */}
      <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
        {/* Main Grid */}
        <div className="grid grid-cols-1 lg:grid-cols-3 gap-8">
          {/* Left Column: Tick List */}
          <div className="lg:col-span-2">
            <TickList ticks={ticks} />
          </div>

          {/* Right Column: Latency */}
          <div>
            <LatencyDisplay latencies={latencies} />
          </div>
        </div>
      </div>

      {/* Footer */}
      <footer className="bg-white border-t mt-12">
        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-6">
          <div className="text-center text-sm text-gray-500">
            <p>EchoArb © 2025 - Real-time ticks across Kalshi and Polymarket</p>
          </div>
        </div>
      </footer>
    </main>
  );
}
