import React, { useEffect, useState } from 'react';
import { getRun } from '../api/runs';
import { RunWithGenerations, Candidate } from '../types';
import { MetricBar } from '../components/MetricBar';

/**
 * Agents and Scoring Page
 *
 * Purpose: Make the multi-agent and scoring aspects concrete
 *
 * Highlights:
 * - Generator agent specializations
 * - Reviewer agent types and reliability
 * - Composite scoring mechanism
 * - Hard gates vs. soft scores
 */
const AgentsPage: React.FC = () => {
  const [run, setRun] = useState<RunWithGenerations | null>(null);
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
    return <div className="page loading">Loading agent data...</div>;
  }

  if (!run) {
    return <div className="page">Error loading data</div>;
  }

  // Get a sample candidate for scoring demonstration
  const sampleCandidate: Candidate = run.generations[2].candidates[0];

  // Define metric weights for composite scoring
  const metricWeights = [
    { name: 'Test Pass Rate', weight: 0.25, key: 'test_pass_rate' },
    { name: 'Coverage', weight: 0.15, key: 'coverage' },
    { name: 'Security', weight: 0.20, key: 'security_score' },
    { name: 'Performance', weight: 0.15, key: 'performance_score' },
    { name: 'UX', weight: 0.15, key: 'ux_score' },
    { name: 'Functionality', weight: 0.10, key: 'functionality_score' },
  ];

  return (
    <div className="page">
      <div className="page-header">
        <h1 className="page-title">Agents & Scoring</h1>
        <p className="page-subtitle">
          Multi-agent architecture with composite evaluation and hard gates
        </p>
      </div>

      {/* Generator Agents */}
      <div className="section">
        <h2 className="section-title">Generator Agents</h2>
        <p className="section-subtitle">
          Specialized LLM agents that produce diverse candidate solutions with different emphases
        </p>

        <div className="grid grid-3 gap-3">
          {run.generator_agents.map((agent) => (
            <div key={agent.id} className="card" style={{ borderLeft: '4px solid var(--color-primary)' }}>
              <h3 className="card-title">{agent.name}</h3>
              <p className="card-content mb-2">{agent.description}</p>

              <div className="badge badge-info mb-3">{agent.specialization}</div>

              <div style={{ fontSize: '0.85rem', color: 'var(--color-text-secondary)' }}>
                <strong>Example DNA Config:</strong>
                <pre style={{ marginTop: '0.5rem', fontSize: '0.8rem' }}>
                  {JSON.stringify(agent.example_prompt_dna, null, 2)}
                </pre>
              </div>
            </div>
          ))}
        </div>

        <div className="card mt-3" style={{ background: 'var(--color-bg-secondary)' }}>
          <h4 className="card-title">Innovation: Multi-Agent Generation</h4>
          <p className="card-content">
            Unlike single-LLM systems, GAD employs multiple specialized generator agents,
            each configured with different prompt DNA to emphasize different quality attributes.
            This produces diverse solutions that explore the full space of tradeoffs, similar
            to how different developers bring different perspectives to a problem.
          </p>
        </div>
      </div>

      {/* Reviewer Agents */}
      <div className="section">
        <h2 className="section-title">Reviewer Agents</h2>
        <p className="section-subtitle">
          Adversarial review agents that critique solutions across multiple dimensions
        </p>

        <div className="grid grid-2 gap-3">
          {run.reviewer_agents.map((agent) => (
            <div key={agent.id} className="card">
              <div className="flex justify-between items-center mb-2">
                <h3 className="card-title mb-0">{agent.name}</h3>
                <span className="badge badge-info">{agent.type}</span>
              </div>

              <p className="card-content mb-3">{agent.description}</p>

              <div className="mb-3">
                <div className="flex justify-between mb-1">
                  <span style={{ fontSize: '0.9rem', fontWeight: 600 }}>Reliability Score</span>
                  <span style={{ fontSize: '0.9rem' }}>{(agent.reliability_score * 100).toFixed(0)}%</span>
                </div>
                <div className="metric-bar-track">
                  <div
                    className="metric-bar-fill"
                    style={{ width: `${agent.reliability_score * 100}%` }}
                  />
                </div>
              </div>

              <div style={{ fontSize: '0.85rem' }}>
                <strong>Example Comments:</strong>
                <ul style={{ paddingLeft: '1.5rem', marginTop: '0.5rem', lineHeight: '1.6' }}>
                  {agent.example_comments.map((comment, idx) => (
                    <li key={idx}>{comment}</li>
                  ))}
                </ul>
              </div>
            </div>
          ))}
        </div>

        <div className="card mt-3" style={{ background: 'var(--color-bg-secondary)' }}>
          <h4 className="card-title">Innovation: Adversarial Review with Reliability Tracking</h4>
          <p className="card-content">
            Reviewer agents act as adversaries, critiquing generated solutions similar to code review.
            Each reviewer has a tracked reliability score based on historical accuracy. This creates
            a <strong>generative-adversarial dynamic</strong> where generators try to produce good
            code while reviewers try to find flaws, driving quality improvement.
          </p>
        </div>
      </div>

      {/* Composite Scoring */}
      <div className="section">
        <h2 className="section-title">Composite Scoring Engine</h2>
        <p className="section-subtitle">
          Multi-dimensional evaluation with weighted metrics and hard gates
        </p>

        <div className="grid grid-2 gap-3">
          {/* Metric Weights Table */}
          <div className="card">
            <h3 className="card-title">Metric Weights</h3>
            <table className="table">
              <thead>
                <tr>
                  <th>Metric</th>
                  <th>Weight</th>
                  <th>Purpose</th>
                </tr>
              </thead>
              <tbody>
                <tr>
                  <td>Test Pass Rate</td>
                  <td><strong>25%</strong></td>
                  <td>Functional correctness</td>
                </tr>
                <tr>
                  <td>Coverage</td>
                  <td><strong>15%</strong></td>
                  <td>Test thoroughness</td>
                </tr>
                <tr>
                  <td>Security</td>
                  <td><strong>20%</strong></td>
                  <td>Vulnerability prevention</td>
                </tr>
                <tr>
                  <td>Performance</td>
                  <td><strong>15%</strong></td>
                  <td>Efficiency & speed</td>
                </tr>
                <tr>
                  <td>UX</td>
                  <td><strong>15%</strong></td>
                  <td>User experience</td>
                </tr>
                <tr>
                  <td>Functionality</td>
                  <td><strong>10%</strong></td>
                  <td>Feature completeness</td>
                </tr>
              </tbody>
            </table>
          </div>

          {/* Example Candidate Scoring */}
          <div className="card">
            <h3 className="card-title">Example: {sampleCandidate.id}</h3>
            <p className="card-content mb-3">
              Candidate from Generation 3 demonstrating composite scoring calculation
            </p>

            <MetricBar label="Test Pass Rate (25%)" value={sampleCandidate.metrics.test_pass_rate} />
            <MetricBar label="Coverage (15%)" value={sampleCandidate.metrics.coverage} />
            <MetricBar label="Security (20%)" value={sampleCandidate.metrics.security_score} color="var(--color-success)" />
            <MetricBar label="Performance (15%)" value={sampleCandidate.metrics.performance_score} color="var(--color-warning)" />
            <MetricBar label="UX (15%)" value={sampleCandidate.metrics.ux_score} color="var(--color-pareto)" />
            <MetricBar label="Functionality (10%)" value={sampleCandidate.metrics.functionality_score} />

            <div className="mt-3 p-3" style={{ background: 'var(--color-bg-secondary)', borderRadius: 'var(--radius)' }}>
              <div className="flex justify-between mb-2">
                <strong>Effective Score:</strong>
                <strong style={{ fontSize: '1.25rem', color: 'var(--color-primary)' }}>
                  {(sampleCandidate.effective_score * 100).toFixed(0)}%
                </strong>
              </div>
              <div className="flex justify-between">
                <span>Gates Passed:</span>
                {sampleCandidate.gates_passed ? (
                  <span className="badge badge-success">✓ All gates passed</span>
                ) : (
                  <span className="badge badge-danger">✗ Failed gates</span>
                )}
              </div>
            </div>
          </div>
        </div>

        {/* Hard Gates Explanation */}
        <div className="card mt-3">
          <h3 className="card-title">Hard Gates vs. Soft Scores</h3>
          <div className="card-content">
            <p className="mb-2">
              <strong>Hard Gates:</strong> Binary pass/fail criteria that must be satisfied for
              a candidate to be considered for breeding. Examples:
            </p>
            <ul style={{ paddingLeft: '1.5rem', lineHeight: '1.8', marginBottom: '1rem' }}>
              <li><code>test_pass_rate &gt;= 0.70</code> - At least 70% of tests must pass</li>
              <li><code>security_score &gt;= 0.60</code> - No critical security vulnerabilities</li>
              <li><code>style_compliance &gt;= 0.50</code> - Basic code quality standards</li>
            </ul>

            <p className="mb-2">
              <strong>Soft Scores:</strong> Weighted metrics that determine relative quality among
              candidates that passed all hard gates. Used for selection and breeding allocation.
            </p>

            <p style={{ marginTop: '1rem', padding: '1rem', background: 'var(--color-bg-secondary)', borderRadius: 'var(--radius)' }}>
              <strong>Innovation:</strong> This two-tier system ensures that only viable solutions
              (passing hard gates) enter the breeding pool, while soft scores guide selection of
              the best among viable candidates. This prevents wasting resources on fundamentally
              broken solutions while still exploring diverse tradeoffs.
            </p>
          </div>
        </div>
      </div>
    </div>
  );
};

export default AgentsPage;
