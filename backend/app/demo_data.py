"""
Demo data for GAD System
Provides scripted demonstration data showing a GAD run evolving a password reset feature
"""
from app.models import (
    Run, Generation, Candidate, Metrics, PromptDNA, DNABundle,
    RPG, RPGNode, RPGEdge, GeneratorAgent, ReviewerAgent, RunWithGenerations
)


# Generator Agents
GENERATOR_AGENTS = [
    GeneratorAgent(
        id="gen_security",
        name="Security-First Generator",
        description="Prioritizes security best practices and vulnerability prevention",
        specialization="Security emphasis with OWASP top-10 awareness",
        example_prompt_dna={
            "system_preamble": "You are a security-focused code generator...",
            "security_weight": 0.9,
            "performance_weight": 0.5
        }
    ),
    GeneratorAgent(
        id="gen_performance",
        name="Performance-Optimized Generator",
        description="Focuses on efficiency, caching, and resource optimization",
        specialization="Performance and scalability emphasis",
        example_prompt_dna={
            "system_preamble": "You are a performance-focused code generator...",
            "security_weight": 0.6,
            "performance_weight": 0.9
        }
    ),
    GeneratorAgent(
        id="gen_ux",
        name="UX-Centered Generator",
        description="Emphasizes user experience, accessibility, and intuitive design",
        specialization="User experience and accessibility focus",
        example_prompt_dna={
            "system_preamble": "You are a UX-focused code generator...",
            "ux_weight": 0.9,
            "accessibility_weight": 0.8
        }
    ),
]


# Reviewer Agents
REVIEWER_AGENTS = [
    ReviewerAgent(
        id="rev_security",
        name="Security Reviewer",
        type="security",
        description="Scans for vulnerabilities, injection risks, and security anti-patterns",
        reliability_score=0.92,
        example_comments=[
            "Potential SQL injection in user input handling",
            "Missing rate limiting on password reset endpoint",
            "Token generation uses cryptographically secure random"
        ]
    ),
    ReviewerAgent(
        id="rev_style",
        name="Style Reviewer",
        type="style",
        description="Checks code style, naming conventions, and documentation",
        reliability_score=0.88,
        example_comments=[
            "Function naming follows camelCase convention",
            "Missing JSDoc comments on public API",
            "Consistent error handling pattern used"
        ]
    ),
    ReviewerAgent(
        id="rev_performance",
        name="Performance Reviewer",
        type="performance",
        description="Analyzes algorithmic complexity, resource usage, and bottlenecks",
        reliability_score=0.85,
        example_comments=[
            "Database query could benefit from indexing",
            "N+1 query pattern detected in user lookup",
            "Efficient caching strategy implemented"
        ]
    ),
    ReviewerAgent(
        id="rev_ux",
        name="UX Reviewer",
        type="ux",
        description="Evaluates user experience, accessibility, and interface clarity",
        reliability_score=0.90,
        example_comments=[
            "Error messages are clear and actionable",
            "Missing ARIA labels for screen readers",
            "Loading states properly communicated"
        ]
    ),
    ReviewerAgent(
        id="rev_license",
        name="License Reviewer",
        type="license",
        description="Checks for license compliance and dependency issues",
        reliability_score=0.95,
        example_comments=[
            "All dependencies have compatible licenses",
            "No GPL-licensed code detected",
            "Attribution requirements satisfied"
        ]
    ),
]


# Sample Prompt DNA for candidates
def create_prompt_dna(generation: int, variant: str) -> PromptDNA:
    """Create sample prompt DNA with variation based on generation and variant"""
    base_dna = PromptDNA(
        system_preamble=f"You are an expert full-stack developer. Generation {generation}, variant {variant}.",
        requirement_frame="Implement a secure password reset feature with email verification",
        exemplars=[
            "Use bcrypt for password hashing",
            "Implement rate limiting to prevent abuse",
            "Send verification tokens via email"
        ],
        tool_flags={
            "use_typescript": True,
            "use_react": True,
            "include_tests": True
        },
        hyperparameters={
            "temperature": 0.7,
            "max_tokens": 2000
        },
        persona_vector={
            "security_focus": 0.8 if variant == "secure" else 0.6,
            "performance_focus": 0.8 if variant == "fast" else 0.6,
            "ux_focus": 0.8 if variant == "ux" else 0.6
        },
        style_preferences=["functional", "typed", "documented"],
        policy_digest="OWASP_top10_v2023",
        trust_region_bounds={"max_deviation": 0.3}
    )
    return base_dna


