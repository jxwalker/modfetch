import { useQuery } from "@tanstack/react-query";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/Card";
import { Badge } from "@/components/ui/Badge";
import { api } from "@/api/client";
import { formatScore } from "@/lib/utils";
import { User, Shield, Zap, Eye, AlertCircle } from "lucide-react";
import type { AgentProfile, ReviewerComment } from "@/types";

export function AgentsAndScoring() {
  const { data: fullRun } = useQuery({
    queryKey: ["fullRun"],
    queryFn: () => api.getFullRun(),
  });

  if (!fullRun) {
    return <div>Loading...</div>;
  }

  const generators = fullRun.agents.filter((a) => a.type === "generator");
  const reviewers = fullRun.agents.filter((a) => a.type === "reviewer");

  // Get sample comments from the first generation
  const sampleComments = fullRun.generations[0]?.candidates
    .flatMap((c) => c.reviewer_comments)
    .slice(0, 6) || [];

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-3xl font-bold text-gray-900">Agents & Scoring</h1>
        <p className="text-gray-600 mt-2">
          Generator and reviewer agents with composite scoring breakdown
        </p>
      </div>

      {/* Generator Agents */}
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center">
            <User className="h-5 w-5 mr-2" />
            Generator Agents
          </CardTitle>
        </CardHeader>
        <CardContent>
          <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
            {generators.map((agent) => (
              <GeneratorCard key={agent.id} agent={agent} />
            ))}
          </div>
        </CardContent>
      </Card>

      {/* Reviewer Agents */}
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center">
            <Eye className="h-5 w-5 mr-2" />
            Reviewer Agents
          </CardTitle>
        </CardHeader>
        <CardContent>
          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            {reviewers.map((agent) => (
              <ReviewerCard key={agent.id} agent={agent} />
            ))}
          </div>
        </CardContent>
      </Card>

      {/* Sample Comments */}
      <Card>
        <CardHeader>
          <CardTitle>Sample Reviewer Comments</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="space-y-3">
            {sampleComments.map((comment, idx) => (
              <CommentCard key={idx} comment={comment} />
            ))}
          </div>
        </CardContent>
      </Card>

      {/* Scoring System */}
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center">
            <Zap className="h-5 w-5 mr-2" />
            Composite Scoring System
          </CardTitle>
        </CardHeader>
        <CardContent>
          <div className="space-y-6">
            <div>
              <h3 className="font-semibold text-gray-900 mb-3">Weighted Metrics</h3>
              <div className="space-y-2">
                <ScoreWeight metric="Test Pass Rate" weight={25} color="bg-green-500" />
                <ScoreWeight metric="Security Score" weight={25} color="bg-blue-500" />
                <ScoreWeight metric="Performance Score" weight={15} color="bg-yellow-500" />
                <ScoreWeight metric="UX Score" weight={15} color="bg-purple-500" />
                <ScoreWeight metric="Coverage" weight={10} color="bg-indigo-500" />
                <ScoreWeight metric="Style Score" weight={10} color="bg-pink-500" />
              </div>
            </div>

            <div>
              <h3 className="font-semibold text-gray-900 mb-3">Hard Gates</h3>
              <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
                <HardGate
                  name="Minimum Test Pass Rate"
                  threshold="≥ 80%"
                  description="All critical tests must pass"
                />
                <HardGate
                  name="Security Threshold"
                  threshold="≥ 70/100"
                  description="Minimum security score required"
                />
                <HardGate
                  name="Zero Critical Vulnerabilities"
                  threshold="= 0"
                  description="No critical vulnerabilities allowed"
                />
                <HardGate
                  name="License Compliance"
                  threshold="Pass"
                  description="All dependencies must have compatible licenses"
                />
              </div>
            </div>

            <div className="bg-yellow-50 border border-yellow-200 rounded-lg p-4">
              <div className="flex items-start">
                <AlertCircle className="h-5 w-5 text-yellow-600 mt-0.5 mr-3" />
                <div>
                  <h4 className="font-semibold text-yellow-900">Gate Failure Penalty</h4>
                  <p className="text-sm text-yellow-800 mt-1">
                    Candidates that fail any hard gate receive a 50% penalty to their
                    effective score and are typically eliminated from breeding.
                  </p>
                </div>
              </div>
            </div>
          </div>
        </CardContent>
      </Card>
    </div>
  );
}

interface GeneratorCardProps {
  agent: AgentProfile;
}

