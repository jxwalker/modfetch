import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/Card";
import { Badge } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { api } from "@/api/client";
import { formatScore } from "@/lib/utils";
import { ScatterChart, Scatter, XAxis, YAxis, CartesianGrid, Tooltip, Legend, ResponsiveContainer } from "recharts";
import { TrendingUp, Target, Zap } from "lucide-react";
import type { ParetoPoint, UCBStats } from "@/types";

export function SelectionAndGEPA() {
  const [selectedGen, setSelectedGen] = useState(2);

  const { data: fullRun } = useQuery({
    queryKey: ["fullRun"],
    queryFn: () => api.getFullRun(),
  });

  const { data: generation } = useQuery({
    queryKey: ["generation", selectedGen],
    queryFn: () => api.getGeneration(selectedGen),
    enabled: !!fullRun,
  });

  if (!fullRun || !generation) {
    return <div>Loading...</div>;
  }

  const survivors = generation.candidates.filter((c) => c.selected_for_breeding);

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-3xl font-bold text-gray-900">Selection & GEPA</h1>
        <p className="text-gray-600 mt-2">
          Pareto-optimal selection with Generator-Evaluator Planning Allocation
        </p>
      </div>

      {/* Generation Selector */}
      <Card>
        <CardContent className="pt-6">
          <div className="flex items-center space-x-2">
            <span className="text-sm font-medium text-gray-700">Select Generation:</span>
            {Array.from({ length: fullRun.total_generations }, (_, i) => (
              <Button
                key={i}
                variant={i === selectedGen ? "primary" : "outline"}
                size="sm"
                onClick={() => setSelectedGen(i)}
              >
                Gen {i}
              </Button>
            ))}
          </div>
        </CardContent>
      </Card>

      {/* Pareto Front Visualization */}
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center">
            <TrendingUp className="h-5 w-5 mr-2" />
            Pareto Front: Security vs. Performance
          </CardTitle>
        </CardHeader>
        <CardContent>
          <div className="h-96">
            <ResponsiveContainer width="100%" height="100%">
              <ScatterChart
                margin={{ top: 20, right: 20, bottom: 20, left: 20 }}
              >
                <CartesianGrid strokeDasharray="3 3" />
                <XAxis
                  type="number"
                  dataKey="objective1"
                  name="Security Score"
                  domain={[0, 100]}
                  label={{ value: "Security Score", position: "insideBottom", offset: -10 }}
                />
                <YAxis
                  type="number"
                  dataKey="objective2"
                  name="Performance Score"
                  domain={[0, 100]}
                  label={{ value: "Performance Score", angle: -90, position: "insideLeft" }}
                />
                <Tooltip
                  cursor={{ strokeDasharray: "3 3" }}
                  content={<ParetoTooltip />}
                />
                <Legend />
                <Scatter
                  name="Selected"
                  data={generation.pareto_front.filter((p) =>
                    survivors.some((s) => s.id === p.candidate_id)
                  )}
                  fill="#10b981"
                  shape="star"
                  line={false}
                />
                <Scatter
                  name="Not Selected"
                  data={generation.pareto_front.filter((p) =>
                    !survivors.some((s) => s.id === p.candidate_id)
                  )}
                  fill="#6b7280"
                  shape="circle"
                  line={false}
                />
              </ScatterChart>
            </ResponsiveContainer>
          </div>

          <div className="mt-4 bg-blue-50 border border-blue-200 rounded-lg p-4">
            <h4 className="font-semibold text-blue-900 mb-2">About Pareto Optimality</h4>
            <p className="text-sm text-blue-800">
              Pareto-optimal candidates cannot be improved in one objective without
              degrading another. GAD uses multi-objective selection to maintain diversity
              and avoid local optima by preserving candidates with different trade-offs.
            </p>
          </div>
        </CardContent>
      </Card>

      {/* Survivors List */}
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center">
            <Target className="h-5 w-5 mr-2" />
            Selected Survivors ({survivors.length})
          </CardTitle>
        </CardHeader>
        <CardContent>
          <div className="space-y-3">
            {survivors.map((candidate) => (
              <div
                key={candidate.id}
                className="border border-green-200 bg-green-50 rounded-lg p-4"
              >
                <div className="flex items-center justify-between mb-2">
                  <div className="flex items-center space-x-3">
                    <h4 className="font-semibold text-gray-900">{candidate.id}</h4>
                    <Badge variant="success">Selected</Badge>
                    {candidate.is_pareto_front && (
                      <Badge variant="info">Pareto Front</Badge>
                    )}
                  </div>
                  <div className="text-xl font-bold text-gray-900">
                    {formatScore(candidate.effective_score)}
                  </div>
                </div>

                <div className="grid grid-cols-3 gap-4 text-sm">
                  <div>
                    <span className="text-gray-600">Security: </span>
                    <span className="font-medium">
                      {formatScore(candidate.metrics.security_score)}
                    </span>
                  </div>
                  <div>
                    <span className="text-gray-600">Performance: </span>
                    <span className="font-medium">
                      {formatScore(candidate.metrics.performance_score)}
                    </span>
                  </div>
                  <div>
                    <span className="text-gray-600">UX: </span>
                    <span className="font-medium">
                      {formatScore(candidate.metrics.ux_score)}
                    </span>
                  </div>
                </div>

                {candidate.survival_reason && (
                  <div className="mt-2 text-sm text-green-700">
                    Reason: {candidate.survival_reason}
                  </div>
                )}
              </div>
            ))}
          </div>
        </CardContent>
      </Card>

      {/* UCB Allocation */}
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center">
            <Zap className="h-5 w-5 mr-2" />
            UCB-Based Agent Allocation (GEPA)
          </CardTitle>
        </CardHeader>
        <CardContent>
          <div className="space-y-6">
            <div>
              <p className="text-sm text-gray-600 mb-4">
                Generator-Evaluator Planning Allocation (GEPA) uses Upper Confidence Bound
                (UCB) to balance exploitation of high-performing agents with exploration of
                underutilized agents.
              </p>

              <div className="space-y-3">
                {generation.ucb_allocations.map((ucb) => (
                  <UCBCard key={ucb.agent_id} ucb={ucb} />
                ))}
              </div>
            </div>

            <div className="bg-purple-50 border border-purple-200 rounded-lg p-4">
              <h4 className="font-semibold text-purple-900 mb-2">
                Expected Information Gain (EIG)
              </h4>
              <p className="text-sm text-purple-800">
                GAD also uses Expected Information Gain to identify "probing actions" -
                experiments that maximize learning about the solution space. This helps
                discover novel approaches and prevents premature convergence.
              </p>
            </div>

            <div>
              <h4 className="font-semibold text-gray-900 mb-3">UCB Formula</h4>
              <div className="bg-gray-50 border border-gray-200 rounded-lg p-4 font-mono text-sm">
                UCB(agent) = mean_reward + exploration_bonus
                <br />
                exploration_bonus = sqrt(2 * ln(total_selections) / agent_selections)
              </div>
            </div>
          </div>
        </CardContent>
      </Card>
    </div>
  );
}