# Generation 1: Initial diverse exploration
GEN1_CANDIDATES = [
    Candidate(
        id="c1_1",
        parent_ids=[],
        prompt_dna_summary="Security-focused, basic implementation",
        metrics=Metrics(
            test_pass_rate=0.60,
            coverage=0.55,
            security_score=0.85,
            performance_score=0.50,
            ux_score=0.45,
            functionality_score=0.65,
            style_compliance=0.70
        ),
        gates_passed=False,
        failed_gates=["test_pass_rate < 0.70"],
        effective_score=0.61,
        is_pareto_front=False,
        selected_for_breeding=False
    ),
    Candidate(
        id="c1_2",
        parent_ids=[],
        prompt_dna_summary="Performance-focused, minimal security",
        metrics=Metrics(
            test_pass_rate=0.75,
            coverage=0.60,
            security_score=0.50,
            performance_score=0.88,
            ux_score=0.55,
            functionality_score=0.70,
            style_compliance=0.65
        ),
        gates_passed=False,
        failed_gates=["security_score < 0.60 (critical vulnerability found)"],
        effective_score=0.66,
        is_pareto_front=False,
        selected_for_breeding=False
    ),
    Candidate(
        id="c1_3",
        parent_ids=[],
        prompt_dna_summary="Balanced approach",
        metrics=Metrics(
            test_pass_rate=0.80,
            coverage=0.70,
            security_score=0.75,
            performance_score=0.70,
            ux_score=0.65,
            functionality_score=0.80,
            style_compliance=0.75
        ),
        gates_passed=True,
        failed_gates=[],
        effective_score=0.74,
        is_pareto_front=True,
        selected_for_breeding=True,
        ucb_score=0.82,
        expected_info_gain=0.35,
        children_allocated=3
    ),
    Candidate(
        id="c1_4",
        parent_ids=[],
        prompt_dna_summary="UX-focused implementation",
        metrics=Metrics(
            test_pass_rate=0.72,
            coverage=0.65,
            security_score=0.70,
            performance_score=0.60,
            ux_score=0.90,
            functionality_score=0.75,
            style_compliance=0.80
        ),
        gates_passed=True,
        failed_gates=[],
        effective_score=0.73,
        is_pareto_front=True,
        selected_for_breeding=True,
        ucb_score=0.79,
        expected_info_gain=0.32,
        children_allocated=2
    ),
]


# Generation 2: Evolution with feedback integration
GEN2_CANDIDATES = [
    Candidate(
        id="c2_1",
        parent_ids=["c1_3"],
        prompt_dna_summary="Balanced + improved test coverage",
        metrics=Metrics(
            test_pass_rate=0.88,
            coverage=0.82,
            security_score=0.78,
            performance_score=0.72,
            ux_score=0.70,
            functionality_score=0.85,
            style_compliance=0.80
        ),
        gates_passed=True,
        failed_gates=[],
        effective_score=0.79,
        is_pareto_front=True,
        selected_for_breeding=True,
        ucb_score=0.86,
        expected_info_gain=0.28,
        children_allocated=3
    ),
    Candidate(
        id="c2_2",
        parent_ids=["c1_3"],
        prompt_dna_summary="Balanced + security hardening",
        metrics=Metrics(
            test_pass_rate=0.85,
            coverage=0.75,
            security_score=0.92,
            performance_score=0.68,
            ux_score=0.68,
            functionality_score=0.82,
            style_compliance=0.78
        ),
        gates_passed=True,
        failed_gates=[],
        effective_score=0.78,
        is_pareto_front=True,
        selected_for_breeding=True,
        ucb_score=0.85,
        expected_info_gain=0.30,
        children_allocated=3
    ),
    Candidate(
        id="c2_3",
        parent_ids=["c1_3"],
        prompt_dna_summary="Balanced + performance optimization",
        metrics=Metrics(
            test_pass_rate=0.82,
            coverage=0.72,
            security_score=0.76,
            performance_score=0.85,
            ux_score=0.67,
            functionality_score=0.83,
            style_compliance=0.77
        ),
        gates_passed=True,
        failed_gates=[],
        effective_score=0.77,
        is_pareto_front=True,
        selected_for_breeding=True,
        ucb_score=0.83,
        expected_info_gain=0.26,
        children_allocated=2
    ),
    Candidate(
        id="c2_4",
        parent_ids=["c1_4"],
        prompt_dna_summary="UX + improved functionality",
        metrics=Metrics(
            test_pass_rate=0.80,
            coverage=0.70,
            security_score=0.74,
            performance_score=0.65,
            ux_score=0.93,
            functionality_score=0.82,
            style_compliance=0.85
        ),
        gates_passed=True,
        failed_gates=[],
        effective_score=0.78,
        is_pareto_front=True,
        selected_for_breeding=True,
        ucb_score=0.84,
        expected_info_gain=0.27,
        children_allocated=2
    ),
    Candidate(
        id="c2_5",
        parent_ids=["c1_4"],
        prompt_dna_summary="UX + regression in coverage",
        metrics=Metrics(
            test_pass_rate=0.70,
            coverage=0.60,
            security_score=0.72,
            performance_score=0.62,
            ux_score=0.92,
            functionality_score=0.78,
            style_compliance=0.82
        ),
        gates_passed=False,
        failed_gates=["test_pass_rate < 0.70"],
        effective_score=0.74,
        is_pareto_front=False,
        selected_for_breeding=False
    ),
]


