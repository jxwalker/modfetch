import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/Card";
import { Badge } from "@/components/ui/Badge";
import { BookOpen, CheckCircle } from "lucide-react";

export function ExaminerScript() {
  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-3xl font-bold text-gray-900">Examiner Script</h1>
        <p className="text-gray-600 mt-2">
          Step-by-step guide for presenting the GAD demo to patent examiners
        </p>
      </div>

      <Card className="border-blue-500 border-2">
        <CardContent className="pt-6">
          <div className="bg-blue-50 p-4 rounded-lg">
            <h3 className="font-semibold text-blue-900 mb-2">Demo Duration</h3>
            <p className="text-blue-800">
              Estimated time: 15-20 minutes for full walkthrough
            </p>
          </div>
        </CardContent>
      </Card>

      {/* Introduction */}
      <ScriptSection
        step={1}
        title="Introduction (2 minutes)"
        duration="2 min"
      >
        <Narration>
          "Good morning/afternoon. Today I'll demonstrate Generative Adversarial Development,
          or GAD - an autonomous multi-agent system that evolves both code and instructions
          until all requirements are met."
        </Narration>
        <Narration>
          "Unlike traditional CI/CD which tests pre-written code, GAD actively generates,
          evaluates, and evolves solutions through an adversarial loop between generator
          agents and reviewer agents."
        </Narration>
        <Action view="Overview">
          Show the Overview page and point to the hero description
        </Action>
      </ScriptSection>

      {/* Core Concept */}
      <ScriptSection
        step={2}
        title="Core Concept & Pipeline (3 minutes)"
        duration="3 min"
      >
        <Narration>
          "The GAD pipeline consists of eight stages that repeat over multiple generations.
          Let me walk through each stage."
        </Narration>
        <Action view="Overview">
          Scroll to the pipeline diagram and explain each component:
        </Action>
        <ul className="list-disc list-inside space-y-1 text-sm text-gray-700 ml-6">
          <li>Requirements are encoded into "Prompt DNA" - genetic instructions for generators</li>
          <li>Generator agents produce multiple code candidates</li>
          <li>CI pipeline runs tests, builds, and security scans</li>
          <li>Reviewer agents analyze quality, security, performance, and UX</li>
          <li>Scoring engine aggregates metrics with weighted scoring and hard gates</li>
          <li>Selection engine chooses survivors using Pareto optimality</li>
          <li>Breeding engine evolves prompt DNA for the next generation</li>
        </ul>
        <Narration>
          "This creates a feedback loop where each generation improves on the previous one."
        </Narration>
      </ScriptSection>

      {/* Loop Explorer */}
      <ScriptSection
        step={3}
        title="Live Generation Walkthrough (4 minutes)"
        duration="4 min"
      >
        <Action view="Loop Explorer">
          Navigate to Loop Explorer and select Generation 0
        </Action>
        <Narration>
          "Let's observe an actual GAD run implementing a secure authentication API.
          In Generation 0, we start with 8 initial candidates from different generator agents."
        </Narration>
        <Action>
          Step through the pipeline stages using the stage buttons
        </Action>
        <Narration>
          "Notice how candidates fail different gates - some have security issues,
          others fail test coverage requirements. Only 2-3 candidates pass all gates
          and are selected for breeding."
        </Narration>
        <Action>
          Advance to Generation 2 or 3
        </Action>
        <Narration>
          "By Generation 3, you can see quality improvements. More candidates pass gates,
          and effective scores are higher because prompt DNA has incorporated feedback
          from previous failures."
        </Narration>
      </ScriptSection>

      {/* Agents and Scoring */}
      <ScriptSection
        step={4}
        title="Agents & Scoring System (3 minutes)"
        duration="3 min"
      >
        <Action view="Agents & Scoring">
          Navigate to Agents & Scoring page
        </Action>
        <Narration>
          "GAD employs two types of agents: generators and reviewers. Each has
          specific specializations."
        </Narration>
        <Action>
          Point to generator agents and their success rates
        </Action>
        <Narration>
          "Generator agents are tracked for performance. Those producing better
          candidates get allocated more resources in future generations."
        </Narration>
        <Action>
          Point to reviewer agents and reliability scores
        </Action>
        <Narration>
          "Reviewer agents have reliability scores. A security reviewer with 95%
          reliability carries more weight than one with 80%."
        </Narration>
        <Action>
          Scroll to scoring system and hard gates
        </Action>
        <Narration>
          "The scoring system uses weighted metrics, but hard gates are binary:
          fail any gate and the candidate receives a 50% penalty, essentially
          eliminating it from breeding."
        </Narration>
      </ScriptSection>

      {/* Selection & GEPA */}
      <ScriptSection
        step={5}
        title="Selection & GEPA (3 minutes)"
        duration="3 min"
      >
        <Action view="Selection & GEPA">
          Navigate to Selection & GEPA
        </Action>
        <Narration>
          "Selection isn't just 'pick the highest score.' GAD uses Pareto-optimal
          multi-objective selection."
        </Narration>
        <Action>
          Point to the Pareto plot
        </Action>
        <Narration>
          "This plot shows security versus performance. Pareto-optimal candidates
          can't be improved in one dimension without sacrificing another. GAD
          preserves this diversity to avoid local optima."
        </Narration>
        <Action>
          Scroll to UCB allocations
        </Action>
        <Narration>
          "Generator-Evaluator Planning Allocation, or GEPA, uses Upper Confidence
          Bound to balance exploitation and exploration. High-performing agents
          are used more, but underutilized agents get periodic chances to explore
          novel approaches."
        </Narration>
      </ScriptSection>

      {/* Prompt DNA */}
      <ScriptSection
        step={6}
        title="Prompt DNA & Evolution (2 minutes)"
        duration="2 min"
      >
        <Action view="Prompt DNA & Trust Regions">
          Navigate to Prompt DNA page
        </Action>
        <Narration>
          "Prompt DNA is the genetic code for candidate generation. It evolves
          through mutations, crossover, and feedback integration."
        </Narration>
        <Action>
          Show trust region similarity score
        </Action>
        <Narration>
          "Trust regions prevent catastrophic changes. Mutations must stay within
          75% similarity to successful parents. If a mutation is too radical,
          it's projected back to the trust region boundary."
        </Narration>
        <Action>
          Point to mutations and feedback history
        </Action>
        <Narration>
          "You can see exactly how the prompt evolved: feedback from reviewers
          is integrated, constraints are tightened based on vulnerabilities found."
        </Narration>
      </ScriptSection>

      {/* DNA Bundle */}
      <ScriptSection
        step={7}
        title="DNA Bundle & Provenance (2 minutes)"
        duration="2 min"
      >
        <Action view="DNA Bundle">
          Navigate to DNA Bundle Viewer
        </Action>
        <Narration>
          "Every candidate has a DNA Bundle - a complete, immutable record with
          three layers: code, prompt, and evaluator."
        </Narration>
        <Action>
          Switch between layers
        </Action>
        <Narration>
          "The code layer contains the implementation and diffs. The prompt layer
          holds the instructions. The evaluator layer captures reviewer reliability,
          UCB stats, and cryptographic proofs for tamper-evidence."
        </Narration>
        <Narration>
          "This ensures full reproducibility and audit trails."
        </Narration>
      </ScriptSection>

      {/* Closing */}
      <ScriptSection
        step={8}
        title="Closing & Questions (1 minute)"
        duration="1 min"
      >
        <Narration>
          "In summary, GAD is an autonomous, adversarial, multi-agent system that
          evolves solutions iteratively. It combines generative AI, multi-objective
          optimization, trust-region constraints, and provenance tracking."
        </Narration>
        <Narration>
          "The key novelties are: the adversarial generator-reviewer loop, prompt DNA
          evolution within trust regions, Pareto-optimal selection with GEPA allocation,
          and the immutable DNA bundle structure."
        </Narration>
        <Narration>
          "I'm happy to answer any questions about the system."
        </Narration>
      </ScriptSection>

      {/* Quick Reference */}
      <Card>
        <CardHeader>
          <CardTitle>Quick Reference</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="space-y-2 text-sm">
            <RefItem term="GAD" definition="Generative Adversarial Development" />
            <RefItem term="Prompt DNA" definition="Genetic instructions for code generation that evolve over generations" />
            <RefItem term="Hard Gates" definition="Binary pass/fail thresholds (test pass rate, security, vulnerabilities, licenses)" />
            <RefItem term="Pareto Front" definition="Set of solutions where no objective can improve without degrading another" />
            <RefItem term="GEPA" definition="Generator-Evaluator Planning Allocation using UCB for agent resource allocation" />
            <RefItem term="Trust Region" definition="Constraint requiring ≥75% similarity to parent prompt DNA" />
            <RefItem term="DNA Bundle" definition="Immutable three-layer record: code + prompt + evaluator" />
            <RefItem term="RPG" definition="Repository Planning Graph - architectural blueprint" />
          </div>
        </CardContent>
      </Card>
    </div>
  );
}

