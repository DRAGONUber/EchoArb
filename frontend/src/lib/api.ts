// src/lib/api.ts
const API_URL = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8000';

export interface SpreadResult {
  pair_id: string;
  description: string;
  kalshi_prob: number | null;
  poly_prob: number | null;
  manifold_prob: number | null;
  kalshi_poly_spread: number | null;
  kalshi_manifold_spread: number | null;
  poly_manifold_spread: number | null;
  max_spread: number;
  max_spread_pair: 'KALSHI-POLY' | 'KALSHI-MANIFOLD' | 'POLY-MANIFOLD';
  timestamp: string;
  data_completeness: number;
}

export interface Alert {
  spread_result: SpreadResult;
  threshold: number;
  severity: 'low' | 'medium' | 'high' | 'critical';
  created_at: string;
}

export interface MarketPair {
  id: string;
  description: string;
  kalshi_tickers: string[];
  poly_token_id: string | null;
  manifold_slug: string | null;
  alert_threshold: number;
}

export interface CacheStats {
  kalshi_contracts: number;
  polymarket_contracts: number;
  manifold_contracts: number;
  total_contracts: number;
  last_updates: number;
}

export interface ConsumerStats {
  running: boolean;
  messages_processed: number;
  messages_failed: number;
  last_message_id: string | null;
  stream_name: string;
  consumer_group: string;
  consumer_name: string;
  pending_messages: number;
}

class APIClient {
  private baseURL: string;

  constructor(baseURL: string = API_URL) {
    this.baseURL = baseURL;
  }

  private async request<T>(endpoint: string, options?: RequestInit): Promise<T> {
    const url = `${this.baseURL}${endpoint}`;

    try {
      const response = await fetch(url, {
        ...options,
        headers: {
          'Content-Type': 'application/json',
          ...options?.headers,
        },
      });

      if (!response.ok) {
        const error = await response.json().catch(() => ({ detail: response.statusText }));
        throw new Error(error.detail || `HTTP ${response.status}`);
      }

      return await response.json();
    } catch (error) {
      console.error(`API request failed: ${endpoint}`, error);
      throw error;
    }
  }

  // Spread endpoints
  async getSpreads(): Promise<SpreadResult[]> {
    return this.request<SpreadResult[]>('/api/v1/spreads');
  }

  async getSpread(pairId: string): Promise<SpreadResult> {
    return this.request<SpreadResult>(`/api/v1/spreads/${pairId}`);
  }

  // Alert endpoints
  async getAlerts(minThreshold: number = 0.05): Promise<Alert[]> {
    return this.request<Alert[]>(`/api/v1/alerts?min_threshold=${minThreshold}`);
  }

  // Market pair endpoints
  async getMarketPairs(): Promise<{ pairs: MarketPair[]; count: number }> {
    return this.request('/api/v1/pairs');
  }

  // Stats endpoints
  async getCacheStats(): Promise<CacheStats> {
    return this.request<CacheStats>('/api/v1/stats/cache');
  }

  async getConsumerStats(): Promise<ConsumerStats> {
    return this.request<ConsumerStats>('/api/v1/stats/consumer');
  }

  // Health check
  async healthCheck(): Promise<{ status: string; environment: string; redis_connected: boolean; market_pairs: number }> {
    return this.request('/health');
  }

  // Debug endpoint (development only)
  async debugUpdatePrice(source: string, contractId: string, price: number): Promise<any> {
    return this.request('/api/v1/debug/update_price', {
      method: 'POST',
      body: JSON.stringify({
        source,
        contract_id: contractId,
        price,
      }),
    });
  }
}

export const api = new APIClient();

// WebSocket URL
export function getWebSocketURL(path: string = '/ws/spreads'): string {
  const wsURL = process.env.NEXT_PUBLIC_WS_URL || 'ws://localhost:8000';
  return `${wsURL}${path}`;
}

// Helper functions
export function formatProbability(prob: number | null | undefined): string {
  if (prob === null || prob === undefined) return 'N/A';
  return `${(prob * 100).toFixed(1)}%`;
}

export function formatSpread(spread: number | null | undefined): string {
  if (spread === null || spread === undefined) return 'N/A';
  return `${(spread * 100).toFixed(2)}%`;
}

export function getSeverityColor(severity: string): string {
  switch (severity) {
    case 'critical':
      return 'text-red-600 bg-red-50';
    case 'high':
      return 'text-orange-600 bg-orange-50';
    case 'medium':
      return 'text-yellow-600 bg-yellow-50';
    case 'low':
      return 'text-blue-600 bg-blue-50';
    default:
      return 'text-gray-600 bg-gray-50';
  }
}

export function getSpreadColor(spread: number): string {
  if (spread >= 0.20) return 'text-red-600';
  if (spread >= 0.15) return 'text-orange-600';
  if (spread >= 0.10) return 'text-yellow-600';
  if (spread >= 0.05) return 'text-blue-600';
  return 'text-gray-600';
}