# Generation 3: Convergence toward optimal solutions
GEN3_CANDIDATES = [
    Candidate(
        id="c3_1",
        parent_ids=["c2_1"],
        prompt_dna_summary="High coverage + security improvements",
        metrics=Metrics(
            test_pass_rate=0.92,
            coverage=0.88,
            security_score=0.85,
            performance_score=0.75,
            ux_score=0.75,
            functionality_score=0.90,
            style_compliance=0.85
        ),
        gates_passed=True,
        failed_gates=[],
        effective_score=0.84,
        is_pareto_front=True,
        selected_for_breeding=True,
        ucb_score=0.89,
        expected_info_gain=0.22,
        children_allocated=3
    ),
    Candidate(
        id="c3_2",
        parent_ids=["c2_2"],
        prompt_dna_summary="Security excellence",
        metrics=Metrics(
            test_pass_rate=0.90,
            coverage=0.82,
            security_score=0.96,
            performance_score=0.70,
            ux_score=0.72,
            functionality_score=0.88,
            style_compliance=0.82
        ),
        gates_passed=True,
        failed_gates=[],
        effective_score=0.83,
        is_pareto_front=True,
        selected_for_breeding=True,
        ucb_score=0.88,
        expected_info_gain=0.20,
        children_allocated=2
    ),
    Candidate(
        id="c3_3",
        parent_ids=["c2_1", "c2_3"],
        prompt_dna_summary="Balanced crossover - coverage + performance",
        metrics=Metrics(
            test_pass_rate=0.90,
            coverage=0.85,
            security_score=0.82,
            performance_score=0.88,
            ux_score=0.73,
            functionality_score=0.88,
            style_compliance=0.83
        ),
        gates_passed=True,
        failed_gates=[],
        effective_score=0.84,
        is_pareto_front=True,
        selected_for_breeding=True,
        ucb_score=0.89,
        expected_info_gain=0.21,
        children_allocated=3
    ),
    Candidate(
        id="c3_4",
        parent_ids=["c2_4"],
        prompt_dna_summary="UX excellence + all-around solid",
        metrics=Metrics(
            test_pass_rate=0.88,
            coverage=0.80,
            security_score=0.80,
            performance_score=0.72,
            ux_score=0.95,
            functionality_score=0.87,
            style_compliance=0.88
        ),
        gates_passed=True,
        failed_gates=[],
        effective_score=0.84,
        is_pareto_front=True,
        selected_for_breeding=True,
        ucb_score=0.88,
        expected_info_gain=0.19,
        children_allocated=2
    ),
]


# Generation 4: Final refinement
GEN4_CANDIDATES = [
    Candidate(
        id="c4_1",
        parent_ids=["c3_1", "c3_3"],
        prompt_dna_summary="Near-optimal all-around solution",
        metrics=Metrics(
            test_pass_rate=0.95,
            coverage=0.92,
            security_score=0.90,
            performance_score=0.85,
            ux_score=0.82,
            functionality_score=0.95,
            style_compliance=0.90
        ),
        gates_passed=True,
        failed_gates=[],
        effective_score=0.90,
        is_pareto_front=True,
        selected_for_breeding=True,
        ucb_score=0.93,
        expected_info_gain=0.12,
        children_allocated=2
    ),
    Candidate(
        id="c4_2",
        parent_ids=["c3_2"],
        prompt_dna_summary="Maximum security solution",
        metrics=Metrics(
            test_pass_rate=0.92,
            coverage=0.85,
            security_score=0.98,
            performance_score=0.72,
            ux_score=0.75,
            functionality_score=0.90,
            style_compliance=0.85
        ),
        gates_passed=True,
        failed_gates=[],
        effective_score=0.85,
        is_pareto_front=True,
        selected_for_breeding=False,
        ucb_score=0.87,
        expected_info_gain=0.08,
        children_allocated=0
    ),
    Candidate(
        id="c4_3",
        parent_ids=["c3_3", "c3_4"],
        prompt_dna_summary="Performance + UX optimized",
        metrics=Metrics(
            test_pass_rate=0.90,
            coverage=0.85,
            security_score=0.85,
            performance_score=0.92,
            ux_score=0.90,
            functionality_score=0.92,
            style_compliance=0.88
        ),
        gates_passed=True,
        failed_gates=[],
        effective_score=0.89,
        is_pareto_front=True,
        selected_for_breeding=True,
        ucb_score=0.91,
        expected_info_gain=0.10,
        children_allocated=1
    ),
]


