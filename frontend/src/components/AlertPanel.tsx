// src/components/AlertPanel.tsx
'use client';

import { getSeverityColor, formatSpread } from '@/lib/api';

interface Alert {
  spread_result: {
    pair_id: string;
    description: string;
    max_spread: number;
    max_spread_pair: string;
  };
  threshold: number;
  severity: 'low' | 'medium' | 'high' | 'critical';
  created_at: string;
}

interface AlertPanelProps {
  alerts: Alert[];
  onDismiss?: (alert: Alert) => void;
}

export default function AlertPanel({ alerts, onDismiss }: AlertPanelProps) {
  const getSeverityIcon = (severity: string): string => {
    switch (severity) {
      case 'critical':
        return '🔴';
      case 'high':
        return '🟠';
      case 'medium':
        return '🟡';
      case 'low':
        return '🔵';
      default:
        return '⚪';
    }
  };

  const getSeverityLabel = (severity: string): string => {
    return severity.charAt(0).toUpperCase() + severity.slice(1);
  };

  if (alerts.length === 0) {
    return (
      <div className="bg-white rounded-lg shadow p-6">
        <h3 className="text-lg font-semibold mb-4">Alerts</h3>
        <div className="text-gray-500 text-center py-8">
          <div className="text-4xl mb-2">✅</div>
          <div>No alerts at this time</div>
        </div>
      </div>
    );
  }

  // Sort by severity (critical → high → medium → low) and then by timestamp
  const severityOrder = { critical: 0, high: 1, medium: 2, low: 3 };
  const sortedAlerts = [...alerts].sort((a, b) => {
    const severityDiff =
      severityOrder[a.severity] - severityOrder[b.severity];
    if (severityDiff !== 0) return severityDiff;
    return new Date(b.created_at).getTime() - new Date(a.created_at).getTime();
  });

  return (
    <div className="bg-white rounded-lg shadow p-6">
      <div className="flex items-center justify-between mb-4">
        <h3 className="text-lg font-semibold">Alerts</h3>
        <div className="flex items-center gap-2">
          <span className="text-sm text-gray-500">{alerts.length} active</span>
          {alerts.some((a) => a.severity === 'critical' || a.severity === 'high') && (
            <span className="animate-pulse text-red-500 text-xl">⚠️</span>
          )}
        </div>
      </div>

      <div className="space-y-3 max-h-96 overflow-y-auto">
        {sortedAlerts.map((alert, index) => (
          <div
            key={index}
            className={`rounded-lg p-4 border-l-4 ${
              alert.severity === 'critical'
                ? 'border-red-500 bg-red-50'
                : alert.severity === 'high'
                ? 'border-orange-500 bg-orange-50'
                : alert.severity === 'medium'
                ? 'border-yellow-500 bg-yellow-50'
                : 'border-blue-500 bg-blue-50'
            }`}
          >
            {/* Header */}
            <div className="flex items-start justify-between mb-2">
              <div className="flex items-center gap-2">
                <span className="text-xl">{getSeverityIcon(alert.severity)}</span>
                <span
                  className={`text-xs font-semibold px-2 py-1 rounded ${getSeverityColor(
                    alert.severity
                  )}`}
                >
                  {getSeverityLabel(alert.severity)}
                </span>
              </div>
              {onDismiss && (
                <button
                  onClick={() => onDismiss(alert)}
                  className="text-gray-400 hover:text-gray-600 text-sm"
                  aria-label="Dismiss alert"
                >
                  ✕
                </button>
              )}
            </div>

            {/* Content */}
            <div className="mb-2">
              <div className="font-medium text-gray-900">
                {alert.spread_result.description}
              </div>
              <div className="text-sm text-gray-600 mt-1">
                {alert.spread_result.max_spread_pair} spread:{' '}
                <span className="font-semibold">
                  {formatSpread(alert.spread_result.max_spread)}
                </span>
              </div>
            </div>

            {/* Footer */}
            <div className="flex items-center justify-between text-xs text-gray-500">
              <div>
                Threshold: {formatSpread(alert.threshold)}
              </div>
              <div>{new Date(alert.created_at).toLocaleTimeString()}</div>
            </div>
          </div>
        ))}
      </div>

      {alerts.length > 5 && (
        <div className="mt-3 text-center text-sm text-gray-500">
          Scroll for more alerts
        </div>
      )}
    </div>
  );
}
