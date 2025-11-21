import React from 'react';
import { Candidate } from '../types';
import { MetricBar } from './MetricBar';

interface CandidateCardProps {
  candidate: Candidate;
  detailed?: boolean;
}

/**
 * Card displaying a candidate solution
 * Shows metrics, gates status, and selection information
 */
export const CandidateCard: React.FC<CandidateCardProps> = ({ candidate, detailed = false }) => {
  return (
    <div className="card">
      <div className="flex justify-between items-center mb-2">
        <h3 className="card-title">{candidate.id}</h3>
        <div className="flex gap-1">
          {candidate.is_pareto_front && (
            <span className="badge badge-pareto">Pareto Front</span>
          )}
          {candidate.selected_for_breeding && (
            <span className="badge badge-success">Selected</span>
          )}
          {!candidate.gates_passed && (
            <span className="badge badge-danger">Failed Gates</span>
          )}
        </div>
      </div>

      <p className="card-content mb-3">{candidate.prompt_dna_summary}</p>

      {candidate.parent_ids.length > 0 && (
        <div className="mb-2">
          <small className="text-secondary">
            Parents: {candidate.parent_ids.join(', ')}
          </small>
        </div>
      )}

      {!candidate.gates_passed && candidate.failed_gates.length > 0 && (
        <div className="mb-3">
          <div className="badge badge-danger">
            {candidate.failed_gates.join(', ')}
          </div>
        </div>
      )}

      {detailed && (
        <>
          <div className="mb-3">
            <MetricBar label="Tests" value={candidate.metrics.test_pass_rate} />
            <MetricBar label="Coverage" value={candidate.metrics.coverage} />
            <MetricBar label="Security" value={candidate.metrics.security_score} color="#10b981" />
            <MetricBar label="Performance" value={candidate.metrics.performance_score} color="#f59e0b" />
            <MetricBar label="UX" value={candidate.metrics.ux_score} color="#8b5cf6" />
            <MetricBar label="Functionality" value={candidate.metrics.functionality_score} />
          </div>

          <div className="flex justify-between text-sm">
            <span>Effective Score: <strong>{candidate.effective_score.toFixed(2)}</strong></span>
            {candidate.ucb_score && (
              <span>UCB: <strong>{candidate.ucb_score.toFixed(2)}</strong></span>
            )}
            {candidate.children_allocated !== undefined && (
              <span>Children: <strong>{candidate.children_allocated}</strong></span>
            )}
          </div>
        </>
      )}
    </div>
  );
};
