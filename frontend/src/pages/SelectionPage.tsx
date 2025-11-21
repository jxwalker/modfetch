import React, { useEffect, useState } from 'react';
import { getRun } from '../api/runs';
import { RunWithGenerations, Generation } from '../types';
import { ParetoPlot } from '../components/ParetoPlot';

/**
 * Selection, GEPA and Agent Economics Page
 *
 * Purpose: Show the novelty of selection, diversity, Pareto front, and UCB-style allocation
 *
 * Highlights:
 * - Pareto scatter plot visualization
 * - GEPA (Generalized Evolutionary Pareto Algorithm) selection
 * - UCB (Upper Confidence Bound) and Expected Information Gain
 * - Resource allocation based on agent economics
 */
const SelectionPage: React.FC = () => {
  const [run, setRun] = useState<RunWithGenerations | null>(null);
  const [selectedGeneration, setSelectedGeneration] = useState<number>(3);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    getRun('run_001')
      .then(data => {
        setRun(data);
        setLoading(false);
      })
      .catch(() => setLoading(false));
  }, []);

  if (loading) {
    return <div className="page loading">Loading selection data...</div>;
  }

  if (!run) {
    return <div className="page">Error loading data</div>;
  }

  const generation: Generation = run.generations[selectedGeneration - 1];
  const selectedCandidates = generation.candidates.filter(c => c.selected_for_breeding);
  const paretoFront = generation.candidates.filter(c => c.is_pareto_front);

  return (
    <div className="page">
      <div className="page-header">
        <h1 className="page-title">Selection, GEPA & Agent Economics</h1>
        <p className="page-subtitle">
          Pareto-optimal selection with diversity preservation and intelligent resource allocation
        </p>
      </div>

      {/* Generation Selector */}
      <div className="card mb-4">
        <h3 className="card-title">Select Generation to Analyze</h3>
        <div className="flex gap-2">
          {run.generations.map((gen) => (
            <button
              key={gen.generation_number}
              className={`btn ${selectedGeneration === gen.generation_number ? 'btn-primary' : 'btn-secondary'}`}
              onClick={() => setSelectedGeneration(gen.generation_number)}
            >
              Gen {gen.generation_number}
            </button>
          ))}
        </div>
      </div>

      {/* Pareto Front Visualization */}
      <div className="section">
        <h2 className="section-title">Pareto Front Visualization</h2>
        <p className="section-subtitle">
          Candidates plotted in 2D metric space. Purple dots represent Pareto-optimal solutions.
        </p>

        <div className="grid grid-2 gap-3">
          <div className="card">
            <h3 className="card-title">Security vs. Performance</h3>
            <ParetoPlot
              candidates={generation.candidates}
              xMetric="security_score"
              yMetric="performance_score"
              xLabel="Security Score"
              yLabel="Performance Score"
            />
          </div>

          <div className="card">
            <h3 className="card-title">Functionality vs. UX</h3>
            <ParetoPlot
              candidates={generation.candidates}
              xMetric="functionality_score"
              yMetric="ux_score"
              xLabel="Functionality"
              yLabel="UX Score"
            />
          </div>
        </div>

        <div className="card mt-3" style={{ background: 'var(--color-bg-secondary)' }}>
          <h4 className="card-title">What is the Pareto Front?</h4>
          <p className="card-content">
            A solution is <strong>Pareto-optimal</strong> if no other solution is strictly better
            in all dimensions. The Pareto front represents the set of optimal tradeoffs—improving
            one metric requires sacrificing another. For example, a highly secure solution might
            have lower performance, while a fast solution might have moderate security.
          </p>
          <p className="card-content mt-2">
            <strong>Innovation:</strong> GAD uses the Pareto front to preserve diversity. Rather
            than selecting only the highest-scoring candidate, it selects multiple Pareto-optimal
            candidates that represent different tradeoff strategies. This prevents premature
            convergence and maintains exploration.
          </p>
        </div>
      </div>

      {/* Selection Table */}
      <div className="section">
        <h2 className="section-title">Selection Decisions</h2>
        <p className="section-subtitle">
          Candidates that survived selection for breeding in the next generation
        </p>

        <table className="table">
          <thead>
            <tr>
              <th>Candidate</th>
              <th>Effective Score</th>
              <th>Pareto Front</th>
              <th>UCB Score</th>
              <th>Info Gain</th>
              <th>Children</th>
              <th>Status</th>
            </tr>
          </thead>
          <tbody>
            {generation.candidates
              .sort((a, b) => (b.effective_score || 0) - (a.effective_score || 0))
              .map((candidate) => (
                <tr key={candidate.id} style={{
                  background: candidate.selected_for_breeding ? 'rgba(16, 185, 129, 0.1)' : 'transparent'
                }}>
                  <td><strong>{candidate.id}</strong></td>
                  <td>{(candidate.effective_score * 100).toFixed(0)}%</td>
                  <td>
                    {candidate.is_pareto_front ? (
                      <span className="badge badge-pareto">Yes</span>
                    ) : (
                      <span className="badge" style={{ background: '#f1f5f9', color: '#64748b' }}>No</span>
                    )}
                  </td>
                  <td>{candidate.ucb_score?.toFixed(2) || '-'}</td>
                  <td>{candidate.expected_info_gain?.toFixed(2) || '-'}</td>
                  <td><strong>{candidate.children_allocated || 0}</strong></td>
                  <td>
                    {candidate.selected_for_breeding ? (
                      <span className="badge badge-success">Selected</span>
                    ) : !candidate.gates_passed ? (
                      <span className="badge badge-danger">Failed Gates</span>
                    ) : (
                      <span className="badge badge-warning">Discarded</span>
                    )}
                  </td>
                </tr>
              ))}
          </tbody>
        </table>
      </div>

      {/* GEPA Algorithm Explanation */}
      <div className="section">
        <h2 className="section-title">GEPA: Generalized Evolutionary Pareto Algorithm</h2>

        <div className="card">
          <h3 className="card-title">How GEPA Works</h3>
          <div className="card-content">
            <ol style={{ paddingLeft: '1.5rem', lineHeight: '1.8' }}>
              <li>
                <strong>Filter by Hard Gates:</strong> Discard any candidates that failed
                required gates (e.g., test_pass_rate &lt; 0.70, critical security issues)
              </li>
              <li>
                <strong>Identify Pareto Front:</strong> Among candidates that passed gates,
                identify those that are Pareto-optimal across all metrics
              </li>
              <li>
                <strong>Preserve Diversity:</strong> Select top-K survivors from Pareto front
                using crowding distance to maintain spread across the tradeoff space
              </li>
              <li>
                <strong>Resource Allocation:</strong> Assign breeding budget (number of children)
                to each survivor based on UCB score and expected information gain
              </li>
            </ol>

            <div style={{ marginTop: '1rem', padding: '1rem', background: 'var(--color-bg-secondary)', borderRadius: 'var(--radius)' }}>
              <strong>Result for Generation {selectedGeneration}:</strong>
              <ul style={{ marginTop: '0.5rem', paddingLeft: '1.5rem' }}>
                <li>{generation.candidates.length} total candidates</li>
                <li>{paretoFront.length} on Pareto front</li>
                <li>{selectedCandidates.length} selected for breeding</li>
                <li>Total children allocated: {selectedCandidates.reduce((sum, c) => sum + (c.children_allocated || 0), 0)}</li>
              </ul>
            </div>
          </div>
        </div>
      </div>

      {/* Agent Economics: UCB and Expected Info Gain */}
      <div className="section">
        <h2 className="section-title">Agent Economics: UCB & Expected Information Gain</h2>

        <div className="grid grid-2 gap-3">
          <div className="card">
            <h3 className="card-title">Upper Confidence Bound (UCB)</h3>
            <p className="card-content">
              UCB balances <strong>exploitation</strong> (choosing known high-performers) and
              <strong>exploration</strong> (trying promising but uncertain candidates).
            </p>
            <div style={{ marginTop: '1rem', fontFamily: 'monospace', background: 'var(--color-bg-secondary)', padding: '1rem', borderRadius: 'var(--radius)' }}>
              UCB = mean_score + c × √(ln(N) / n)
            </div>
            <p className="card-content" style={{ marginTop: '1rem', fontSize: '0.9rem' }}>
              Where <strong>c</strong> controls exploration, <strong>N</strong> is total evaluations,
              and <strong>n</strong> is evaluations of this candidate's lineage.
            </p>
          </div>

          <div className="card">
            <h3 className="card-title">Expected Information Gain</h3>
            <p className="card-content">
              Measures how much we expect to learn from breeding this candidate. Candidates
              with high variance or unexplored mutations have high information gain.
            </p>
            <p className="card-content" style={{ marginTop: '1rem' }}>
              <strong>High Info Gain:</strong> Novel prompt DNA combinations, untested tradeoffs
            </p>
            <p className="card-content">
              <strong>Low Info Gain:</strong> Well-explored regions of the solution space
            </p>
          </div>
        </div>

        <div className="card mt-3">
          <h3 className="card-title">Children Allocation Example</h3>
          <div className="card-content">
            <p className="mb-2">
              Based on UCB and information gain, GAD allocates breeding resources:
            </p>

            <table className="table">
              <thead>
                <tr>
                  <th>Candidate</th>
                  <th>UCB Score</th>
                  <th>Info Gain</th>
                  <th>Children Allocated</th>
                  <th>Reasoning</th>
                </tr>
              </thead>
              <tbody>
                {selectedCandidates.map((candidate) => (
                  <tr key={candidate.id}>
                    <td><strong>{candidate.id}</strong></td>
                    <td>{candidate.ucb_score?.toFixed(2)}</td>
                    <td>{candidate.expected_info_gain?.toFixed(2)}</td>
                    <td><strong>{candidate.children_allocated}</strong></td>
                    <td style={{ fontSize: '0.85rem' }}>
                      {(candidate.ucb_score || 0) > 0.85 && (candidate.expected_info_gain || 0) > 0.25
                        ? 'High performance + high uncertainty → more children'
                        : (candidate.ucb_score || 0) > 0.85
                        ? 'High performance → moderate allocation'
                        : 'Lower uncertainty → fewer children'}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>

            <div style={{ marginTop: '1rem', padding: '1rem', background: 'var(--color-bg-secondary)', borderRadius: 'var(--radius)' }}>
              <strong>Innovation:</strong> This economic approach treats breeding budget as a
              limited resource and allocates it to maximize expected improvement. High-performing
              candidates with uncertainty get more investment, while well-explored candidates
              get less. This is analogous to portfolio optimization in finance.
            </div>
          </div>
        </div>
      </div>
    </div>
  );
};

export default SelectionPage;
