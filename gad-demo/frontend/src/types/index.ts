/**
 * TypeScript types matching the backend models
 */

export interface PromptDNA {
  id: string;
  system_prompt: string;
  task_description: string;
  constraints: string[];
  examples: string[];
  temperature: number;
  top_p: number;
  feedback_history: string[];
  generation: number;
  parent_ids: string[];
  mutations: Array<{ type: string; change: string }>;
  trust_region_similarity?: number;
}

export interface Metrics {
  test_pass_rate: number;
  coverage: number;
  performance_score: number;
  security_score: number;
  ux_score: number;
  style_score: number;
  license_compliance: boolean;
  vulnerability_count: number;
}

export interface GateResult {
  gate_name: string;
  passed: boolean;
  message: string;
  threshold?: number;
  actual?: number;
}

export interface ReviewerComment {
  reviewer_id: string;
  reviewer_type: string;
  timestamp: string;
  severity: "critical" | "warning" | "info";
  category: string;
  message: string;
  line_numbers?: number[];
}

export interface Candidate {
  id: string;
  generation: number;
  parent_ids: string[];
  prompt_dna_id: string;
  prompt_dna_summary: string;
  metrics: Metrics;
  effective_score: number;
  weighted_scores: Record<string, number>;
  gates_passed: boolean;
  gate_results: GateResult[];
  is_pareto_front: boolean;
  selected_for_breeding: boolean;
  survival_reason?: string;
  branch: string;
  commit_id: string;
  generator_agent_id: string;
  reviewer_comments: ReviewerComment[];
}

export interface AgentProfile {
  id: string;
  name: string;
  type: "generator" | "reviewer";
  specialization: string;
  reliability_score?: number;
  generations_participated: number;
  successful_candidates?: number;
}

export interface UCBStats {
  agent_id: string;
  mean_reward: number;
  confidence_interval: number;
  exploration_bonus: number;
  total_score: number;
  times_selected: number;
}

export interface ParetoPoint {
  candidate_id: string;
  objective1: number;
  objective2: number;
  label: string;
}

export interface Generation {
  number: number;
  candidates: Candidate[];
  pareto_front: ParetoPoint[];
  ucb_allocations: UCBStats[];
  survivors: string[];
  breeding_pairs: Array<[string, string]>;
  summary: string;
}

export interface CodeLayer {
  branch: string;
  commit_id: string;
  diff_summary: string;
  files_changed: number;
  lines_added: number;
  lines_removed: number;
  diff_url?: string;
}

export interface EvaluatorLayer {
  reviewer_reliabilities: Record<string, number>;
  anti_cheat_seed: string;
  ucb_stats: UCBStats[];
  policy_version: string;
  merkle_root: string;
}

export interface DNABundle {
  id: string;
  candidate_id: string;
  code_layer: CodeLayer;
  prompt_layer: PromptDNA;
  evaluator_layer: EvaluatorLayer;
  provenance_hash: string;
  parent_hashes: string[];
  timestamp: string;
}

export interface RPGNode {
  id: string;
  type: "capability" | "module" | "file" | "function" | "test";
  name: string;
  description: string;
  status: "planned" | "in_progress" | "implemented" | "tested";
}

export interface RPGEdge {
  source: string;
  target: string;
  type: "implements" | "calls" | "depends" | "tested_by";
}

export interface RepositoryPlanningGraph {
  nodes: RPGNode[];
  edges: RPGEdge[];
}

export interface GADRun {
  id: string;
  name: string;
  requirement: string;
  total_generations: number;
  generations: Generation[];
  final_candidate_id?: string;
  rpg: RepositoryPlanningGraph;
  agents: AgentProfile[];
}

export interface RunSummary {
  run_id: string;
  name: string;
  total_generations: number;
  total_candidates: number;
  total_survivors: number;
  final_survivor?: {
    id: string;
    effective_score: number;
    gates_passed: boolean;
  };
  requirement: string;
}