function GeneratorCard({ agent }: GeneratorCardProps) {
  return (
    <Card>
      <CardContent className="pt-6">
        <div className="flex items-start justify-between mb-3">
          <div>
            <h4 className="font-semibold text-gray-900">{agent.name}</h4>
            <Badge variant="info" className="mt-1">
              Generator
            </Badge>
          </div>
        </div>

        <p className="text-sm text-gray-600 mb-4">{agent.specialization}</p>

        <div className="space-y-2 text-sm">
          <div className="flex justify-between">
            <span className="text-gray-600">Generations:</span>
            <span className="font-medium">{agent.generations_participated}</span>
          </div>
          <div className="flex justify-between">
            <span className="text-gray-600">Successful:</span>
            <span className="font-medium text-green-600">
              {agent.successful_candidates}
            </span>
          </div>
          <div className="flex justify-between">
            <span className="text-gray-600">Success Rate:</span>
            <span className="font-medium">
              {agent.successful_candidates && agent.generations_participated
                ? `${Math.round((agent.successful_candidates / (agent.generations_participated * 2)) * 100)}%`
                : "N/A"}
            </span>
          </div>
        </div>
      </CardContent>
    </Card>
  );
}

interface ReviewerCardProps {
  agent: AgentProfile;
}

function ReviewerCard({ agent }: ReviewerCardProps) {
  return (
    <Card>
      <CardContent className="pt-6">
        <div className="flex items-start justify-between mb-3">
          <div>
            <h4 className="font-semibold text-gray-900">{agent.name}</h4>
            <Badge variant="warning" className="mt-1">
              Reviewer
            </Badge>
          </div>
          {agent.reliability_score && (
            <div className="text-right">
              <div className="text-xl font-bold text-gray-900">
                {formatScore(agent.reliability_score * 100)}
              </div>
              <div className="text-xs text-gray-500">Reliability</div>
            </div>
          )}
        </div>

        <p className="text-sm text-gray-600 mb-4">{agent.specialization}</p>

        <div className="space-y-2 text-sm">
          <div className="flex justify-between">
            <span className="text-gray-600">Generations:</span>
            <span className="font-medium">{agent.generations_participated}</span>
          </div>
          {agent.reliability_score && (
            <div className="mt-2">
              <div className="w-full bg-gray-200 rounded-full h-2">
                <div
                  className="bg-green-500 h-2 rounded-full"
                  style={{ width: `${agent.reliability_score * 100}%` }}
                />
              </div>
            </div>
          )}
        </div>
      </CardContent>
    </Card>
  );
}

interface CommentCardProps {
  comment: ReviewerComment;
}

function CommentCard({ comment }: CommentCardProps) {
  const severityColors = {
    critical: "border-red-500 bg-red-50",
    warning: "border-yellow-500 bg-yellow-50",
    info: "border-blue-500 bg-blue-50",
  };

  return (
    <div
      className={`border-l-4 p-3 rounded ${severityColors[comment.severity]}`}
    >
      <div className="flex items-start justify-between mb-2">
        <div className="flex items-center space-x-2">
          <Badge
            variant={
              comment.severity === "critical"
                ? "error"
                : comment.severity === "warning"
                ? "warning"
                : "info"
            }
          >
            {comment.severity}
          </Badge>
          <Badge variant="default">{comment.category}</Badge>
        </div>
        <span className="text-xs text-gray-500">{comment.reviewer_id}</span>
      </div>
      <p className="text-sm text-gray-800">{comment.message}</p>
      {comment.line_numbers && comment.line_numbers.length > 0 && (
        <p className="text-xs text-gray-600 mt-1">
          Lines: {comment.line_numbers.join(", ")}
        </p>
      )}
    </div>
  );
}

interface ScoreWeightProps {
  metric: string;
  weight: number;
  color: string;
}

function ScoreWeight({ metric, weight, color }: ScoreWeightProps) {
  return (
    <div>
      <div className="flex justify-between text-sm mb-1">
        <span className="text-gray-700">{metric}</span>
        <span className="font-medium">{weight}%</span>
      </div>
      <div className="w-full bg-gray-200 rounded-full h-2">
        <div
          className={`${color} h-2 rounded-full`}
          style={{ width: `${weight}%` }}
        />
      </div>
    </div>
  );
}

interface HardGateProps {
  name: string;
  threshold: string;
  description: string;
}

function HardGate({ name, threshold, description }: HardGateProps) {
  return (
    <div className="border border-gray-200 rounded-lg p-3">
      <div className="flex items-center justify-between mb-2">
        <h4 className="font-semibold text-gray-900 text-sm">{name}</h4>
        <Badge variant="error">{threshold}</Badge>
      </div>
      <p className="text-xs text-gray-600">{description}</p>
    </div>
  );
}
