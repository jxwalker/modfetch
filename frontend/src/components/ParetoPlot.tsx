import React from 'react';
import { ScatterChart, Scatter, XAxis, YAxis, CartesianGrid, Tooltip, Legend, ResponsiveContainer, ZAxis } from 'recharts';
import { Candidate } from '../types';

interface ParetoPlotProps {
  candidates: Candidate[];
  xMetric: keyof Candidate['metrics'];
  yMetric: keyof Candidate['metrics'];
  xLabel: string;
  yLabel: string;
}

/**
 * Pareto Front Scatter Plot
 *
 * Visualizes candidates in 2D metric space, highlighting Pareto-optimal solutions
 */
export const ParetoPlot: React.FC<ParetoPlotProps> = ({
  candidates,
  xMetric,
  yMetric,
  xLabel,
  yLabel
}) => {
  // Prepare data for scatter plot
  const paretoData = candidates
    .filter(c => c.is_pareto_front && c.gates_passed)
    .map(c => ({
      x: c.metrics[xMetric] * 100,
      y: c.metrics[yMetric] * 100,
      name: c.id,
      size: c.selected_for_breeding ? 200 : 100,
    }));

  const nonParetoData = candidates
    .filter(c => !c.is_pareto_front || !c.gates_passed)
    .map(c => ({
      x: c.metrics[xMetric] * 100,
      y: c.metrics[yMetric] * 100,
      name: c.id,
      size: 80,
    }));

  return (
    <ResponsiveContainer width="100%" height={400}>
      <ScatterChart margin={{ top: 20, right: 20, bottom: 20, left: 20 }}>
        <CartesianGrid strokeDasharray="3 3" />
        <XAxis
          type="number"
          dataKey="x"
          name={xLabel}
          unit="%"
          domain={[0, 100]}
          label={{ value: xLabel, position: 'insideBottom', offset: -10 }}
        />
        <YAxis
          type="number"
          dataKey="y"
          name={yLabel}
          unit="%"
          domain={[0, 100]}
          label={{ value: yLabel, angle: -90, position: 'insideLeft' }}
        />
        <ZAxis type="number" dataKey="size" range={[50, 200]} />
        <Tooltip
          cursor={{ strokeDasharray: '3 3' }}
          content={({ active, payload }) => {
            if (active && payload && payload.length) {
              const data = payload[0].payload;
              return (
                <div style={{
                  background: 'white',
                  padding: '10px',
                  border: '1px solid #ccc',
                  borderRadius: '4px'
                }}>
                  <p style={{ fontWeight: 'bold', marginBottom: '5px' }}>{data.name}</p>
                  <p>{xLabel}: {data.x.toFixed(1)}%</p>
                  <p>{yLabel}: {data.y.toFixed(1)}%</p>
                </div>
              );
            }
            return null;
          }}
        />
        <Legend />
        <Scatter
          name="Non-Pareto"
          data={nonParetoData}
          fill="#94a3b8"
          opacity={0.5}
        />
        <Scatter
          name="Pareto Front"
          data={paretoData}
          fill="#8b5cf6"
        />
      </ScatterChart>
    </ResponsiveContainer>
  );
};
