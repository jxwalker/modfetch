import React from 'react';
import { Link } from 'react-router-dom';
import { DiagramGadLoop } from '../components/DiagramGadLoop';

/**
 * Overview Page
 *
 * Purpose: Give a patent examiner a high-level understanding of GAD in under 3 minutes
 *
 * Key inventions highlighted:
 * - Multi-agent generative + adversarial loop
 * - Composite scoring and hard gates
 * - GEPA-style selection and Pareto front diversity
 * - Prompt DNA evolution
 * - Agent economics using UCB
 * - DNA bundle and RPG
 */
const OverviewPage: React.FC = () => {
  return (
    <div className="page">
      {/* Hero Section */}
      <div className="hero">
        <h1 className="hero-title">
          Generative Adversarial Development (GAD)
        </h1>
        <p className="hero-subtitle">
          An autonomous multi-agent coding loop that evolves AI prompts and code
          until all tests, policies, and user acceptance checks pass
        </p>
        <Link to="/loop" className="btn btn-primary" style={{ marginTop: '1rem' }}>
          Play Demo →
        </Link>
      </div>

      {/* GAD Loop Diagram */}
      <div className="section">
        <h2 className="section-title text-center">The GAD Loop</h2>
        <p className="section-subtitle text-center">
          A complete cycle from requirements to validated, production-ready code
        </p>

        <DiagramGadLoop />

        {/* Stage Explanations */}
        <div className="grid grid-4 mt-4">
          <div className="card">
            <h3 className="card-title">Orchestrator</h3>
            <p className="card-content">
              Central coordinator that manages the evolutionary loop, tracks generations,
              and maintains the Repository Planning Graph (RPG) for architectural coherence.
            </p>
          </div>

          <div className="card">
            <h3 className="card-title">Generator Agents</h3>
            <p className="card-content">
              <strong>Key Innovation:</strong> Multiple specialized LLM agents generate diverse
              candidate solutions with different emphasis (security, performance, UX).
            </p>
          </div>

          <div className="card">
            <h3 className="card-title">CI Pipeline</h3>
            <p className="card-content">
              Automated testing infrastructure runs all tests. Failed tests become
              <strong> hard gates</strong> that disqualify candidates from breeding.
            </p>
          </div>

          <div className="card">
            <h3 className="card-title">Reviewer Agents</h3>
            <p className="card-content">
              <strong>Key Innovation:</strong> Adversarial multi-agent review with
              reliability tracking. Reviews cover security, style, performance, UX, and licensing.
            </p>
          </div>

          <div className="card">
            <h3 className="card-title">UAT Simulator</h3>
            <p className="card-content">
              Simulates user acceptance testing. Feedback is integrated into prompt DNA
              for next generation, closing the loop on user requirements.
            </p>
          </div>

          <div className="card">
            <h3 className="card-title">Scoring Engine</h3>
            <p className="card-content">
              <strong>Key Innovation:</strong> Composite scoring combines test results,
              coverage, security, performance, and UX into weighted effective score with hard gates.
            </p>
          </div>

          <div className="card">
            <h3 className="card-title">Selector (GEPA)</h3>
            <p className="card-content">
              <strong>Key Innovation:</strong> Generalized Evolutionary Pareto Algorithm
              selects diverse survivors from Pareto front, preserving innovation across dimensions.
            </p>
          </div>

          <div className="card">
            <h3 className="card-title">DNA Evolution</h3>
            <p className="card-content">
              <strong>Key Innovation:</strong> Evolution happens at the <em>prompt level</em>.
              Prompt DNA encodes system preamble, exemplars, hyperparameters, and feedback within trust regions.
            </p>
          </div>
        </div>
      </div>

      {/* Key Inventions Summary */}
      <div className="section">
        <h2 className="section-title">Core Innovations</h2>

        <div className="grid grid-2 gap-3">
          <div className="card" style={{ borderLeft: '4px solid var(--color-primary)' }}>
            <h3 className="card-title">1. Multi-Agent Generative + Adversarial Loop</h3>
            <p className="card-content">
              Unlike single-LLM approaches, GAD uses multiple specialized generator agents
              producing diverse solutions, combined with adversarial reviewer agents that
              critique and improve quality. This mimics peer review in human development teams.
            </p>
          </div>

          <div className="card" style={{ borderLeft: '4px solid var(--color-success)' }}>
            <h3 className="card-title">2. Composite Scoring with Hard Gates</h3>
            <p className="card-content">
              Solutions are evaluated across multiple dimensions (tests, coverage, security,
              performance, UX) with configurable weights. Hard gates (e.g., test_pass_rate &gt; 0.70)
              act as binary filters, while soft scores guide selection among viable candidates.
            </p>
          </div>

          <div className="card" style={{ borderLeft: '4px solid var(--color-pareto)' }}>
            <h3 className="card-title">3. GEPA Selection & Pareto Front Diversity</h3>
            <p className="card-content">
              The Generalized Evolutionary Pareto Algorithm (GEPA) selects survivors based on
              Pareto-optimality, preserving diversity. This prevents premature convergence and
              maintains exploration of tradeoff spaces (e.g., security vs. performance).
            </p>
          </div>

          <div className="card" style={{ borderLeft: '4px solid var(--color-warning)' }}>
            <h3 className="card-title">4. Prompt DNA Evolution & Trust Regions</h3>
            <p className="card-content">
              <strong>Core novelty:</strong> Evolution operates on prompt configurations (DNA),
              not just code. DNA bundles encode system preambles, exemplars, hyperparameters,
              and feedback. Trust regions constrain mutations to maintain stability.
            </p>
          </div>

          <div className="card" style={{ borderLeft: '4px solid var(--color-danger)' }}>
            <h3 className="card-title">5. Agent Economics: UCB & Expected Info Gain</h3>
            <p className="card-content">
              Resource allocation uses Upper Confidence Bound (UCB) and Expected Information Gain.
              High-performing candidates with uncertainty get more children in next generation,
              balancing exploitation and exploration.
            </p>
          </div>

          <div className="card" style={{ borderLeft: '4px solid var(--color-secondary)' }}>
            <h3 className="card-title">6. DNA Bundle & Repository Planning Graph (RPG)</h3>
            <p className="card-content">
              DNA bundles provide complete lineage provenance. The RPG tracks architectural
              dependencies (capabilities → modules → files → functions) to maintain long-horizon
              coherence and guide generators toward consistent implementations.
            </p>
          </div>
        </div>
      </div>

      {/* Call to Action */}
      <div className="text-center mt-4">
        <Link to="/loop" className="btn btn-primary">
          Explore the Loop in Action →
        </Link>
      </div>
    </div>
  );
};

export default OverviewPage;
