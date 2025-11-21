/**
 * TypeScript types matching the backend models
 */

export interface PromptDNA {
  system_preamble: string;
  requirement_frame: string;
  exemplars: string[];
  tool_flags: Record<string, boolean>;
  hyperparameters: Record<string, any>;
  persona_vector: Record<string, number>;
  style_preferences: string[];
  policy_digest: string;
  trust_region_bounds?: Record<string, number>;
}

export interface Metrics {
  test_pass_rate: number;
  coverage: number;
  security_score: number;
  performance_score: number;
  ux_score: number;
  functionality_score: number;
  style_compliance: number;
}

export interface Candidate {
  id: string;
  parent_ids: string[];
  prompt_dna_summary: string;
  metrics: Metrics;
  gates_passed: boolean;
  failed_gates: string[];
  effective_score: number;
  is_pareto_front: boolean;
  selected_for_breeding: boolean;
  ucb_score?: number;
  expected_info_gain?: number;
  children_allocated?: number;
}

export interface GeneratorAgent {
  id: string;
  name: string;
  description: string;
  specialization: string;
  example_prompt_dna: Record<string, any>;
}

export interface ReviewerAgent {
  id: string;
  name: string;
  type: string;
  description: string;
  reliability_score: number;
  example_comments: string[];
}

export interface Generation {
  generation_number: number;
  summary: string;
  candidates: Candidate[];
  pareto_front_count: number;
  selected_for_breeding_count: number;
  avg_score: number;
  best_score: number;
  diversity_score: number;
}

export interface DNABundle {
  line_id: string;
  branch_ref: string;
  prompt_dna: PromptDNA;
  feedback_summary: string;
  evidence_metrics: Metrics;
  selector_state: Record<string, any>;
  evaluator_state: Record<string, any>;
  policy_state: Record<string, any>;
  provenance: Record<string, any>;
}

export interface RPGNode {
  id: string;
  type: string;
  name: string;
  description: string;
  implementation_status: string;
  associated_tests: string[];
  touched_by_generations: number[];
}

export interface RPGEdge {
  from_node: string;
  to_node: string;
  relation_type: string;
}

export interface RPG {
  nodes: RPGNode[];
  edges: RPGEdge[];
}

export interface Run {
  id: string;
  name: string;
  description: string;
  requirement_summary: string;
  total_generations: number;
  final_status: string;
  generator_agents: GeneratorAgent[];
  reviewer_agents: ReviewerAgent[];
}

export interface RunWithGenerations extends Run {
  generations: Generation[];
}
