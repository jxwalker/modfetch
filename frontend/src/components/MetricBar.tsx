import React from 'react';

interface MetricBarProps {
  label: string;
  value: number;
  color?: string;
}

/**
 * Visual metric bar component
 * Displays a metric with a progress bar visualization
 */
export const MetricBar: React.FC<MetricBarProps> = ({ label, value, color = 'var(--color-primary)' }) => {
  const percentage = Math.round(value * 100);

  return (
    <div className="metric-bar">
      <div className="metric-label">
        <span className="metric-label-name">{label}</span>
        <span className="metric-label-value">{percentage}%</span>
      </div>
      <div className="metric-bar-track">
        <div
          className="metric-bar-fill"
          style={{
            width: `${percentage}%`,
            background: color
          }}
        />
      </div>
    </div>
  );
};