# Generations
GENERATIONS = [
    Generation(
        generation_number=1,
        summary="Initial diverse exploration. 4 candidates from different generator agents. 2 failed hard gates (test coverage, security). 2 survivors selected for breeding.",
        candidates=GEN1_CANDIDATES,
        pareto_front_count=2,
        selected_for_breeding_count=2,
        avg_score=0.69,
        best_score=0.74,
        diversity_score=0.82
    ),
    Generation(
        generation_number=2,
        summary="First evolution. 5 candidates bred from Gen1 survivors. Feedback integration improved scores. 4 survivors on Pareto front. One candidate regressed and failed gates.",
        candidates=GEN2_CANDIDATES,
        pareto_front_count=4,
        selected_for_breeding_count=4,
        avg_score=0.77,
        best_score=0.79,
        diversity_score=0.75
    ),
    Generation(
        generation_number=3,
        summary="Convergence phase. 4 candidates including one crossover. All passed gates. Strong improvements in coverage and security. 4 survivors selected.",
        candidates=GEN3_CANDIDATES,
        pareto_front_count=4,
        selected_for_breeding_count=4,
        avg_score=0.84,
        best_score=0.84,
        diversity_score=0.68
    ),
    Generation(
        generation_number=4,
        summary="Final refinement. 3 high-quality candidates. Near-optimal solution achieved (c4_1). Pareto front shows tradeoff between all-around excellence and specialized optimization.",
        candidates=GEN4_CANDIDATES,
        pareto_front_count=3,
        selected_for_breeding_count=2,
        avg_score=0.88,
        best_score=0.90,
        diversity_score=0.60
    ),
]


# RPG for password reset feature
RPG_NODES = [
    RPGNode(
        id="cap_password_reset",
        type="capability",
        name="Password Reset Capability",
        description="Complete user password reset flow with email verification",
        implementation_status="complete",
        associated_tests=["test_password_reset_flow"],
        touched_by_generations=[1, 2, 3, 4]
    ),
    RPGNode(
        id="mod_auth",
        type="module",
        name="Authentication Module",
        description="Core authentication and user management",
        implementation_status="complete",
        associated_tests=["test_auth_module"],
        touched_by_generations=[1, 2, 3]
    ),
    RPGNode(
        id="mod_email",
        type="module",
        name="Email Service Module",
        description="Email sending and template management",
        implementation_status="complete",
        associated_tests=["test_email_service"],
        touched_by_generations=[1, 2, 3]
    ),
    RPGNode(
        id="file_reset_controller",
        type="file",
        name="reset_controller.ts",
        description="HTTP endpoints for password reset",
        implementation_status="complete",
        associated_tests=["test_reset_endpoints"],
        touched_by_generations=[1, 2, 3, 4]
    ),
    RPGNode(
        id="file_token_service",
        type="file",
        name="token_service.ts",
        description="Reset token generation and validation",
        implementation_status="complete",
        associated_tests=["test_token_service"],
        touched_by_generations=[1, 2, 3, 4]
    ),
    RPGNode(
        id="func_generate_token",
        type="function",
        name="generateResetToken",
        description="Creates cryptographically secure reset token",
        implementation_status="complete",
        associated_tests=["test_generate_token"],
        touched_by_generations=[1, 2, 3]
    ),
    RPGNode(
        id="func_validate_token",
        type="function",
        name="validateResetToken",
        description="Validates token and checks expiration",
        implementation_status="complete",
        associated_tests=["test_validate_token"],
        touched_by_generations=[1, 2, 3, 4]
    ),
    RPGNode(
        id="func_send_reset_email",
        type="function",
        name="sendResetEmail",
        description="Sends password reset email to user",
        implementation_status="complete",
        associated_tests=["test_send_email"],
        touched_by_generations=[1, 2, 3]
    ),
    RPGNode(
        id="test_integration",
        type="test",
        name="integration_test_suite",
        description="End-to-end password reset flow tests",
        implementation_status="complete",
        associated_tests=[],
        touched_by_generations=[2, 3, 4]
    ),
]


