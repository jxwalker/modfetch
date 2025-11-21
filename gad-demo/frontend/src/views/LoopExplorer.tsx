import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/Card";
import { Button } from "@/components/ui/Button";
import { Badge } from "@/components/ui/Badge";
import { api } from "@/api/client";
import { formatScore, formatPercentage } from "@/lib/utils";
import { ChevronLeft, ChevronRight, Check, X } from "lucide-react";
import type { Candidate } from "@/types";

const STAGES = [
  "Requirement",
  "Prompt DNA",
  "Generator Agents",
  "CI Pipeline",
  "Reviewer Agents",
  "Scoring",
  "Selection",
  "Breeding",
];

export function LoopExplorer() {
  const [currentGen, setCurrentGen] = useState(0);
  const [currentStage, setCurrentStage] = useState(0);

  const { data: fullRun } = useQuery({
    queryKey: ["fullRun"],
    queryFn: () => api.getFullRun(),
  });

  const { data: generation } = useQuery({
    queryKey: ["generation", currentGen],
    queryFn: () => api.getGeneration(currentGen),
    enabled: !!fullRun,
  });

  if (!fullRun || !generation) {
    return <div>Loading...</div>;
  }

  const totalGenerations = fullRun.total_generations;

  return (
    <div className="space-y-6">
      {/* Header */}
      <div>
        <h1 className="text-3xl font-bold text-gray-900">Loop Explorer</h1>
        <p className="text-gray-600 mt-2">
          Step through the GAD pipeline generation by generation
        </p>
      </div>

      {/* Generation Selector */}
      <Card>
        <CardContent className="pt-6">
          <div className="flex items-center justify-between">
            <Button
              variant="outline"
              onClick={() => setCurrentGen(Math.max(0, currentGen - 1))}
              disabled={currentGen === 0}
            >
              <ChevronLeft className="h-4 w-4 mr-1" /> Previous
            </Button>

            <div className="flex items-center space-x-2">
              {Array.from({ length: totalGenerations }, (_, i) => (
                <button
                  key={i}
                  onClick={() => setCurrentGen(i)}
                  className={`w-10 h-10 rounded-full font-semibold transition-colors ${
                    i === currentGen
                      ? "bg-blue-600 text-white"
                      : "bg-gray-200 text-gray-700 hover:bg-gray-300"
                  }`}
                >
                  {i}
                </button>
              ))}
            </div>

            <Button
              variant="outline"
              onClick={() => setCurrentGen(Math.min(totalGenerations - 1, currentGen + 1))}
              disabled={currentGen === totalGenerations - 1}
            >
              Next <ChevronRight className="h-4 w-4 ml-1" />
            </Button>
          </div>

          <div className="mt-4 text-center">
            <h2 className="text-xl font-semibold">Generation {currentGen}</h2>
            <p className="text-sm text-gray-600">{generation.summary}</p>
          </div>
        </CardContent>
      </Card>

      {/* Stage Navigator */}
      <Card>
        <CardHeader>
          <CardTitle>Pipeline Stages</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="grid grid-cols-4 gap-4">
            {STAGES.map((stage, idx) => (
              <button
                key={stage}
                onClick={() => setCurrentStage(idx)}
                className={`p-3 rounded-lg border-2 transition-colors ${
                  idx === currentStage
                    ? "border-blue-600 bg-blue-50"
                    : "border-gray-200 hover:border-gray-300"
                }`}
              >
                <div className="text-sm font-medium">{stage}</div>
              </button>
            ))}
          </div>

          <div className="mt-6">
            <StageContent
              stage={STAGES[currentStage]}
              generation={generation}
              requirement={fullRun.requirement}
            />
          </div>
        </CardContent>
      </Card>

      {/* Candidates Grid */}
      <Card>
        <CardHeader>
          <CardTitle>Candidates ({generation.candidates.length})</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            {generation.candidates.map((candidate) => (
              <CandidateCard key={candidate.id} candidate={candidate} />
            ))}
          </div>
        </CardContent>
      </Card>
    </div>
  );
}

interface StageContentProps {
  stage: string;
  generation: any;
  requirement: string;
}

