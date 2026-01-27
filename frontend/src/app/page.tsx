// src/app/page.tsx
'use client';

import { useEffect, useState, useCallback } from 'react';
import { useWebSocket } from '@/hooks/useWebSocket';
import { api, getWebSocketURL, SpreadResult, Alert } from '@/lib/api';
import SpreadChart from '@/components/SpreadChart';
import LatencyDisplay from '@/components/LatencyDisplay';
import MarketPairList from '@/components/MarketPairList';
import AlertPanel from '@/components/AlertPanel';

interface SpreadHistory {
  [pairId: string]: SpreadResult[];
}

export default function Home() {
  const [spreads, setSpreads] = useState<SpreadResult[]>([]);
  const [alerts, setAlerts] = useState<Alert[]>([]);
  const [spreadHistory, setSpreadHistory] = useState<SpreadHistory>({});
  const [stats, setStats] = useState<{ cache: any; consumer: any }>({ cache: null, consumer: null });
  const [latencies, setLatencies] = useState([
    { source: 'KALSHI', avgLatency: 0, p95Latency: 0, lastUpdate: new Date() },
    { source: 'POLYMARKET', avgLatency: 0, p95Latency: 0, lastUpdate: new Date() },
    { source: 'MANIFOLD', avgLatency: 0, p95Latency: 0, lastUpdate: new Date() },
  ]);

  // WebSocket connection
  const { connected, lastMessage } = useWebSocket({
    url: getWebSocketURL('/ws/spreads'),
    onMessage: handleWebSocketMessage,
    reconnectInterval: 3000,
    maxReconnectAttempts: 10,
  });

  function handleWebSocketMessage(message: any) {
    if (message.type === 'spread_update' || message.type === 'initial_spreads') {
      const newSpreads = message.spreads || [];
      setSpreads(newSpreads);

      // Update spread history
      setSpreadHistory((prev) => {
        const updated = { ...prev };
        newSpreads.forEach((spread: SpreadResult) => {
          if (!updated[spread.pair_id]) {
            updated[spread.pair_id] = [];
          }
          // Keep last 100 points
          updated[spread.pair_id] = [...updated[spread.pair_id], spread].slice(-100);
        });
        return updated;
      });

      // Update latency if available
      if (message.trigger_tick?.latency_ms) {
        updateLatency(message.trigger_tick.source, message.trigger_tick.latency_ms);
      }
    }
  }

  function updateLatency(source: string, latency: number) {
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
  }

  // Fetch initial data
  useEffect(() => {
    async function fetchInitialData() {
      try {
        const [spreadsData, alertsData] = await Promise.all([
          api.getSpreads(),
          api.getAlerts(0.05),
        ]);
        setSpreads(spreadsData);
        setAlerts(alertsData);
      } catch (error) {
        console.error('Failed to fetch initial data:', error);
      }
    }

    fetchInitialData();
  }, []);

  // Periodic stats update
  useEffect(() => {
    async function fetchStats() {
      try {
        const [cacheStats, consumerStats] = await Promise.all([
          api.getCacheStats(),
          api.getConsumerStats(),
        ]);
        setStats({ cache: cacheStats, consumer: consumerStats });
      } catch (error) {
        console.error('Failed to fetch stats:', error);
      }
    }

    fetchStats();
    const interval = setInterval(fetchStats, 10000); // Every 10 seconds
    return () => clearInterval(interval);
  }, []);

  // Update alerts based on spreads
  useEffect(() => {
    const newAlerts: Alert[] = spreads
      .filter((spread) => spread.max_spread >= 0.05)
      .map((spread) => ({
        spread_result: spread,
        threshold: 0.05,
        severity:
          spread.max_spread >= 0.20
            ? ('critical' as const)
            : spread.max_spread >= 0.15
            ? ('high' as const)
            : spread.max_spread >= 0.10
            ? ('medium' as const)
            : ('low' as const),
        created_at: spread.timestamp,
      }));
    setAlerts(newAlerts);
  }, [spreads]);

  const dismissAlert = useCallback((alert: Alert) => {
    setAlerts((prev) => prev.filter((a) => a !== alert));
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
                Real-time prediction market arbitrage scanner
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
              {stats.consumer && (
                <div className="text-sm text-gray-500">
                  {(stats.consumer as any).messages_processed} ticks processed
                </div>
              )}
            </div>
          </div>
        </div>
      </header>

      {/* Main Content */}
      <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
        {/* Alerts Section */}
        {alerts.length > 0 && (
          <div className="mb-8">
            <AlertPanel alerts={alerts} onDismiss={dismissAlert} />
          </div>
        )}

        {/* Stats Row */}
        <div className="grid grid-cols-1 lg:grid-cols-3 gap-6 mb-8">
          <div className="bg-white rounded-lg shadow p-6">
            <h3 className="text-sm font-medium text-gray-500 mb-2">Active Pairs</h3>
            <div className="text-3xl font-bold text-gray-900">{spreads.length}</div>
          </div>
          <div className="bg-white rounded-lg shadow p-6">
            <h3 className="text-sm font-medium text-gray-500 mb-2">Avg Spread</h3>
            <div className="text-3xl font-bold text-gray-900">
              {spreads.length > 0
                ? `${(
                    (spreads.reduce((sum, s) => sum + s.max_spread, 0) /
                      spreads.length) *
                    100
                  ).toFixed(2)}%`
                : 'N/A'}
            </div>
          </div>
          <div className="bg-white rounded-lg shadow p-6">
            <h3 className="text-sm font-medium text-gray-500 mb-2">Max Spread</h3>
            <div className="text-3xl font-bold text-red-600">
              {spreads.length > 0
                ? `${(Math.max(...spreads.map((s) => s.max_spread)) * 100).toFixed(2)}%`
                : 'N/A'}
            </div>
          </div>
        </div>

        {/* Main Grid */}
        <div className="grid grid-cols-1 lg:grid-cols-3 gap-8">
          {/* Left Column: Market Pairs */}
          <div className="lg:col-span-2">
            <MarketPairList spreads={spreads} />

            {/* Charts */}
            <div className="mt-8 space-y-8">
              {Object.entries(spreadHistory).map(([pairId, history]) => (
                <SpreadChart
                  key={pairId}
                  data={history}
                  pairId={pairId}
                  description={history[history.length - 1]?.description || pairId}
                />
              ))}
            </div>
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
            <p>EchoArb © 2025 - Real-time arbitrage across Kalshi, Polymarket, and Manifold</p>
          </div>
        </div>
      </footer>
    </main>
  );
}