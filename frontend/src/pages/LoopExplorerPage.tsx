import React, { useEffect, useState } from 'react';
import { getRun } from '../api/runs';
import { RunWithGenerations, Generation } from '../types';
import { CandidateCard } from '../components/CandidateCard';

/**
 * Loop Explorer Page
 *
 * Purpose: Show how a GAD run progresses over multiple generations
 *
 * Features:
 * - Generation timeline selector
 * - Detailed view of candidates in each generation
 * - Visual representation of Pareto front and selection
 * - Step-by-step progression through the loop
 */
const LoopExplorerPage: React.FC = () => {
  const [run, setRun] = useState<RunWithGenerations | null>(null);
  const [selectedGeneration, setSelectedGeneration] = useState<number>(1);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    // Load the sample run
    // In a real implementation, this would allow selecting from multiple runs
    getRun('run_001')
      .then(data => {
        setRun(data);
        setLoading(false);
      })
      .catch(err => {
        setError(err.message);
        setLoading(false);
      });
  }, []);

  if (loading) {
    return <div className="page loading">Loading run data...</div>;
  }

  if (error || !run) {
    return <div className="page">Error: {error || 'Run not found'}</div>;
  }

  const generation = run.generations[selectedGeneration - 1];

  return (
    <div className="page">
      <div className="page-header">
        <h1 className="page-title">Loop Explorer</h1>
        <p className="page-subtitle">
          {run.name}: {run.description}
        </p>
      </div>

      {/* Run Summary */}
      <div className="card mb-4">
        <h3 className="card-title">Requirement</h3>
        <p className="card-content">{run.requirement_summary}</p>
        <div className="mt-2">
          <span className="badge badge-info">
            {run.total_generations} Generations
          </span>
          <span className="badge badge-success ml-2">
            {run.final_status}
          </span>
        </div>
      </div>

      {/* Generation Timeline */}
      <div className="section">
        <h2 className="section-title">Generation Timeline</h2>
        <p className="section-subtitle mb-3">
          Click on a generation to explore its candidates and evolution
        </p>

        <div className="timeline">
          {run.generations.map((gen) => (
            <div
              key={gen.generation_number}
              className={`timeline-item ${selectedGeneration === gen.generation_number ? 'active' : ''}`}
              onClick={() => setSelectedGeneration(gen.generation_number)}
            >
              <div className="timeline-item-title">Gen {gen.generation_number}</div>
              <div className="timeline-item-meta">
                {gen.candidates.length} candidates
              </div>
              <div className="timeline-item-meta">
                Avg: {(gen.avg_score * 100).toFixed(0)}%
              </div>
              <div className="timeline-item-meta">
                Best: {(gen.best_score * 100).toFixed(0)}%
              </div>
            </div>
          ))}
        </div>
      </div>

      {/* Selected Generation Details */}
      <div className="section">
        <div className="flex justify-between items-center mb-3">
          <h2 className="section-title mb-0">
            Generation {generation.generation_number}
          </h2>
          <div className="flex gap-2">
            <button
              className="btn btn-secondary"
              onClick={() => setSelectedGeneration(Math.max(1, selectedGeneration - 1))}
              disabled={selectedGeneration === 1}
            >
              ← Previous
            </button>
            <button
              className="btn btn-secondary"
              onClick={() => setSelectedGeneration(Math.min(run.total_generations, selectedGeneration + 1))}
              disabled={selectedGeneration === run.total_generations}
            >
              Next →
            </button>
          </div>
        </div>

        <p className="section-subtitle">{generation.summary}</p>

        {/* Generation Stats */}
        <div className="grid grid-4 mb-4">
          <div className="card text-center">
            <div style={{ fontSize: '2rem', fontWeight: 'bold', color: 'var(--color-primary)' }}>
              {generation.candidates.length}
            </div>
            <div className="text-secondary">Total Candidates</div>
          </div>

          <div className="card text-center">
            <div style={{ fontSize: '2rem', fontWeight: 'bold', color: 'var(--color-pareto)' }}>
              {generation.pareto_front_count}
            </div>
            <div className="text-secondary">Pareto Front</div>
          </div>

          <div className="card text-center">
            <div style={{ fontSize: '2rem', fontWeight: 'bold', color: 'var(--color-success)' }}>
              {generation.selected_for_breeding_count}
            </div>
            <div className="text-secondary">Selected</div>
          </div>

          <div className="card text-center">
            <div style={{ fontSize: '2rem', fontWeight: 'bold', color: 'var(--color-warning)' }}>
              {(generation.diversity_score * 100).toFixed(0)}%
            </div>
            <div className="text-secondary">Diversity</div>
          </div>
        </div>

        {/* Candidates Grid */}
        <h3 className="section-title">Candidates</h3>
        <div className="grid grid-2 gap-3">
          {generation.candidates.map((candidate) => (
            <CandidateCard key={candidate.id} candidate={candidate} detailed />
          ))}
        </div>
      </div>

      {/* Evolution Insights */}
      {selectedGeneration > 1 && (
        <div className="section">
          <h2 className="section-title">Evolution Insights</h2>
          <div className="card">
            <h3 className="card-title">What Changed?</h3>
            <div className="card-content">
              <ul style={{ paddingLeft: '1.5rem', lineHeight: '1.8' }}>
                <li>
                  <strong>Score Improvement:</strong> Average score increased from{' '}
                  {(run.generations[selectedGeneration - 2].avg_score * 100).toFixed(0)}% to{' '}
                  {(generation.avg_score * 100).toFixed(0)}%
                </li>
                <li>
                  <strong>Pareto Front:</strong> {generation.pareto_front_count} candidates represent
                  optimal tradeoffs across metrics
                </li>
                <li>
                  <strong>Selection:</strong> {generation.selected_for_breeding_count} survivors
                  chosen based on GEPA algorithm for next generation
                </li>
                <li>
                  <strong>Feedback Integration:</strong> Reviewer comments and test failures from
                  previous generation integrated into Prompt DNA
                </li>
              </ul>
            </div>
          </div>
        </div>
      )}

      {/* Final Generation Summary */}
      {selectedGeneration === run.total_generations && (
        <div className="section">
          <div className="card" style={{ borderLeft: '4px solid var(--color-success)' }}>
            <h3 className="card-title">Final Generation Reached</h3>
            <p className="card-content">
              The GAD loop has converged to high-quality solutions. The best candidate
              (score: {(generation.best_score * 100).toFixed(0)}%) represents a production-ready
              implementation that passed all hard gates and achieved strong scores across
              all evaluation dimensions.
            </p>
            <p className="card-content mt-2">
              <strong>Key achievements:</strong> All tests passing, security vulnerabilities
              addressed, performance optimized, and UX requirements satisfied.
            </p>
          </div>
        </div>
      )}
    </div>
  );
};

export default LoopExplorerPage;
