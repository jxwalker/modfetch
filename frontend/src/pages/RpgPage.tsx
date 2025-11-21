import React, { useEffect, useState } from 'react';
import { getRPG } from '../api/runs';
import { RPG, RPGNode } from '../types';

/**
 * Repository Planning Graph (RPG) Page
 *
 * Purpose: Explain how GAD maintains long-horizon architectural coherence
 *
 * Highlights:
 * - Graph structure of capabilities, modules, files, functions, tests
 * - Dependency tracking across generations
 * - How RPG guides and constrains generators
 */
const RpgPage: React.FC = () => {
  const [rpg, setRpg] = useState<RPG | null>(null);
  const [selectedNode, setSelectedNode] = useState<RPGNode | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    getRPG('run_001')
      .then(data => {
        setRpg(data);
        if (data.nodes.length > 0) {
          setSelectedNode(data.nodes[0]);
        }
        setLoading(false);
      })
      .catch(() => setLoading(false));
  }, []);

  if (loading) {
    return <div className="page loading">Loading RPG data...</div>;
  }

  if (!rpg) {
    return <div className="page">Error loading RPG</div>;
  }

  // Group nodes by type
  const nodesByType = rpg.nodes.reduce((acc, node) => {
    if (!acc[node.type]) acc[node.type] = [];
    acc[node.type].push(node);
    return acc;
  }, {} as Record<string, RPGNode[]>);

  // Get edges for selected node
  const relatedEdges = selectedNode
    ? rpg.edges.filter(e => e.from_node === selectedNode.id || e.to_node === selectedNode.id)
    : [];

  const typeColors: Record<string, string> = {
    capability: '#2563eb',
    module: '#10b981',
    file: '#f59e0b',
    function: '#8b5cf6',
    test: '#ef4444',
  };

  return (
    <div className="page">
      <div className="page-header">
        <h1 className="page-title">Repository Planning Graph (RPG)</h1>
        <p className="page-subtitle">
          Maintaining long-horizon architectural coherence across generations
        </p>
      </div>

      {/* Innovation Highlight */}
      <div className="card mb-4" style={{ borderLeft: '4px solid var(--color-primary)' }}>
        <h3 className="card-title">Innovation: Architectural Memory</h3>
        <p className="card-content">
          The RPG tracks the system's architecture as a graph of dependencies and relationships.
          As GAD evolves code across generations, the RPG ensures generators understand the
          existing structure, avoid breaking dependencies, and maintain coherent design.
          This solves the <strong>long-horizon coherence problem</strong> in autonomous code generation.
        </p>
      </div>

      {/* Graph Visualization */}
      <div className="section">
        <h2 className="section-title">Graph Structure</h2>
        <p className="section-subtitle">
          Visual representation of the password reset feature architecture
        </p>

        {/* Simple SVG graph visualization */}
        <div className="diagram">
          <svg width="100%" height="500" viewBox="0 0 900 500">
            {/* Capability (top) */}
            <g>
              <rect
                x="350"
                y="20"
                width="200"
                height="60"
                rx="8"
                fill={typeColors.capability}
                stroke="white"
                strokeWidth="2"
                cursor="pointer"
                onClick={() => setSelectedNode(nodesByType.capability?.[0])}
              />
              <text x="450" y="50" textAnchor="middle" fill="white" fontWeight="bold">
                Password Reset
              </text>
              <text x="450" y="65" textAnchor="middle" fill="white" fontSize="11">
                Capability
              </text>
            </g>

            {/* Modules (second level) */}
            <g>
              <rect
                x="100"
                y="140"
                width="150"
                height="50"
                rx="6"
                fill={typeColors.module}
                stroke="white"
                strokeWidth="2"
                cursor="pointer"
                onClick={() => setSelectedNode(nodesByType.module?.[0])}
              />
              <text x="175" y="170" textAnchor="middle" fill="white" fontWeight="bold" fontSize="14">
                Auth Module
              </text>

              <rect
                x="650"
                y="140"
                width="150"
                height="50"
                rx="6"
                fill={typeColors.module}
                stroke="white"
                strokeWidth="2"
                cursor="pointer"
                onClick={() => setSelectedNode(nodesByType.module?.[1])}
              />
              <text x="725" y="170" textAnchor="middle" fill="white" fontWeight="bold" fontSize="14">
                Email Module
              </text>
            </g>

            {/* Files (third level) */}
            <g>
              <rect
                x="40"
                y="250"
                width="130"
                height="45"
                rx="6"
                fill={typeColors.file}
                stroke="white"
                strokeWidth="2"
                cursor="pointer"
                onClick={() => setSelectedNode(nodesByType.file?.[0])}
              />
              <text x="105" y="275" textAnchor="middle" fill="white" fontSize="12">
                reset_controller.ts
              </text>

              <rect
                x="190"
                y="250"
                width="130"
                height="45"
                rx="6"
                fill={typeColors.file}
                stroke="white"
                strokeWidth="2"
                cursor="pointer"
                onClick={() => setSelectedNode(nodesByType.file?.[1])}
              />
              <text x="255" y="275" textAnchor="middle" fill="white" fontSize="12">
                token_service.ts
              </text>

              <rect
                x="650"
                y="250"
                width="150"
                height="45"
                rx="6"
                fill={typeColors.file}
                stroke="white"
                strokeWidth="2"
              />
              <text x="725" y="275" textAnchor="middle" fill="white" fontSize="12">
                email service
              </text>
            </g>

            {/* Functions (fourth level) */}
            <g>
              <rect
                x="20"
                y="360"
                width="120"
                height="40"
                rx="4"
                fill={typeColors.function}
                stroke="white"
                strokeWidth="1"
                cursor="pointer"
                onClick={() => setSelectedNode(nodesByType.function?.[0])}
              />
              <text x="80" y="383" textAnchor="middle" fill="white" fontSize="11">
                generateToken
              </text>

              <rect
                x="160"
                y="360"
                width="120"
                height="40"
                rx="4"
                fill={typeColors.function}
                stroke="white"
                strokeWidth="1"
                cursor="pointer"
                onClick={() => setSelectedNode(nodesByType.function?.[1])}
              />
              <text x="220" y="383" textAnchor="middle" fill="white" fontSize="11">
                validateToken
              </text>

              <rect
                x="665"
                y="360"
                width="120"
                height="40"
                rx="4"
                fill={typeColors.function}
                stroke="white"
                strokeWidth="1"
                cursor="pointer"
                onClick={() => setSelectedNode(nodesByType.function?.[2])}
              />
              <text x="725" y="383" textAnchor="middle" fill="white" fontSize="11">
                sendResetEmail
              </text>
            </g>

            {/* Test (bottom right) */}
            <g>
              <rect
                x="600"
                y="450"
                width="150"
                height="40"
                rx="4"
                fill={typeColors.test}
                stroke="white"
                strokeWidth="2"
                cursor="pointer"
                onClick={() => setSelectedNode(nodesByType.test?.[0])}
              />
              <text x="675" y="473" textAnchor="middle" fill="white" fontSize="11">
                Integration Tests
              </text>
            </g>

            {/* Edges */}
            <defs>
              <marker id="arrow" markerWidth="10" markerHeight="10" refX="9" refY="3" orient="auto">
                <polygon points="0 0, 10 3, 0 6" fill="#64748b" />
              </marker>
            </defs>

            {/* Capability to modules */}
            <line x1="390" y1="80" x2="200" y2="140" stroke="#64748b" strokeWidth="2" markerEnd="url(#arrow)" />
            <line x1="510" y1="80" x2="700" y2="140" stroke="#64748b" strokeWidth="2" markerEnd="url(#arrow)" />

            {/* Modules to files */}
            <line x1="150" y1="190" x2="105" y2="250" stroke="#64748b" strokeWidth="2" markerEnd="url(#arrow)" />
            <line x1="200" y1="190" x2="255" y2="250" stroke="#64748b" strokeWidth="2" markerEnd="url(#arrow)" />
            <line x1="725" y1="190" x2="725" y2="250" stroke="#64748b" strokeWidth="2" markerEnd="url(#arrow)" />

            {/* Files to functions */}
            <line x1="105" y1="295" x2="80" y2="360" stroke="#64748b" strokeWidth="1.5" markerEnd="url(#arrow)" />
            <line x1="255" y1="295" x2="220" y2="360" stroke="#64748b" strokeWidth="1.5" markerEnd="url(#arrow)" />
            <line x1="725" y1="295" x2="725" y2="360" stroke="#64748b" strokeWidth="1.5" markerEnd="url(#arrow)" />

            {/* Test to capability */}
            <line x1="675" y1="450" x2="500" y2="80" stroke="#ef4444" strokeWidth="2" strokeDasharray="5,5" />
          </svg>
        </div>

        <div className="flex gap-2 mt-3 justify-center">
          <span className="badge" style={{ background: typeColors.capability, color: 'white' }}>Capability</span>
          <span className="badge" style={{ background: typeColors.module, color: 'white' }}>Module</span>
          <span className="badge" style={{ background: typeColors.file, color: 'white' }}>File</span>
          <span className="badge" style={{ background: typeColors.function, color: 'white' }}>Function</span>
          <span className="badge" style={{ background: typeColors.test, color: 'white' }}>Test</span>
        </div>
      </div>

      {/* Node Details */}
      {selectedNode && (
        <div className="section">
          <h2 className="section-title">Node Details</h2>

          <div className="grid grid-2 gap-3">
            <div className="card">
              <div className="flex justify-between items-center mb-2">
                <h3 className="card-title mb-0">{selectedNode.name}</h3>
                <span className="badge" style={{ background: typeColors[selectedNode.type], color: 'white' }}>
                  {selectedNode.type}
                </span>
              </div>

              <p className="card-content mb-3">{selectedNode.description}</p>

              <table className="table">
                <tbody>
                  <tr>
                    <td><strong>Status</strong></td>
                    <td>
                      <span className={`badge ${selectedNode.implementation_status === 'complete' ? 'badge-success' : 'badge-warning'}`}>
                        {selectedNode.implementation_status}
                      </span>
                    </td>
                  </tr>
                  <tr>
                    <td><strong>Associated Tests</strong></td>
                    <td>{selectedNode.associated_tests.join(', ') || 'None'}</td>
                  </tr>
                  <tr>
                    <td><strong>Touched by Generations</strong></td>
                    <td>
                      {selectedNode.touched_by_generations.map(gen => (
                        <span key={gen} className="badge badge-info" style={{ marginRight: '0.5rem' }}>
                          Gen {gen}
                        </span>
                      ))}
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>

            <div className="card">
              <h3 className="card-title">Relationships</h3>

              {relatedEdges.length > 0 ? (
                <table className="table">
                  <thead>
                    <tr>
                      <th>From</th>
                      <th>Relation</th>
                      <th>To</th>
                    </tr>
                  </thead>
                  <tbody>
                    {relatedEdges.map((edge, idx) => {
                      const fromNode = rpg.nodes.find(n => n.id === edge.from_node);
                      const toNode = rpg.nodes.find(n => n.id === edge.to_node);
                      return (
                        <tr key={idx}>
                          <td>{fromNode?.name || edge.from_node}</td>
                          <td>
                            <span className="badge badge-info">{edge.relation_type}</span>
                          </td>
                          <td>{toNode?.name || edge.to_node}</td>
                        </tr>
                      );
                    })}
                  </tbody>
                </table>
              ) : (
                <p className="card-content">No relationships found for this node.</p>
              )}
            </div>
          </div>
        </div>
      )}

      {/* All Nodes Table */}
      <div className="section">
        <h2 className="section-title">All Nodes</h2>

        <table className="table">
          <thead>
            <tr>
              <th>Name</th>
              <th>Type</th>
              <th>Description</th>
              <th>Status</th>
              <th>Generations</th>
            </tr>
          </thead>
          <tbody>
            {rpg.nodes.map((node) => (
              <tr
                key={node.id}
                onClick={() => setSelectedNode(node)}
                style={{ cursor: 'pointer' }}
              >
                <td><strong>{node.name}</strong></td>
                <td>
                  <span className="badge" style={{ background: typeColors[node.type], color: 'white' }}>
                    {node.type}
                  </span>
                </td>
                <td style={{ fontSize: '0.9rem' }}>{node.description}</td>
                <td>
                  <span className={`badge ${node.implementation_status === 'complete' ? 'badge-success' : 'badge-warning'}`}>
                    {node.implementation_status}
                  </span>
                </td>
                <td>{node.touched_by_generations.join(', ')}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      {/* How RPG Works */}
      <div className="section">
        <h2 className="section-title">How RPG Maintains Coherence</h2>

        <div className="grid grid-2 gap-3">
          <div className="card">
            <h3 className="card-title">1. Parsing & Analysis</h3>
            <p className="card-content">
              At each generation, GAD parses the codebase to extract structure:
              capabilities, modules, files, functions, and their dependencies.
              This creates a live snapshot of the system architecture.
            </p>
          </div>

          <div className="card">
            <h3 className="card-title">2. Constraint Generation</h3>
            <p className="card-content">
              The RPG generates constraints for generators: "Don't break module X",
              "Function Y must maintain interface", "Add tests for capability Z".
              These constraints are injected into prompt DNA.
            </p>
          </div>

          <div className="card">
            <h3 className="card-title">3. Impact Analysis</h3>
            <p className="card-content">
              Before accepting a candidate, GAD checks if changes violate dependencies.
              For example, if a generator modifies a function signature, RPG identifies
              all callers and ensures they're updated or tests fail.
            </p>
          </div>

          <div className="card">
            <h3 className="card-title">4. Evolution Tracking</h3>
            <p className="card-content">
              RPG tracks which nodes were touched by which generations. This helps
              identify stable vs. volatile parts of the codebase and guides future
              mutation strategies.
            </p>
          </div>
        </div>

        <div className="card mt-3" style={{ background: 'var(--color-bg-secondary)' }}>
          <h4 className="card-title">Innovation: Long-Horizon Coherence</h4>
          <p className="card-content">
            Traditional LLM code generation treats each request independently, leading to
            architectural drift over time. The RPG solves this by maintaining a <strong>persistent
            memory of system structure</strong> that guides all generators. As the system evolves
            across generations, the RPG ensures new code fits into the existing architecture
            rather than creating conflicting implementations.
          </p>
          <p className="card-content mt-2">
            This is analogous to how human developers maintain mental models of codebases—the
            RPG externalizes this mental model as an explicit graph that can be queried,
            analyzed, and used to constrain generation.
          </p>
        </div>
      </div>
    </div>
  );
};

export default RpgPage;