RPG_EDGES = [
    RPGEdge(from_node="cap_password_reset", to_node="mod_auth", relation_type="depends_on"),
    RPGEdge(from_node="cap_password_reset", to_node="mod_email", relation_type="depends_on"),
    RPGEdge(from_node="mod_auth", to_node="file_reset_controller", relation_type="contains"),
    RPGEdge(from_node="mod_auth", to_node="file_token_service", relation_type="contains"),
    RPGEdge(from_node="file_reset_controller", to_node="func_generate_token", relation_type="calls"),
    RPGEdge(from_node="file_reset_controller", to_node="func_validate_token", relation_type="calls"),
    RPGEdge(from_node="file_reset_controller", to_node="func_send_reset_email", relation_type="calls"),
    RPGEdge(from_node="file_token_service", to_node="func_generate_token", relation_type="contains"),
    RPGEdge(from_node="file_token_service", to_node="func_validate_token", relation_type="contains"),
    RPGEdge(from_node="mod_email", to_node="func_send_reset_email", relation_type="contains"),
    RPGEdge(from_node="test_integration", to_node="cap_password_reset", relation_type="tests"),
    RPGEdge(from_node="test_integration", to_node="file_reset_controller", relation_type="tests"),
    RPGEdge(from_node="test_integration", to_node="file_token_service", relation_type="tests"),
]


RPG_DATA = RPG(nodes=RPG_NODES, edges=RPG_EDGES)


# DNA Bundle example
DNA_BUNDLE_EXAMPLE = DNABundle(
    line_id="line_c3_1",
    branch_ref="gad/gen3/candidate1",
    prompt_dna=create_prompt_dna(3, "balanced"),
    feedback_summary="Gen2 feedback: Improve test coverage especially edge cases. Security reviewer noted good practices. Performance reviewer suggested index optimization.",
    evidence_metrics=Metrics(
        test_pass_rate=0.92,
        coverage=0.88,
        security_score=0.85,
        performance_score=0.75,
        ux_score=0.75,
        functionality_score=0.90,
        style_compliance=0.85
    ),
    selector_state={
        "generation": 3,
        "parent_ids": ["c2_1"],
        "selection_method": "GEPA",
        "pareto_rank": 1,
        "crowding_distance": 0.45
    },
    evaluator_state={
        "total_tests_run": 125,
        "tests_passed": 115,
        "security_scans_performed": 3,
        "performance_benchmarks": 8
    },
    policy_state={
        "owasp_compliance": True,
        "license_compliance": True,
        "style_guide_version": "2023.1"
    },
    provenance={
        "lineage": ["c1_3", "c2_1", "c3_1"],
        "generation_created": 3,
        "mutations_from_parent": ["increased_test_coverage", "added_edge_case_handling"],
        "crossover_source": None
    }
)


# Main run
SAMPLE_RUN = RunWithGenerations(
    id="run_001",
    name="Password Reset Feature",
    description="Implement secure password reset with email verification",
    requirement_summary="Users need ability to reset forgotten passwords. System must send verification email with secure token, validate token on reset, and enforce password strength requirements. Must meet OWASP security standards.",
    total_generations=4,
    final_status="success",
    generator_agents=GENERATOR_AGENTS,
    reviewer_agents=REVIEWER_AGENTS,
    generations=GENERATIONS
)


def get_run(run_id: str) -> RunWithGenerations:
    """Get a specific run by ID"""
    if run_id == "run_001":
        return SAMPLE_RUN
    return None


def get_generation(run_id: str, generation_number: int) -> Generation:
    """Get a specific generation from a run"""
    if run_id == "run_001" and 1 <= generation_number <= 4:
        return GENERATIONS[generation_number - 1]
    return None


def get_dna_bundle(run_id: str, line_id: str) -> DNABundle:
    """Get DNA bundle for a specific lineage"""
    if run_id == "run_001" and line_id == "line_c3_1":
        return DNA_BUNDLE_EXAMPLE
    return None


def get_rpg(run_id: str) -> RPG:
    """Get Repository Planning Graph for a run"""
    if run_id == "run_001":
        return RPG_DATA
    return None
