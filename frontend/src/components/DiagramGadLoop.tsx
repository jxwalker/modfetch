import React from 'react';

interface DiagramGadLoopProps {
  activeStage?: string | null;
}

/**
 * Visual diagram of the GAD loop
 * Shows the full cycle from requirements through selection
 *
 * In a real implementation, this could be:
 * - An interactive SVG with clickable nodes
 * - A D3.js visualization
 * - An animated flow diagram
 */
export const DiagramGadLoop: React.FC<DiagramGadLoopProps> = ({ activeStage = null }) => {
  const stages = [
    { id: 'orchestrator', name: 'Orchestrator', description: 'Central coordinator' },
    { id: 'generators', name: 'Generator Agents', description: 'Multi-agent code generation' },
    { id: 'ci', name: 'CI Pipeline', description: 'Automated testing' },
    { id: 'reviewers', name: 'Reviewer Agents', description: 'Adversarial review' },
    { id: 'uat', name: 'UAT Simulator', description: 'User acceptance' },
    { id: 'scoring', name: 'Scoring Engine', description: 'Composite metrics' },
    { id: 'selector', name: 'Selector', description: 'GEPA + Pareto front' },
    { id: 'breeding', name: 'DNA Evolution', description: 'Prompt DNA breeding' },
  ];

  return (
    <div className="diagram">
      <svg width="100%" height="500" viewBox="0 0 800 500">
        {/* Central Orchestrator */}
        <g>
          <circle
            cx="400"
            cy="250"
            r="60"
            fill={activeStage === 'orchestrator' ? 'var(--color-primary)' : 'var(--color-bg-secondary)'}
            stroke="var(--color-primary)"
            strokeWidth="3"
          />
          <text x="400" y="245" textAnchor="middle" fontWeight="bold" fontSize="16" fill="var(--color-text)">
            Orchestrator
          </text>
          <text x="400" y="265" textAnchor="middle" fontSize="12" fill="var(--color-text-secondary)">
            GAD Loop
          </text>
        </g>

        {/* Generators (top left) */}
        <g>
          <rect
            x="50"
            y="50"
            width="140"
            height="80"
            rx="8"
            fill={activeStage === 'generators' ? 'var(--color-primary)' : 'var(--color-bg)'}
            stroke="var(--color-primary)"
            strokeWidth="2"
          />
          <text x="120" y="80" textAnchor="middle" fontWeight="bold" fontSize="14" fill="var(--color-text)">
            Generators
          </text>
          <text x="120" y="100" textAnchor="middle" fontSize="11" fill="var(--color-text-secondary)">
            Multi-agent
          </text>
          <text x="120" y="115" textAnchor="middle" fontSize="11" fill="var(--color-text-secondary)">
            code generation
          </text>
        </g>

        {/* CI Pipeline (top right) */}
        <g>
          <rect
            x="610"
            y="50"
            width="140"
            height="80"
            rx="8"
            fill={activeStage === 'ci' ? 'var(--color-primary)' : 'var(--color-bg)'}
            stroke="var(--color-success)"
            strokeWidth="2"
          />
          <text x="680" y="80" textAnchor="middle" fontWeight="bold" fontSize="14" fill="var(--color-text)">
            CI Pipeline
          </text>
          <text x="680" y="100" textAnchor="middle" fontSize="11" fill="var(--color-text-secondary)">
            Automated
          </text>
          <text x="680" y="115" textAnchor="middle" fontSize="11" fill="var(--color-text-secondary)">
            testing
          </text>
        </g>

        {/* Reviewers (right) */}
        <g>
          <rect
            x="610"
            y="210"
            width="140"
            height="80"
            rx="8"
            fill={activeStage === 'reviewers' ? 'var(--color-primary)' : 'var(--color-bg)'}
            stroke="var(--color-danger)"
            strokeWidth="2"
          />
          <text x="680" y="240" textAnchor="middle" fontWeight="bold" fontSize="14" fill="var(--color-text)">
            Reviewers
          </text>
          <text x="680" y="260" textAnchor="middle" fontSize="11" fill="var(--color-text-secondary)">
            Adversarial
          </text>
          <text x="680" y="275" textAnchor="middle" fontSize="11" fill="var(--color-text-secondary)">
            review agents
          </text>
        </g>

        {/* Scoring (bottom right) */}
        <g>
          <rect
            x="610"
            y="370"
            width="140"
            height="80"
            rx="8"
            fill={activeStage === 'scoring' ? 'var(--color-primary)' : 'var(--color-bg)'}
            stroke="var(--color-warning)"
            strokeWidth="2"
          />
          <text x="680" y="400" textAnchor="middle" fontWeight="bold" fontSize="14" fill="var(--color-text)">
            Scoring
          </text>
          <text x="680" y="420" textAnchor="middle" fontSize="11" fill="var(--color-text-secondary)">
            Composite
          </text>
          <text x="680" y="435" textAnchor="middle" fontSize="11" fill="var(--color-text-secondary)">
            metrics
          </text>
        </g>

        {/* Selector (bottom) */}
        <g>
          <rect
            x="330"
            y="370"
            width="140"
            height="80"
            rx="8"
            fill={activeStage === 'selector' ? 'var(--color-primary)' : 'var(--color-bg)'}
            stroke="var(--color-pareto)"
            strokeWidth="2"
          />
          <text x="400" y="400" textAnchor="middle" fontWeight="bold" fontSize="14" fill="var(--color-text)">
            Selector
          </text>
          <text x="400" y="420" textAnchor="middle" fontSize="11" fill="var(--color-text-secondary)">
            GEPA + Pareto
          </text>
          <text x="400" y="435" textAnchor="middle" fontSize="11" fill="var(--color-text-secondary)">
            front
          </text>
        </g>

        {/* DNA Evolution (left) */}
        <g>
          <rect
            x="50"
            y="210"
            width="140"
            height="80"
            rx="8"
            fill={activeStage === 'breeding' ? 'var(--color-primary)' : 'var(--color-bg)'}
            stroke="var(--color-pareto)"
            strokeWidth="2"
          />
          <text x="120" y="240" textAnchor="middle" fontWeight="bold" fontSize="14" fill="var(--color-text)">
            DNA Evolution
          </text>
          <text x="120" y="260" textAnchor="middle" fontSize="11" fill="var(--color-text-secondary)">
            Prompt DNA
          </text>
          <text x="120" y="275" textAnchor="middle" fontSize="11" fill="var(--color-text-secondary)">
            breeding
          </text>
        </g>

        {/* Arrows showing flow */}
        <defs>
          <marker id="arrowhead" markerWidth="10" markerHeight="7" refX="9" refY="3.5" orient="auto">
            <polygon points="0 0, 10 3.5, 0 7" fill="var(--color-primary)" />
          </marker>
        </defs>

        {/* Orchestrator to Generators */}
        <line x1="360" y1="210" x2="180" y2="120" stroke="var(--color-primary)" strokeWidth="2" markerEnd="url(#arrowhead)" />

        {/* Generators to CI */}
        <line x1="190" y1="90" x2="610" y2="90" stroke="var(--color-primary)" strokeWidth="2" markerEnd="url(#arrowhead)" />

        {/* CI to Reviewers */}
        <line x1="680" y1="130" x2="680" y2="210" stroke="var(--color-primary)" strokeWidth="2" markerEnd="url(#arrowhead)" />

        {/* Reviewers to Scoring */}
        <line x1="680" y1="290" x2="680" y2="370" stroke="var(--color-primary)" strokeWidth="2" markerEnd="url(#arrowhead)" />

        {/* Scoring to Selector */}
        <line x1="610" y1="410" x2="470" y2="410" stroke="var(--color-primary)" strokeWidth="2" markerEnd="url(#arrowhead)" />

        {/* Selector to DNA Evolution */}
        <line x1="330" y1="400" x2="190" y2="280" stroke="var(--color-primary)" strokeWidth="2" markerEnd="url(#arrowhead)" />

        {/* DNA Evolution back to Orchestrator */}
        <line x1="190" y1="250" x2="350" y2="250" stroke="var(--color-primary)" strokeWidth="2" markerEnd="url(#arrowhead)" />
      </svg>
    </div>
  );
};
