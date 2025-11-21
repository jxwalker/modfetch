import React, { useEffect, useState } from 'react';
import { getDNABundle } from '../api/runs';
import { DNABundle } from '../types';

/**
 * DNA Bundle Page
 *
 * Purpose: Expose the prompt-level innovation and persistence of state across generations
 *
 * Highlights:
 * - Prompt DNA structure and components
 * - Trust regions for controlled mutation
 * - DNA bundle: complete hereditary package
 * - Provenance and lineage tracking
 */
const DnaBundlePage: React.FC = () => {
  const [dnaBundle, setDnaBundle] = useState<DNABundle | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    // Load sample DNA bundle
    getDNABundle('run_001', 'line_c3_1')
      .then(data => {
        setDnaBundle(data);
        setLoading(false);
      })
      .catch(() => setLoading(false));
  }, []);

  if (loading) {
    return <div className="page loading">Loading DNA bundle...</div>;
  }

  if (!dnaBundle) {
    return <div className="page">Error loading DNA bundle</div>;
  }

  return (
    <div className="page">
      <div className="page-header">
        <h1 className="page-title">Prompt DNA & DNA Bundle</h1>
        <p className="page-subtitle">
          Evolution at the prompt level: heritable information encoded in prompt configurations
        </p>
      </div>

      {/* Bundle Overview */}
      <div className="card mb-4" style={{ borderLeft: '4px solid var(--color-primary)' }}>
        <h3 className="card-title">DNA Bundle: {dnaBundle.line_id}</h3>
        <div className="grid grid-2 gap-3 mt-2">
          <div>
            <p><strong>Branch:</strong> <code>{dnaBundle.branch_ref}</code></p>
            <p><strong>Generation:</strong> {dnaBundle.selector_state.generation}</p>
          </div>
          <div>
            <p><strong>Selection Method:</strong> {dnaBundle.selector_state.selection_method}</p>
            <p><strong>Pareto Rank:</strong> {dnaBundle.selector_state.pareto_rank}</p>
          </div>
        </div>
      </div>

      {/* Core Innovation Highlight */}
      <div className="card mb-4" style={{ background: 'linear-gradient(135deg, #667eea 0%, #764ba2 100%)', color: 'white' }}>
        <h3 className="card-title" style={{ color: 'white' }}>
          Core Innovation: Evolution at the Prompt Level
        </h3>
        <p style={{ opacity: 0.95, lineHeight: '1.6' }}>
          Unlike traditional genetic programming that evolves code directly, GAD evolves
          <strong> prompt DNA</strong>—the instructions given to LLMs. This allows hereditary
          information (system preambles, exemplars, hyperparameters, feedback) to be
          encoded, mutated, and bred. The key insight: <em>the generator's instructions
          are the genes, not the code itself.</em>
        </p>
      </div>

      {/* Prompt DNA Structure */}
      <div className="section">
        <h2 className="section-title">Prompt DNA Structure</h2>
        <p className="section-subtitle">
          Complete specification of heritable prompt configuration
        </p>

        <div className="grid grid-2 gap-3">
          <div className="card">
            <h3 className="card-title">System Preamble</h3>
            <p className="card-content mb-2">
              The foundational instruction that shapes the LLM's behavior and expertise level.
            </p>
            <pre style={{
              background: 'var(--color-bg-secondary)',
              padding: '1rem',
              borderRadius: 'var(--radius)',
              fontSize: '0.85rem',
              overflow: 'auto'
            }}>
              {dnaBundle.prompt_dna.system_preamble}
            </pre>
          </div>

          <div className="card">
            <h3 className="card-title">Requirement Frame</h3>
            <p className="card-content mb-2">
              How the user requirement is framed and presented to the generator.
            </p>
            <pre style={{
              background: 'var(--color-bg-secondary)',
              padding: '1rem',
              borderRadius: 'var(--radius)',
              fontSize: '0.85rem',
              overflow: 'auto'
            }}>
              {dnaBundle.prompt_dna.requirement_frame}
            </pre>
          </div>

          <div className="card">
            <h3 className="card-title">Exemplars</h3>
            <p className="card-content mb-2">
              Code examples and patterns that guide the generator's style and approach.
            </p>
            <ul style={{ paddingLeft: '1.5rem', lineHeight: '1.6', fontSize: '0.9rem' }}>
              {dnaBundle.prompt_dna.exemplars.map((exemplar, idx) => (
                <li key={idx}>{exemplar}</li>
              ))}
            </ul>
          </div>

          <div className="card">
            <h3 className="card-title">Tool Flags</h3>
            <p className="card-content mb-2">
              Configuration flags for tools, frameworks, and language features.
            </p>
            <pre style={{
              background: 'var(--color-bg-secondary)',
              padding: '1rem',
              borderRadius: 'var(--radius)',
              fontSize: '0.85rem'
            }}>
              {JSON.stringify(dnaBundle.prompt_dna.tool_flags, null, 2)}
            </pre>
          </div>

          <div className="card">
            <h3 className="card-title">Hyperparameters</h3>
            <p className="card-content mb-2">
              LLM generation parameters like temperature and token limits.
            </p>
            <pre style={{
              background: 'var(--color-bg-secondary)',
              padding: '1rem',
              borderRadius: 'var(--radius)',
              fontSize: '0.85rem'
            }}>
              {JSON.stringify(dnaBundle.prompt_dna.hyperparameters, null, 2)}
            </pre>
          </div>

          <div className="card">
            <h3 className="card-title">Persona Vector</h3>
            <p className="card-content mb-2">
              Multi-dimensional emphasis on different quality attributes (0-1 scale).
            </p>
            <pre style={{
              background: 'var(--color-bg-secondary)',
              padding: '1rem',
              borderRadius: 'var(--radius)',
              fontSize: '0.85rem'
            }}>
              {JSON.stringify(dnaBundle.prompt_dna.persona_vector, null, 2)}
            </pre>
          </div>

          <div className="card">
            <h3 className="card-title">Style Preferences</h3>
            <p className="card-content mb-2">
              Code style and architectural preferences.
            </p>
            <ul style={{ paddingLeft: '1.5rem', lineHeight: '1.6', fontSize: '0.9rem' }}>
              {dnaBundle.prompt_dna.style_preferences.map((pref, idx) => (
                <li key={idx}><code>{pref}</code></li>
              ))}
            </ul>
          </div>

          <div className="card">
            <h3 className="card-title">Policy Digest</h3>
            <p className="card-content mb-2">
              Reference to security and compliance policies the code must satisfy.
            </p>
            <div className="badge badge-info">{dnaBundle.prompt_dna.policy_digest}</div>
          </div>
        </div>
      </div>

      {/* Trust Regions */}
      <div className="section">
        <h2 className="section-title">Trust Regions</h2>

        <div className="card">
          <h3 className="card-title">Constrained Mutation for Stability</h3>
          <div className="card-content">
            <p className="mb-2">
              Trust regions limit how much prompt DNA can mutate in a single generation.
              This prevents drastic changes that might break working solutions while still
              allowing gradual improvement.
            </p>

            {dnaBundle.prompt_dna.trust_region_bounds && (
              <div style={{ marginTop: '1rem' }}>
                <strong>Trust Region Bounds:</strong>
                <pre style={{
                  background: 'var(--color-bg-secondary)',
                  padding: '1rem',
                  borderRadius: 'var(--radius)',
                  fontSize: '0.85rem',
                  marginTop: '0.5rem'
                }}>
                  {JSON.stringify(dnaBundle.prompt_dna.trust_region_bounds, null, 2)}
                </pre>
              </div>
            )}

            <div style={{ marginTop: '1rem', padding: '1rem', background: 'var(--color-bg-secondary)', borderRadius: 'var(--radius)' }}>
              <strong>How it works:</strong> When breeding creates a child prompt DNA, mutations
              are checked against the trust region. If a mutation exceeds the bounds (e.g.,
              changing persona_vector by more than 0.3), the child is projected back into
              the valid region. This balances exploration with stability.
            </div>
          </div>
        </div>
      </div>

      {/* Feedback Integration */}
      <div className="section">
        <h2 className="section-title">Feedback Integration</h2>

        <div className="card">
          <h3 className="card-title">Accumulated Review Feedback</h3>
          <p className="card-content" style={{ whiteSpace: 'pre-wrap' }}>
            {dnaBundle.feedback_summary}
          </p>

          <div style={{ marginTop: '1rem', padding: '1rem', background: 'var(--color-bg-secondary)', borderRadius: 'var(--radius)' }}>
            <strong>Feedback Loop:</strong> Reviewer comments, test failures, and UAT results
            from previous generations are summarized and injected into the prompt DNA for the
            next generation. This closes the loop—generators learn from their mistakes through
            the evolved prompt instructions.
          </div>
        </div>
      </div>

      {/* Complete DNA Bundle */}
      <div className="section">
        <h2 className="section-title">Complete DNA Bundle</h2>
        <p className="section-subtitle">
          Multi-layered hereditary information with full provenance
        </p>

        <div className="grid grid-2 gap-3">
          <div className="card">
            <h3 className="card-title">Evidence Metrics</h3>
            <p className="card-content mb-2">
              Test and evaluation results for this candidate.
            </p>
            <table className="table">
              <tbody>
                <tr>
                  <td>Test Pass Rate</td>
                  <td><strong>{(dnaBundle.evidence_metrics.test_pass_rate * 100).toFixed(0)}%</strong></td>
                </tr>
                <tr>
                  <td>Coverage</td>
                  <td><strong>{(dnaBundle.evidence_metrics.coverage * 100).toFixed(0)}%</strong></td>
                </tr>
                <tr>
                  <td>Security</td>
                  <td><strong>{(dnaBundle.evidence_metrics.security_score * 100).toFixed(0)}%</strong></td>
                </tr>
                <tr>
                  <td>Performance</td>
                  <td><strong>{(dnaBundle.evidence_metrics.performance_score * 100).toFixed(0)}%</strong></td>
                </tr>
              </tbody>
            </table>
          </div>

          <div className="card">
            <h3 className="card-title">Evaluator State</h3>
            <p className="card-content mb-2">
              Snapshot of test execution and benchmarking.
            </p>
            <pre style={{
              background: 'var(--color-bg-secondary)',
              padding: '1rem',
              borderRadius: 'var(--radius)',
              fontSize: '0.85rem'
            }}>
              {JSON.stringify(dnaBundle.evaluator_state, null, 2)}
            </pre>
          </div>

          <div className="card">
            <h3 className="card-title">Policy State</h3>
            <p className="card-content mb-2">
              Compliance status for security and licensing policies.
            </p>
            <pre style={{
              background: 'var(--color-bg-secondary)',
              padding: '1rem',
              borderRadius: 'var(--radius)',
              fontSize: '0.85rem'
            }}>
              {JSON.stringify(dnaBundle.policy_state, null, 2)}
            </pre>
          </div>

          <div className="card">
            <h3 className="card-title">Provenance</h3>
            <p className="card-content mb-2">
              Complete lineage and mutation history.
            </p>
            <pre style={{
              background: 'var(--color-bg-secondary)',
              padding: '1rem',
              borderRadius: 'var(--radius)',
              fontSize: '0.85rem'
            }}>
              {JSON.stringify(dnaBundle.provenance, null, 2)}
            </pre>

            <div style={{ marginTop: '1rem' }}>
              <strong>Lineage:</strong>
              <div style={{ display: 'flex', alignItems: 'center', gap: '0.5rem', marginTop: '0.5rem' }}>
                {dnaBundle.provenance.lineage.map((ancestor: string, idx: number) => (
                  <React.Fragment key={ancestor}>
                    <span className="badge badge-info">{ancestor}</span>
                    {idx < dnaBundle.provenance.lineage.length - 1 && <span>→</span>}
                  </React.Fragment>
                ))}
              </div>
            </div>
          </div>
        </div>
      </div>

      {/* Key Takeaways */}
      <div className="card" style={{ background: 'var(--color-bg-secondary)' }}>
        <h3 className="card-title">Key Innovation: DNA Bundle</h3>
        <div className="card-content">
          <p className="mb-2">
            The DNA bundle is a <strong>complete package of hereditary information</strong> that
            can be versioned, stored, and bred across generations. It contains:
          </p>
          <ul style={{ paddingLeft: '1.5rem', lineHeight: '1.8' }}>
            <li><strong>Code layer:</strong> Not shown here, but includes actual generated code</li>
            <li><strong>Prompt layer:</strong> Prompt DNA configuration</li>
            <li><strong>Evaluator layer:</strong> Test results and metrics</li>
            <li><strong>Feedback layer:</strong> Accumulated reviews and corrections</li>
            <li><strong>Provenance layer:</strong> Full lineage and mutation history</li>
          </ul>
          <p style={{ marginTop: '1rem' }}>
            This multi-layered approach enables <strong>reproducible evolution</strong>—any
            candidate can be recreated, its lineage traced, and its mutations understood.
            This is critical for debugging, auditing, and understanding why certain solutions work.
          </p>
        </div>
      </div>
    </div>
  );
};

export default DnaBundlePage;
