import { useQuery } from "@tanstack/react-query";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/Card";
import { Badge } from "@/components/ui/Badge";
import { api } from "@/api/client";
import { getStatusColor } from "@/lib/utils";
import { Network, FileCode, Package, Layers } from "lucide-react";
import type { RPGNode, RPGEdge } from "@/types";

export function RepositoryPlanningGraph() {
  const { data: rpg } = useQuery({
    queryKey: ["rpg"],
    queryFn: () => api.getRPG(),
  });

  if (!rpg) {
    return <div>Loading...</div>;
  }

  // Group nodes by type
  const nodesByType = rpg.nodes.reduce((acc, node) => {
    if (!acc[node.type]) acc[node.type] = [];
    acc[node.type].push(node);
    return acc;
  }, {} as Record<string, RPGNode[]>);

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-3xl font-bold text-gray-900">
          Repository Planning Graph
        </h1>
        <p className="text-gray-600 mt-2">
          Architectural blueprint showing capabilities, modules, and dependencies
        </p>
      </div>

      {/* RPG Overview */}
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center">
            <Network className="h-5 w-5 mr-2" />
            Graph Overview
          </CardTitle>
        </CardHeader>
        <CardContent>
          <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
            <StatCard label="Nodes" value={rpg.nodes.length} />
            <StatCard label="Edges" value={rpg.edges.length} />
            <StatCard
              label="Capabilities"
              value={nodesByType.capability?.length || 0}
            />
            <StatCard
              label="Tests"
              value={nodesByType.test?.length || 0}
            />
          </div>

          <div className="mt-6 bg-blue-50 border border-blue-200 rounded-lg p-4">
            <h4 className="font-semibold text-blue-900 mb-2">
              What is the RPG?
            </h4>
            <p className="text-sm text-blue-800">
              The Repository Planning Graph is a persistent architectural blueprint
              that tracks capabilities, modules, files, functions, and tests. It
              provides structural context to generator agents and helps maintain
              consistency across generations.
            </p>
          </div>
        </CardContent>
      </Card>

      {/* Graph Visualization */}
      <Card>
        <CardHeader>
          <CardTitle>Graph Visualization</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="bg-gray-50 border-2 border-gray-200 rounded-lg p-8">
            <SVGGraph nodes={rpg.nodes} edges={rpg.edges} />
          </div>
        </CardContent>
      </Card>

      {/* Nodes by Type */}
      <div className="grid grid-cols-1 gap-6">
        {Object.entries(nodesByType).map(([type, nodes]) => (
          <Card key={type}>
            <CardHeader>
              <CardTitle className="flex items-center capitalize">
                <TypeIcon type={type} />
                <span className="ml-2">{type}s ({nodes.length})</span>
              </CardTitle>
            </CardHeader>
            <CardContent>
              <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
                {nodes.map((node) => (
                  <NodeCard key={node.id} node={node} />
                ))}
              </div>
            </CardContent>
          </Card>
        ))}
      </div>

      {/* Edges */}
      <Card>
        <CardHeader>
          <CardTitle>Relationships ({rpg.edges.length})</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="space-y-2">
            {rpg.edges.map((edge, idx) => (
              <EdgeCard key={idx} edge={edge} nodes={rpg.nodes} />
            ))}
          </div>
        </CardContent>
      </Card>
    </div>
  );
}

interface StatCardProps {
  label: string;
  value: number;
}

function StatCard({ label, value }: StatCardProps) {
  return (
    <div className="bg-gray-50 border border-gray-200 rounded-lg p-4">
      <div className="text-2xl font-bold text-gray-900">{value}</div>
      <div className="text-sm text-gray-600">{label}</div>
    </div>
  );
}

interface NodeCardProps {
  node: RPGNode;
}

function NodeCard({ node }: NodeCardProps) {
  return (
    <div className="border border-gray-200 rounded-lg p-3">
      <div className="flex items-start justify-between mb-2">
        <h4 className="font-semibold text-gray-900 text-sm">{node.name}</h4>
        <Badge
          variant={node.status === "tested" || node.status === "implemented" ? "success" : "default"}
          className="text-xs"
        >
          {node.status}
        </Badge>
      </div>
      <p className="text-xs text-gray-600 mb-2">{node.description}</p>
      <code className="text-xs text-gray-500">{node.id}</code>
    </div>
  );
}