function StageContent({ stage, generation, requirement }: StageContentProps) {
  switch (stage) {
    case "Requirement":
      return (
        <div className="bg-purple-50 p-4 rounded-lg">
          <h3 className="font-semibold text-purple-900 mb-2">System Requirement</h3>
          <p className="text-purple-800">{requirement}</p>
        </div>
      );

    case "Prompt DNA":
      return (
        <div className="bg-blue-50 p-4 rounded-lg">
          <h3 className="font-semibold text-blue-900 mb-2">Prompt DNA Evolution</h3>
          <p className="text-blue-800">
            Generation {generation.number}: Prompt DNA refined from{" "}
            {generation.candidates[0]?.parent_ids.length || 0} parent(s) with feedback
            integration and constraint refinement.
          </p>
        </div>
      );

    case "Generator Agents":
      return (
        <div className="bg-green-50 p-4 rounded-lg">
          <h3 className="font-semibold text-green-900 mb-2">Code Generation</h3>
          <p className="text-green-800">
            {generation.candidates.length} candidates generated by specialized agents:
          </p>
          <div className="mt-2 space-y-1">
            {generation.ucb_allocations.map((ucb: any) => (
              <div key={ucb.agent_id} className="text-sm text-green-700">
                • {ucb.agent_id}: Score {formatScore(ucb.total_score)} (selected{" "}
                {ucb.times_selected} times)
              </div>
            ))}
          </div>
        </div>
      );

    case "CI Pipeline":
      return (
        <div className="bg-orange-50 p-4 rounded-lg">
          <h3 className="font-semibold text-orange-900 mb-2">Continuous Integration</h3>
          <div className="space-y-2 text-orange-800">
            <div>✓ Tests executed for all candidates</div>
            <div>✓ Coverage reports generated</div>
            <div>✓ Security scans completed</div>
            <div>✓ License compliance checked</div>
          </div>
        </div>
      );

    case "Reviewer Agents":
      return (
        <div className="bg-red-50 p-4 rounded-lg">
          <h3 className="font-semibold text-red-900 mb-2">Agent Reviews</h3>
          <p className="text-red-800">
            Specialized reviewers analyzing security, performance, UX, and code quality.
          </p>
          <div className="mt-2 text-sm text-red-700">
            Total comments: {generation.candidates.reduce((sum: number, c: any) => sum + c.reviewer_comments.length, 0)}
          </div>
        </div>
      );

    case "Scoring":
      return (
        <div className="bg-yellow-50 p-4 rounded-lg">
          <h3 className="font-semibold text-yellow-900 mb-2">Weighted Scoring</h3>
          <div className="space-y-1 text-yellow-800 text-sm">
            <div>• Test Pass Rate: 25%</div>
            <div>• Security: 25%</div>
            <div>• Performance: 15%</div>
            <div>• UX: 15%</div>
            <div>• Coverage: 10%</div>
            <div>• Style: 10%</div>
          </div>
        </div>
      );

    case "Selection":
      return (
        <div className="bg-indigo-50 p-4 rounded-lg">
          <h3 className="font-semibold text-indigo-900 mb-2">GEPA Selection</h3>
          <p className="text-indigo-800">
            {generation.survivors.length} candidates selected for breeding using
            Pareto-optimal multi-objective selection and UCB allocation.
          </p>
        </div>
      );

    case "Breeding":
      return (
        <div className="bg-pink-50 p-4 rounded-lg">
          <h3 className="font-semibold text-pink-900 mb-2">Prompt DNA Evolution</h3>
          <p className="text-pink-800">
            {generation.breeding_pairs.length} breeding pairs created. Next generation
            will inherit and mutate prompt DNA from survivors.
          </p>
        </div>
      );

    default:
      return null;
  }
}

interface CandidateCardProps {
  candidate: Candidate;
}

function CandidateCard({ candidate }: CandidateCardProps) {
  return (
    <Card className={candidate.selected_for_breeding ? "border-green-500 border-2" : ""}>
      <CardContent className="pt-4">
        <div className="flex items-start justify-between mb-3">
          <div>
            <h4 className="font-semibold text-gray-900">{candidate.id}</h4>
            <div className="flex items-center space-x-2 mt-1">
              {candidate.gates_passed ? (
                <Badge variant="success">
                  <Check className="h-3 w-3 mr-1" />
                  Passed Gates
                </Badge>
              ) : (
                <Badge variant="error">
                  <X className="h-3 w-3 mr-1" />
                  Failed Gates
                </Badge>
              )}
              {candidate.selected_for_breeding && (
                <Badge variant="info">Selected</Badge>
              )}
              {candidate.is_pareto_front && (
                <Badge variant="default">Pareto</Badge>
              )}
            </div>
          </div>
          <div className="text-right">
            <div className="text-2xl font-bold text-gray-900">
              {formatScore(candidate.effective_score)}
            </div>
            <div className="text-xs text-gray-500">Score</div>
          </div>
        </div>

        <div className="space-y-2">
          <MetricBar
            label="Tests"
            value={candidate.metrics.test_pass_rate}
            color="bg-green-500"
          />
          <MetricBar
            label="Security"
            value={candidate.metrics.security_score / 100}
            color="bg-blue-500"
          />
          <MetricBar
            label="Performance"
            value={candidate.metrics.performance_score / 100}
            color="bg-yellow-500"
          />
        </div>

        {!candidate.gates_passed && (
          <div className="mt-3 text-xs text-red-600">
            Failed: {candidate.gate_results.filter((g) => !g.passed).map((g) => g.gate_name).join(", ")}
          </div>
        )}

        {candidate.survival_reason && (
          <div className="mt-3 text-xs text-green-700 font-medium">
            {candidate.survival_reason}
          </div>
        )}
      </CardContent>
    </Card>
  );
}

interface MetricBarProps {
  label: string;
  value: number;
  color: string;
}

function MetricBar({ label, value, color }: MetricBarProps) {
  return (
    <div>
      <div className="flex justify-between text-xs text-gray-600 mb-1">
        <span>{label}</span>
        <span>{formatPercentage(value)}</span>
      </div>
      <div className="w-full bg-gray-200 rounded-full h-2">
        <div
          className={`${color} h-2 rounded-full transition-all`}
          style={{ width: `${value * 100}%` }}
        />
      </div>
    </div>
  );
}