interface ScriptSectionProps {
  step: number;
  title: string;
  duration: string;
  children: React.ReactNode;
}

function ScriptSection({ step, title, duration, children }: ScriptSectionProps) {
  return (
    <Card>
      <CardHeader>
        <div className="flex items-center justify-between">
          <CardTitle className="flex items-center">
            <div className="w-8 h-8 bg-blue-600 text-white rounded-full flex items-center justify-center font-bold mr-3">
              {step}
            </div>
            {title}
          </CardTitle>
          <Badge variant="info">{duration}</Badge>
        </div>
      </CardHeader>
      <CardContent>
        <div className="space-y-3">{children}</div>
      </CardContent>
    </Card>
  );
}

function Narration({ children }: { children: React.ReactNode }) {
  return (
    <div className="bg-purple-50 border-l-4 border-purple-500 p-3 rounded">
      <div className="flex items-start">
        <BookOpen className="h-5 w-5 text-purple-600 mr-2 flex-shrink-0 mt-0.5" />
        <p className="text-sm text-gray-800 italic">"{children}"</p>
      </div>
    </div>
  );
}

interface ActionProps {
  view?: string;
  children: React.ReactNode;
}

function Action({ view, children }: ActionProps) {
  return (
    <div className="bg-green-50 border-l-4 border-green-500 p-3 rounded">
      <div className="flex items-start">
        <CheckCircle className="h-5 w-5 text-green-600 mr-2 flex-shrink-0 mt-0.5" />
        <div>
          {view && (
            <Badge variant="success" className="mb-1">
              {view}
            </Badge>
          )}
          <p className="text-sm text-gray-800">{children}</p>
        </div>
      </div>
    </div>
  );
}

interface RefItemProps {
  term: string;
  definition: string;
}

function RefItem({ term, definition }: RefItemProps) {
  return (
    <div className="flex">
      <span className="font-semibold text-gray-900 w-32 flex-shrink-0">{term}:</span>
      <span className="text-gray-700">{definition}</span>
    </div>
  );
}