function ParetoTooltip({ active, payload }: any) {
  if (active && payload && payload.length) {
    const data = payload[0].payload as ParetoPoint;
    return (
      <div className="bg-white border border-gray-300 rounded-lg p-3 shadow-lg">
        <p className="font-semibold text-gray-900">{data.label}</p>
        <p className="text-sm text-gray-600">
          Security: {formatScore(data.objective1)}
        </p>
        <p className="text-sm text-gray-600">
          Performance: {formatScore(data.objective2)}
        </p>
      </div>
    );
  }
  return null;
}

interface UCBCardProps {
  ucb: UCBStats;
}

function UCBCard({ ucb }: UCBCardProps) {
  return (
    <div className="border border-gray-200 rounded-lg p-4">
      <div className="flex items-center justify-between mb-3">
        <h4 className="font-semibold text-gray-900">{ucb.agent_id}</h4>
        <div className="text-xl font-bold text-blue-600">
          {formatScore(ucb.total_score)}
        </div>
      </div>

      <div className="grid grid-cols-2 gap-3 text-sm">
        <div>
          <span className="text-gray-600">Mean Reward:</span>
          <div className="font-medium">{formatScore(ucb.mean_reward)}</div>
        </div>
        <div>
          <span className="text-gray-600">Exploration Bonus:</span>
          <div className="font-medium">{formatScore(ucb.exploration_bonus)}</div>
        </div>
        <div>
          <span className="text-gray-600">Confidence Interval:</span>
          <div className="font-medium">±{formatScore(ucb.confidence_interval)}</div>
        </div>
        <div>
          <span className="text-gray-600">Times Selected:</span>
          <div className="font-medium">{ucb.times_selected}</div>
        </div>
      </div>

      <div className="mt-3">
        <div className="w-full bg-gray-200 rounded-full h-2">
          <div
            className="bg-blue-500 h-2 rounded-full transition-all"
            style={{ width: `${ucb.total_score * 100}%` }}
          />
        </div>
      </div>
    </div>
  );
}
