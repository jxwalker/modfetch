"""
Data models for GAD System Demo
These models represent the core concepts in the Generative Adversarial Development system
"""
from typing import List, Dict, Any, Optional
from pydantic import BaseModel, Field


class PromptDNA(BaseModel):
    """
    Prompt DNA encodes the heritable information for generating code
    Key invention: Evolution at the prompt level, not just code level
    """
    system_preamble: str
    requirement_frame: str
    exemplars: List[str]
    tool_flags: Dict[str, bool]
    hyperparameters: Dict[str, Any]
    persona_vector: Dict[str, float]
    style_preferences: List[str]
    policy_digest: str
    trust_region_bounds: Optional[Dict[str, float]] = None


class Metrics(BaseModel):
    """Evaluation metrics for a candidate solution"""
    test_pass_rate: float = Field(ge=0, le=1)
    coverage: float = Field(ge=0, le=1)
    security_score: float = Field(ge=0, le=1)
    performance_score: float = Field(ge=0, le=1)
    ux_score: float = Field(ge=0, le=1)
    functionality_score: float = Field(ge=0, le=1)
    style_compliance: float = Field(ge=0, le=1)


class Candidate(BaseModel):
    """
    A candidate solution in a generation
    Key invention: Multi-dimensional evaluation with hard gates and soft scores
    """
    id: str
    parent_ids: List[str]
    prompt_dna_summary: str
    metrics: Metrics
    gates_passed: bool
    failed_gates: List[str]
    effective_score: float
    is_pareto_front: bool
    selected_for_breeding: bool
    ucb_score: Optional[float] = None
    expected_info_gain: Optional[float] = None
    children_allocated: Optional[int] = None


class GeneratorAgent(BaseModel):
    """Generator agent configuration"""
    id: str
    name: str
    description: str
    specialization: str
    example_prompt_dna: Dict[str, Any]


class ReviewerAgent(BaseModel):
    """
    Reviewer agent configuration
    Key invention: Multi-agent adversarial review with reliability tracking
    """
    id: str
    name: str
    type: str  # security, style, performance, ux, license
    description: str
    reliability_score: float = Field(ge=0, le=1)
    example_comments: List[str]


class Generation(BaseModel):
    """
    A single generation in the GAD loop
    Key invention: Evolutionary loop with Pareto-based selection
    """
    generation_number: int
    summary: str
    candidates: List[Candidate]
    pareto_front_count: int
    selected_for_breeding_count: int
    avg_score: float
    best_score: float
    diversity_score: float


class DNABundle(BaseModel):
    """
    DNA Bundle: Complete state package for a lineage
    Key invention: Multi-layered hereditary information with provenance
    """
    line_id: str
    branch_ref: str
    prompt_dna: PromptDNA
    feedback_summary: str
    evidence_metrics: Metrics
    selector_state: Dict[str, Any]
    evaluator_state: Dict[str, Any]
    policy_state: Dict[str, Any]
    provenance: Dict[str, Any]


class RPGNode(BaseModel):
    """
    Repository Planning Graph node
    Key invention: Long-horizon architectural coherence
    """
    id: str
    type: str  # capability, module, file, function, test
    name: str
    description: str
    implementation_status: str
    associated_tests: List[str]
    touched_by_generations: List[int]


class RPGEdge(BaseModel):
    """Edge in the Repository Planning Graph"""
    from_node: str
    to_node: str
    relation_type: str  # implements, depends_on, calls, tested_by


class RPG(BaseModel):
    """
    Repository Planning Graph
    Maintains architectural coherence across generations
    """
    nodes: List[RPGNode]
    edges: List[RPGEdge]


class Run(BaseModel):
    """
    A complete GAD run
    Represents the full evolutionary process for a feature
    """
    id: str
    name: str
    description: str
    requirement_summary: str
    total_generations: int
    final_status: str
    generator_agents: List[GeneratorAgent]
    reviewer_agents: List[ReviewerAgent]


class RunWithGenerations(Run):
    """Run with all generation data"""
    generations: List[Generation]