interface EdgeCardProps {
  edge: RPGEdge;
  nodes: RPGNode[];
}

function EdgeCard({ edge, nodes }: EdgeCardProps) {
  const sourceNode = nodes.find((n) => n.id === edge.source);
  const targetNode = nodes.find((n) => n.id === edge.target);

  const edgeTypeColors: Record<string, string> = {
    implements: "bg-blue-100 text-blue-800",
    calls: "bg-green-100 text-green-800",
    depends: "bg-yellow-100 text-yellow-800",
    tested_by: "bg-purple-100 text-purple-800",
  };

  return (
    <div className="flex items-center space-x-3 text-sm bg-gray-50 p-2 rounded">
      <span className="font-medium text-gray-700">{sourceNode?.name || edge.source}</span>
      <Badge className={edgeTypeColors[edge.type] || ""}>
        {edge.type}
      </Badge>
      <span className="font-medium text-gray-700">{targetNode?.name || edge.target}</span>
    </div>
  );
}

function TypeIcon({ type }: { type: string }) {
  switch (type) {
    case "capability":
      return <Layers className="h-5 w-5" />;
    case "module":
      return <Package className="h-5 w-5" />;
    case "file":
    case "function":
    case "test":
      return <FileCode className="h-5 w-5" />;
    default:
      return <Network className="h-5 w-5" />;
  }
}

interface SVGGraphProps {
  nodes: RPGNode[];
  edges: RPGEdge[];
}

function SVGGraph({ nodes, edges }: SVGGraphProps) {
  // Simple hierarchical layout
  const nodePositions: Record<string, { x: number; y: number }> = {};
  const typeYPositions: Record<string, number> = {
    capability: 50,
    module: 150,
    file: 250,
    function: 350,
    test: 450,
  };

  nodes.forEach((node, idx) => {
    const y = typeYPositions[node.type] || 300;
    const nodesOfType = nodes.filter((n) => n.type === node.type);
    const indexInType = nodesOfType.indexOf(node);
    const spacing = 600 / (nodesOfType.length + 1);
    const x = spacing * (indexInType + 1) + 50;
    nodePositions[node.id] = { x, y };
  });

  return (
    <svg width="100%" height="500" viewBox="0 0 700 500" className="mx-auto">
      {/* Draw edges */}
      {edges.map((edge, idx) => {
        const source = nodePositions[edge.source];
        const target = nodePositions[edge.target];
        if (!source || !target) return null;

        return (
          <g key={idx}>
            <line
              x1={source.x}
              y1={source.y}
              x2={target.x}
              y2={target.y}
              stroke="#9ca3af"
              strokeWidth="2"
              markerEnd="url(#arrowhead)"
            />
          </g>
        );
      })}

      {/* Draw nodes */}
      {nodes.map((node) => {
        const pos = nodePositions[node.id];
        if (!pos) return null;

        const typeColors: Record<string, string> = {
          capability: "#dbeafe",
          module: "#fef3c7",
          file: "#e0e7ff",
          function: "#fce7f3",
          test: "#d1fae5",
        };

        return (
          <g key={node.id}>
            <rect
              x={pos.x - 40}
              y={pos.y - 15}
              width="80"
              height="30"
              fill={typeColors[node.type] || "#f3f4f6"}
              stroke="#6b7280"
              strokeWidth="2"
              rx="5"
            />
            <text
              x={pos.x}
              y={pos.y + 5}
              textAnchor="middle"
              fontSize="10"
              fill="#374151"
              fontWeight="600"
            >
              {node.name.length > 10 ? node.name.slice(0, 10) + "..." : node.name}
            </text>
          </g>
        );
      })}

      {/* Arrow marker definition */}
      <defs>
        <marker
          id="arrowhead"
          markerWidth="10"
          markerHeight="10"
          refX="9"
          refY="3"
          orient="auto"
        >
          <polygon points="0 0, 10 3, 0 6" fill="#9ca3af" />
        </marker>
      </defs>
    </svg>
  );
}
