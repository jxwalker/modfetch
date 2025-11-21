import React from 'react';
import { Link } from 'react-router-dom';

/**
 * Examiner Script Page
 *
 * Purpose: Provide a guided script for presenters and patent examiners
 *
 * Provides:
 * - Step-by-step demo walkthrough
 * - Key talking points for each section
 * - Innovation highlights to emphasize
 * - Estimated timing for each step
 */
const ExaminerScriptPage: React.FC = () => {
  const steps = [
    {
      number: 1,
      title: 'Overview Introduction',
      duration: '3-4 minutes',
      page: '/',
      actions: [
        'Navigate to Overview page',
        'Read the hero section aloud',
        'Show the GAD loop diagram',
        'Point out each stage: Orchestrator → Generators → CI → Reviewers → Scoring → Selector → DNA Evolution'
      ],
      talkingPoints: [
        'GAD is fundamentally different from single-LLM code generation',
        'Key innovation #1: Multiple specialized generator agents create diverse solutions',
        'Key innovation #2: Adversarial reviewer agents critique like human code review',
        'Key innovation #3: Evolution happens at the PROMPT level, not just code level',
        'The loop runs autonomously until all gates pass and quality converges'
      ],
      emphasize: [
        'Multi-agent architecture (not just one LLM)',
        'Generative-adversarial dynamic (generators vs. reviewers)',
        'Autonomous until success (no human in the loop)'
      ]
    },
    {
      number: 2,
      title: 'Loop Explorer - Generation 1',
      duration: '4-5 minutes',
      page: '/loop',
      actions: [
        'Navigate to Loop Explorer',
        'Read the requirement summary',
        'Click on Generation 1 in the timeline',
        'Show the 4 initial candidates',
        'Point out that 2 failed hard gates',
        'Highlight the 2 survivors selected for breeding'
      ],
      talkingPoints: [
        'Generation 1 is diverse exploration - different generator agents with different emphasis',
        'c1_1: Security-focused but low test coverage → FAILED gate',
        'c1_2: Performance-optimized but security vulnerability → FAILED gate',
        'c1_3: Balanced approach → PASSED all gates, selected for breeding',
        'c1_4: UX-focused → PASSED gates, selected',
        'Hard gates are binary: fail any gate, you\'re disqualified from breeding',
        'Only viable solutions enter the evolutionary pool'
      ],
      emphasize: [
        'Initial diversity from specialized agents',
        'Hard gates enforce minimum quality thresholds',
        'Selection is not just "pick the best" - it\'s Pareto-optimal with diversity'
      ]
    },
    {
      number: 3,
      title: 'Loop Explorer - Generation 2 & 3',
      duration: '3-4 minutes',
      page: '/loop',
      actions: [
        'Click on Generation 2',
        'Show how scores improved from Gen1',
        'Point out 4 survivors on Pareto front',
        'Click on Generation 3',
        'Show continued improvement',
        'Note the presence of a crossover candidate (c3_3) with two parents'
      ],
      talkingPoints: [
        'Generation 2: Children inherit prompt DNA from Gen1 survivors',
        'Feedback from reviewers and failed tests integrated into new prompt DNA',
        'Average score increased from 69% to 77%',
        'Generation 3: Convergence begins - scores now 84%',
        'c3_3 is a crossover: combines DNA from both c2_1 and c2_3',
        'All Gen3 candidates passed gates - quality is high across the board'
      ],
      emphasize: [
        'Feedback integration: failures become learning opportunities',
        'Crossover breeding: combining successful strategies',
        'Progressive improvement: each generation better than the last'
      ]
    },
    {
      number: 4,
      title: 'Agents and Scoring',
      duration: '4-5 minutes',
      page: '/agents',
      actions: [
        'Navigate to Agents & Scoring page',
        'Show the three generator agents and their specializations',
        'Scroll to reviewer agents',
        'Point out reliability scores for each reviewer',
        'Show the composite scoring section',
        'Explain the metric weights table',
        'Show example candidate with all metrics visualized'
      ],
      talkingPoints: [
        'Generator agents have different persona vectors - security-focus vs. performance-focus',
        'This creates natural diversity in the candidate pool',
        'Reviewer agents act as adversaries - they try to find flaws',
        'Each reviewer has tracked reliability - more reliable reviewers have more influence',
        'Composite scoring: 6 metrics with configurable weights',
        'Test pass rate gets 25% weight, security gets 20%, etc.',
        'Hard gates: test_pass_rate ≥ 70%, security_score ≥ 60%',
        'Soft scores guide selection among viable candidates'
      ],
      emphasize: [
        'Multi-agent both on generation AND review sides',
        'Reliability tracking for reviewers (not all reviewers are equal)',
        'Two-tier evaluation: hard gates + soft scores'
      ]
    },
    {
      number: 5,
      title: 'Selection and GEPA',
      duration: '5-6 minutes',
      page: '/selection',
      actions: [
        'Navigate to Selection & GEPA page',
        'Show the Pareto scatter plots',
        'Point out purple dots (Pareto front)',
        'Scroll to the selection decisions table',
        'Show UCB scores and children allocation',
        'Explain the GEPA algorithm section'
      ],
      talkingPoints: [
        'Pareto front: solutions where improving one metric requires sacrificing another',
        'Not all high-scoring candidates are on the Pareto front',
        'GEPA preserves diversity - selects multiple Pareto-optimal solutions',
        'UCB (Upper Confidence Bound) balances exploitation and exploration',
        'High-performing candidates with uncertainty get more children',
        'Expected Information Gain: how much we learn from breeding this candidate',
        'Resource allocation is like portfolio optimization - invest where expected return is high'
      ],
      emphasize: [
        'GEPA is NOT "survival of the fittest" - it\'s "survival of the diverse optimal"',
        'Agent economics: intelligent resource allocation using UCB',
        'This prevents premature convergence to local optima'
      ]
    },
    {
      number: 6,
      title: 'Prompt DNA and DNA Bundle',
      duration: '5-6 minutes',
      page: '/dna',
      actions: [
        'Navigate to DNA Bundle page',
        'Read the innovation highlight (purple box)',
        'Show the Prompt DNA structure',
        'Point out: system preamble, exemplars, persona vector',
        'Scroll to Trust Regions section',
        'Show the feedback integration',
        'Show the complete DNA bundle with provenance'
      ],
      talkingPoints: [
        'CRITICAL INNOVATION: Evolution at the prompt level',
        'The genes are not the code - the genes are the INSTRUCTIONS to the LLM',
        'Prompt DNA contains: system preamble, exemplars, hyperparameters, persona vector',
        'Persona vector: security_focus: 0.8, performance_focus: 0.6, etc.',
        'When breeding, these vectors are mutated and crossed over',
        'Trust regions prevent drastic mutations that break working solutions',
        'Feedback summary shows accumulated learnings from previous generations',
        'DNA bundle includes: prompt layer, code layer, evaluator layer, provenance',
        'Complete reproducibility - any candidate can be recreated from its DNA'
      ],
      emphasize: [
        'Prompt DNA is the core invention - this is what makes evolution work with LLMs',
        'Trust regions provide stability while allowing improvement',
        'DNA bundle enables full traceability and auditability'
      ]
    },
    {
      number: 7,
      title: 'Repository Planning Graph',
      duration: '4-5 minutes',
      page: '/rpg',
      actions: [
        'Navigate to RPG page',
        'Show the graph visualization',
        'Click on different nodes to show details',
        'Point out "touched by generations" field',
        'Scroll to "How RPG Maintains Coherence" section'
      ],
      talkingPoints: [
        'RPG solves the long-horizon coherence problem',
        'As code evolves over many generations, architecture can drift',
        'RPG maintains a graph of: capabilities, modules, files, functions, dependencies',
        'Before generating, RPG provides constraints: "Don\'t break this interface"',
        'After generating, RPG validates: "Did you violate any dependencies?"',
        'Tracks which parts of the codebase were modified in which generations',
        'This is like a human developer\'s mental model, but explicit and queryable'
      ],
      emphasize: [
        'RPG provides architectural memory across generations',
        'Prevents the system from "forgetting" its own structure',
        'Enables GAD to work on large, complex codebases with many dependencies'
      ]
    },
    {
      number: 8,
      title: 'Wrap-up and Q&A',
      duration: '5 minutes',
      page: '/',
      actions: [
        'Return to Overview page',
        'Recap the 6 core innovations',
        'Invite questions'
      ],
      talkingPoints: [
        'Summary of GAD innovations:',
        '1. Multi-agent generative + adversarial loop',
        '2. Composite scoring with hard gates and soft scores',
        '3. GEPA selection preserving Pareto diversity',
        '4. Prompt DNA evolution with trust regions',
        '5. Agent economics: UCB and expected information gain',
        '6. DNA bundle and RPG for coherence and provenance',
        '',
        'Key differentiators from prior art:',
        '- Not single-LLM: multiple specialized agents',
        '- Not just code evolution: prompt evolution',
        '- Not greedy selection: Pareto-optimal diversity',
        '- Not stateless: RPG maintains architectural memory',
        '',
        'Result: Autonomous code generation that maintains quality, diversity, and coherence'
      ],
      emphasize: [
        'GAD is a complete system, not just one technique',
        'Each innovation addresses a specific problem in autonomous code generation',
        'The combination creates an autonomous coding loop that works'
      ]
    }
  ];

  return (
    <div className="page">
      <div className="page-header">
        <h1 className="page-title">Demo Script for Examiners</h1>
        <p className="page-subtitle">
          Guided walkthrough for presenting GAD to patent examiners and technical reviewers
        </p>
      </div>

      {/* Timing Overview */}
      <div className="card mb-4" style={{ background: 'var(--color-bg-secondary)' }}>
        <h3 className="card-title">Total Demo Time: 30-40 minutes</h3>
        <p className="card-content">
          This script provides a comprehensive walkthrough of the GAD system. Adjust timing
          based on audience questions and interest level. The core innovations (steps 4-7)
          are the most important for patent examination.
        </p>
      </div>

      {/* Steps */}
      {steps.map((step) => (
        <div key={step.number} className="section">
          <div className="flex justify-between items-center mb-2">
            <h2 className="section-title mb-0">
              Step {step.number}: {step.title}
            </h2>
            <div className="flex gap-2 items-center">
              <span className="badge badge-info">{step.duration}</span>
              <Link to={step.page} className="btn btn-primary">
                Go to Page →
              </Link>
            </div>
          </div>

          <div className="grid grid-2 gap-3">
            <div className="card">
              <h3 className="card-title">Actions</h3>
              <ol style={{ paddingLeft: '1.5rem', lineHeight: '1.8' }}>
                {step.actions.map((action, idx) => (
                  <li key={idx}>{action}</li>
                ))}
              </ol>
            </div>

            <div className="card">
              <h3 className="card-title">Talking Points</h3>
              <ul style={{ paddingLeft: '1.5rem', lineHeight: '1.8' }}>
                {step.talkingPoints.map((point, idx) => (
                  <li key={idx}>{point}</li>
                ))}
              </ul>
            </div>
          </div>

          <div className="card mt-3" style={{ borderLeft: '4px solid var(--color-primary)' }}>
            <h4 className="card-title">Key Points to Emphasize</h4>
            <ul style={{ paddingLeft: '1.5rem', lineHeight: '1.8' }}>
              {step.emphasize.map((point, idx) => (
                <li key={idx}><strong>{point}</strong></li>
              ))}
            </ul>
          </div>
        </div>
      ))}

      {/* Tips for Examiners */}
      <div className="section">
        <h2 className="section-title">Tips for Patent Examiners</h2>

        <div className="grid grid-2 gap-3">
          <div className="card">
            <h3 className="card-title">Focus Areas for Prior Art Search</h3>
            <ul style={{ paddingLeft: '1.5rem', lineHeight: '1.8' }}>
              <li>Multi-agent code generation systems</li>
              <li>Evolutionary prompt engineering</li>
              <li>Pareto-optimal selection in genetic algorithms</li>
              <li>Code generation with architectural coherence</li>
              <li>Adversarial training in software generation</li>
              <li>Upper confidence bound in resource allocation</li>
            </ul>
          </div>

          <div className="card">
            <h3 className="card-title">Key Differentiators</h3>
            <ul style={{ paddingLeft: '1.5rem', lineHeight: '1.8' }}>
              <li><strong>Not single-LLM:</strong> Multiple specialized agents</li>
              <li><strong>Not code evolution:</strong> Prompt/DNA evolution</li>
              <li><strong>Not greedy:</strong> Pareto diversity preservation</li>
              <li><strong>Not stateless:</strong> RPG architectural memory</li>
              <li><strong>Not human-in-loop:</strong> Fully autonomous</li>
              <li><strong>Not ad-hoc:</strong> Systematic agent economics</li>
            </ul>
          </div>
        </div>
      </div>

      {/* Common Questions */}
      <div className="section">
        <h2 className="section-title">Common Questions & Answers</h2>

        <div className="card mb-3">
          <h3 className="card-title">Q: How is this different from GitHub Copilot or ChatGPT?</h3>
          <p className="card-content">
            <strong>A:</strong> Copilot and ChatGPT are single-LLM, single-shot systems. They
            generate code once based on a prompt, with no evolution, no multi-agent review,
            and no architectural memory. GAD runs an iterative loop with multiple agents,
            evolves prompts across generations, uses adversarial review, and maintains
            architectural coherence via RPG. It's autonomous until success, not one-shot.
          </p>
        </div>

        <div className="card mb-3">
          <h3 className="card-title">Q: Genetic algorithms for code have been tried before. What's new?</h3>
          <p className="card-content">
            <strong>A:</strong> Traditional genetic programming evolves code directly (syntax trees, programs).
            GAD evolves <em>prompts to LLMs</em>. The gene is not the code—it's the instruction.
            This is fundamentally different because LLMs handle syntax and basic correctness,
            while evolution optimizes the high-level strategy encoded in the prompt. Additionally,
            GAD uses Pareto-optimal selection (not fitness-only) and maintains architectural
            memory via RPG.
          </p>
        </div>

        <div className="card mb-3">
          <h3 className="card-title">Q: What about the cost of running multiple LLM agents?</h3>
          <p className="card-content">
            <strong>A:</strong> GAD uses agent economics (UCB, expected information gain) to
            allocate resources efficiently. Not all candidates get equal budget. High-performing
            candidates with uncertainty get more children, while well-explored candidates get
            less. This is built into the system design to manage cost vs. quality tradeoff.
          </p>
        </div>

        <div className="card">
          <h3 className="card-title">Q: Can this work on real, large codebases?</h3>
          <p className="card-content">
            <strong>A:</strong> Yes, that's the purpose of the RPG (Repository Planning Graph).
            It tracks the existing architecture, dependencies, and interfaces. Before generating
            code, RPG provides constraints. After generating, RPG validates no dependencies are
            broken. This allows GAD to maintain coherence even as it evolves code across many
            files and modules over many generations.
          </p>
        </div>
      </div>
    </div>
  );
};

export default ExaminerScriptPage;
