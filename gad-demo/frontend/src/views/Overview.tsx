import { useNavigate } from "react-router-dom";
import { Button } from "@/components/ui/Button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/Card";
import { ArrowRight, Code, TestTube, Shield, Users, GitBranch, Zap } from "lucide-react";

export function Overview() {
  const navigate = useNavigate();

  return (
    <div className="space-y-8">
      {/* Hero Section */}
      <div className="text-center space-y-4">
        <h1 className="text-4xl font-bold text-gray-900">
          Generative Adversarial Development (GAD)
        </h1>
        <p className="text-xl text-gray-600 max-w-3xl mx-auto">
          An autonomous multi-agent coding loop that evolves instructions and code
          until all tests, policies, security checks and user acceptance behaviors pass.
        </p>
        <Button
          size="lg"
          onClick={() => navigate("/loop")}
          className="mt-4"
        >
          Start Demo <ArrowRight className="ml-2 h-5 w-5" />
        </Button>
      </div>

      {/* Pipeline Diagram */}
      <Card>
        <CardHeader>
          <CardTitle>The GAD Pipeline</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="py-8">
            <GADPipelineDiagram />
          </div>
        </CardContent>
      </Card>

      {/* Key Components */}
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
        <ComponentCard
          icon={<Code className="h-6 w-6" />}
          title="Generator Agents"
          description="AI agents that produce code candidates from prompt DNA instructions"
        />
        <ComponentCard
          icon={<TestTube className="h-6 w-6" />}
          title="CI Pipeline"
          description="Automated testing, builds, and compliance checks for each candidate"
        />
        <ComponentCard
          icon={<Shield className="h-6 w-6" />}
          title="Reviewer Agents"
          description="Specialized agents analyzing security, performance, UX, and quality"
        />
        <ComponentCard
          icon={<Users className="h-6 w-6" />}
          title="UAT Simulator"
          description="Real user flow simulation capturing UX metrics"
        />
        <ComponentCard
          icon={<Zap className="h-6 w-6" />}
          title="Scoring & Selection"
          description="Weighted metrics, hard gates, and Pareto-optimal selection"
        />
        <ComponentCard
          icon={<GitBranch className="h-6 w-6" />}
          title="Breeding Engine"
          description="Evolves prompt DNA through crossover, mutation, and feedback"
        />
      </div>

      {/* How It Works */}
      <Card>
        <CardHeader>
          <CardTitle>How GAD Works</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="space-y-4">
            <ProcessStep
              number={1}
              title="Generation"
              description="Generator agents create multiple code candidates from prompt DNA"
            />
            <ProcessStep
              number={2}
              title="Evaluation"
              description="CI pipeline runs tests, builds code, and scans for vulnerabilities"
            />
            <ProcessStep
              number={3}
              title="Review"
              description="Reviewer agents analyze each candidate for quality, security, and UX"
            />
            <ProcessStep
              number={4}
              title="Scoring"
              description="Weighted scoring with hard gates determines candidate viability"
            />
            <ProcessStep
              number={5}
              title="Selection"
              description="Pareto-optimal candidates selected using GEPA (UCB + EIG)"
            />
            <ProcessStep
              number={6}
              title="Breeding"
              description="Survivors' prompt DNA is evolved through mutation and crossover"
            />
            <ProcessStep
              number={7}
              title="Iteration"
              description="Process repeats until a fully compliant solution emerges"
            />
          </div>
        </CardContent>
      </Card>
    </div>
  );
}

function GADPipelineDiagram() {
  return (
    <div className="flex flex-col items-center space-y-6">
      {/* Row 1: Requirement */}
      <DiagramNode label="Requirement" color="bg-purple-100 text-purple-700 border-purple-300" />
      <ArrowDown />

      {/* Row 2: Prompt DNA */}
      <DiagramNode label="Prompt DNA" color="bg-blue-100 text-blue-700 border-blue-300" />
      <ArrowDown />

      {/* Row 3: Generator Agents */}
      <div className="flex items-center space-x-4">
        <DiagramNode label="Generator 1" color="bg-green-100 text-green-700 border-green-300" size="sm" />
        <DiagramNode label="Generator 2" color="bg-green-100 text-green-700 border-green-300" size="sm" />
        <DiagramNode label="Generator 3" color="bg-green-100 text-green-700 border-green-300" size="sm" />
      </div>
      <ArrowDown />

      {/* Row 4: CI + Reviewers */}
      <div className="flex items-center space-x-8">
        <DiagramNode label="CI Pipeline" color="bg-orange-100 text-orange-700 border-orange-300" />
        <DiagramNode label="Reviewer Agents" color="bg-red-100 text-red-700 border-red-300" />
      </div>
      <ArrowDown />

      {/* Row 5: Scoring */}
      <DiagramNode label="Scoring Engine" color="bg-yellow-100 text-yellow-700 border-yellow-300" />
      <ArrowDown />

      {/* Row 6: Selection */}
      <DiagramNode label="Selection (GEPA)" color="bg-indigo-100 text-indigo-700 border-indigo-300" />

      {/* Feedback Loop */}
      <div className="flex items-center space-x-4">
        <ArrowDown />
        <div className="text-sm text-gray-500 italic">Feedback loop →</div>
      </div>

      {/* Row 7: Breeding */}
      <DiagramNode label="Breeding Engine" color="bg-pink-100 text-pink-700 border-pink-300" />
    </div>
  );
}

interface DiagramNodeProps {
  label: string;
  color: string;
  size?: "sm" | "md";
}

function DiagramNode({ label, color, size = "md" }: DiagramNodeProps) {
  const sizeClasses = size === "sm" ? "px-4 py-2 text-sm" : "px-6 py-3 text-base";
  return (
    <div className={`${color} ${sizeClasses} rounded-lg border-2 font-medium shadow-sm`}>
      {label}
    </div>
  );
}

function ArrowDown() {
  return (
    <div className="flex flex-col items-center">
      <div className="w-0.5 h-4 bg-gray-400" />
      <div className="w-0 h-0 border-l-4 border-r-4 border-t-4 border-transparent border-t-gray-400" />
    </div>
  );
}

interface ComponentCardProps {
  icon: React.ReactNode;
  title: string;
  description: string;
}

function ComponentCard({ icon, title, description }: ComponentCardProps) {
  return (
    <Card>
      <CardContent className="pt-6">
        <div className="flex items-start space-x-3">
          <div className="flex-shrink-0 text-blue-600">{icon}</div>
          <div>
            <h3 className="font-semibold text-gray-900 mb-1">{title}</h3>
            <p className="text-sm text-gray-600">{description}</p>
          </div>
        </div>
      </CardContent>
    </Card>
  );
}

interface ProcessStepProps {
  number: number;
  title: string;
  description: string;
}

function ProcessStep({ number, title, description }: ProcessStepProps) {
  return (
    <div className="flex items-start space-x-4">
      <div className="flex-shrink-0 w-8 h-8 bg-blue-600 text-white rounded-full flex items-center justify-center font-semibold">
        {number}
      </div>
      <div className="flex-1">
        <h4 className="font-semibold text-gray-900">{title}</h4>
        <p className="text-sm text-gray-600 mt-1">{description}</p>
      </div>
    </div>
  );
}
