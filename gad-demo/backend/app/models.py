"""
Data models for the GAD demo system.
These models represent the core entities in the Generative Adversarial Development pipeline.
"""

from typing import List, Dict, Optional, Any
from pydantic import BaseModel, Field


class PromptDNA(BaseModel):
    """Represents the genetic instructions for code generation."""
    id: str
    system_prompt: str
    task_description: str
    constraints: List[str]
    examples: List[str]
    temperature: float
    top_p: float
    feedback_history: List[str]
    generation: int
    parent_ids: List[str]
    mutations: List[Dict[str, Any]]
    trust_region_similarity: Optional[float] = None


class Metrics(BaseModel):
    """Aggregated metrics for a candidate solution."""
    test_pass_rate: float = Field(ge=0, le=1)
    coverage: float = Field(ge=0, le=1)
    performance_score: float = Field(ge=0, le=100)
    security_score: float = Field(ge=0, le=100)
    ux_score: float = Field(ge=0, le=100)
    style_score: float = Field(ge=0, le=100)
    license_compliance: bool
    vulnerability_count: int = Field(ge=0)


class GateResult(BaseModel):
    """Result of a hard gate check."""
    gate_name: str
    passed: bool
    message: str
    threshold: Optional[float] = None
    actual: Optional[float] = None


class ReviewerComment(BaseModel):
    """Comment from a reviewer agent."""
    reviewer_id: str
    reviewer_type: str
    timestamp: str
    severity: str  # "critical", "warning", "info"
    category: str  # "security", "performance", "ux", "quality"
    message: str
    line_numbers: Optional[List[int]] = None


class Candidate(BaseModel):
    """A candidate solution in a generation."""
    id: str
    generation: int
    parent_ids: List[str]
    prompt_dna_id: str
    prompt_dna_summary: str

    # Metrics
    metrics: Metrics

    # Scoring
    effective_score: float
    weighted_scores: Dict[str, float]

    # Gates
    gates_passed: bool
    gate_results: List[GateResult]

    # Selection
    is_pareto_front: bool
    selected_for_breeding: bool
    survival_reason: Optional[str] = None

    # Provenance
    branch: str
    commit_id: str
    generator_agent_id: str

    # Reviews
    reviewer_comments: List[ReviewerComment]


class AgentProfile(BaseModel):
    """Profile of a generator or reviewer agent."""
    id: str
    name: str
    type: str  # "generator" or "reviewer"
    specialization: str
    reliability_score: Optional[float] = None  # For reviewers
    generations_participated: int
    successful_candidates: Optional[int] = None  # For generators


class UCBStats(BaseModel):
    """Upper Confidence Bound statistics for agent allocation."""
    agent_id: str
    mean_reward: float
    confidence_interval: float
    exploration_bonus: float
    total_score: float
    times_selected: int


class ParetoPoint(BaseModel):
    """Point on the Pareto front."""
    candidate_id: str
    objective1: float  # e.g., quality
    objective2: float  # e.g., performance
    label: str


class Generation(BaseModel):
    """A complete generation in the GAD run."""
    number: int
    candidates: List[Candidate]
    pareto_front: List[ParetoPoint]
    ucb_allocations: List[UCBStats]
    survivors: List[str]  # candidate IDs
    breeding_pairs: List[tuple[str, str]]
    summary: str


class CodeLayer(BaseModel):
    """Code layer of a DNA bundle."""
    branch: str
    commit_id: str
    diff_summary: str
    files_changed: int
    lines_added: int
    lines_removed: int
    diff_url: Optional[str] = None


class EvaluatorLayer(BaseModel):
    """Evaluator layer of a DNA bundle."""
    reviewer_reliabilities: Dict[str, float]
    anti_cheat_seed: str
    ucb_stats: List[UCBStats]
    policy_version: str
    merkle_root: str


class DNABundle(BaseModel):
    """Complete DNA bundle with all three layers."""
    id: str
    candidate_id: str
    code_layer: CodeLayer
    prompt_layer: PromptDNA
    evaluator_layer: EvaluatorLayer
    provenance_hash: str
    parent_hashes: List[str]
    timestamp: str


class RPGNode(BaseModel):
    """Node in the Repository Planning Graph."""
    id: str
    type: str  # "capability", "module", "file", "function", "test"
    name: str
    description: str
    status: str  # "planned", "in_progress", "implemented", "tested"


class RPGEdge(BaseModel):
    """Edge in the Repository Planning Graph."""
    source: str
    target: str
    type: str  # "implements", "calls", "depends", "tested_by"


class RepositoryPlanningGraph(BaseModel):
    """Complete RPG structure."""
    nodes: List[RPGNode]
    edges: List[RPGEdge]


class GADRun(BaseModel):
    """Complete GAD run with all generations."""
    id: str
    name: str
    requirement: str
    total_generations: int
    generations: List[Generation]
    final_candidate_id: Optional[str] = None
    rpg: RepositoryPlanningGraph
    agents: List[AgentProfile]
