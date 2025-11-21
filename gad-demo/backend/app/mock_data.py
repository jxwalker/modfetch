"""
Mock data generator for the GAD demo system.
Creates deterministic, realistic data for a 5-generation GAD run.
"""

from typing import List, Dict
from app.models import (
    GADRun, Generation, Candidate, Metrics, GateResult, ReviewerComment,
    AgentProfile, UCBStats, ParetoPoint, DNABundle, CodeLayer, EvaluatorLayer,
    PromptDNA, RepositoryPlanningGraph, RPGNode, RPGEdge
)
import random
from datetime import datetime, timedelta


def generate_prompt_dna(generation: int, candidate_id: str, parent_ids: List[str]) -> PromptDNA:
    """Generate a prompt DNA object."""
    base_task = "Implement a secure REST API endpoint for user authentication with JWT tokens"

    mutations = []
    if generation > 0:
        if random.random() > 0.5:
            mutations.append({
                "type": "feedback_integration",
                "change": "Added explicit error handling requirements from reviewer feedback"
            })
        if random.random() > 0.7:
            mutations.append({
                "type": "constraint_refinement",
                "change": "Tightened security constraints based on vulnerability scan"
            })

    feedback_history = []
    if generation > 0:
        feedback_history = [
            f"Gen {generation-1}: Improve input validation",
            f"Gen {generation-1}: Add rate limiting",
        ]

    similarity = None
    if generation > 0:
        similarity = 0.85 + random.random() * 0.1  # High similarity within trust region

    return PromptDNA(
        id=f"prompt-dna-{candidate_id}",
        system_prompt="You are an expert backend engineer specializing in secure API development.",
        task_description=base_task + (f" [Gen {generation} refinement]" if generation > 0 else ""),
        constraints=[
            "Must use bcrypt for password hashing",
            "JWT tokens must expire in 15 minutes",
            "Rate limit: 10 requests per minute per IP",
            "All inputs must be validated and sanitized"
        ],
        examples=[
            "Example: POST /auth/login with email and password",
            "Example: Return JWT token on success, 401 on failure"
        ],
        temperature=0.7,
        top_p=0.9,
        feedback_history=feedback_history,
        generation=generation,
        parent_ids=parent_ids,
        mutations=mutations,
        trust_region_similarity=similarity
    )


def generate_metrics(generation: int, is_good: bool) -> Metrics:
    """Generate metrics for a candidate."""
    if is_good:
        return Metrics(
            test_pass_rate=0.95 + random.random() * 0.05,
            coverage=0.85 + random.random() * 0.1,
            performance_score=85 + random.random() * 10,
            security_score=90 + random.random() * 10,
            ux_score=80 + random.random() * 15,
            style_score=90 + random.random() * 10,
            license_compliance=True,
            vulnerability_count=random.randint(0, 1)
        )
    else:
        return Metrics(
            test_pass_rate=0.6 + random.random() * 0.2,
            coverage=0.5 + random.random() * 0.2,
            performance_score=50 + random.random() * 30,
            security_score=40 + random.random() * 40,
            ux_score=50 + random.random() * 30,
            style_score=60 + random.random() * 30,
            license_compliance=random.random() > 0.3,
            vulnerability_count=random.randint(2, 8)
        )


def generate_gate_results(metrics: Metrics) -> tuple[bool, List[GateResult]]:
    """Generate gate results based on metrics."""
    gates = [
        GateResult(
            gate_name="Minimum Test Pass Rate",
            passed=metrics.test_pass_rate >= 0.8,
            message="All critical tests must pass",
            threshold=0.8,
            actual=metrics.test_pass_rate
        ),
        GateResult(
            gate_name="Security Threshold",
            passed=metrics.security_score >= 70,
            message="Security score must meet minimum threshold",
            threshold=70.0,
            actual=metrics.security_score
        ),
        GateResult(
            gate_name="Zero Critical Vulnerabilities",
            passed=metrics.vulnerability_count == 0,
            message="No critical vulnerabilities allowed",
            threshold=0.0,
            actual=float(metrics.vulnerability_count)
        ),
        GateResult(
            gate_name="License Compliance",
            passed=metrics.license_compliance,
            message="All dependencies must have compatible licenses",
            threshold=None,
            actual=None
        )
    ]

    all_passed = all(gate.passed for gate in gates)
    return all_passed, gates


def generate_reviewer_comments(metrics: Metrics, gates_passed: bool) -> List[ReviewerComment]:
    """Generate reviewer comments."""
    comments = []

    if not gates_passed:
        comments.append(ReviewerComment(
            reviewer_id="reviewer-security-001",
            reviewer_type="security",
            timestamp=datetime.now().isoformat(),
            severity="critical",
            category="security",
            message="SQL injection vulnerability detected in login endpoint",
            line_numbers=[45, 46, 47]
        ))

    if metrics.performance_score < 80:
        comments.append(ReviewerComment(
            reviewer_id="reviewer-performance-001",
            reviewer_type="performance",
            timestamp=datetime.now().isoformat(),
            severity="warning",
            category="performance",
            message="Database query not optimized, consider adding index",
            line_numbers=[78, 79]
        ))

    if metrics.ux_score > 85:
        comments.append(ReviewerComment(
            reviewer_id="reviewer-ux-001",
            reviewer_type="ux",
            timestamp=datetime.now().isoformat(),
            severity="info",
            category="ux",
            message="Error messages are clear and actionable - excellent UX",
            line_numbers=None
        ))

    return comments


def generate_candidate(
    gen_num: int,
    candidate_num: int,
    parent_ids: List[str],
    is_good: bool,
    is_pareto: bool = False,
    selected: bool = False
) -> Candidate:
    """Generate a single candidate."""
    cand_id = f"gen{gen_num}-cand{candidate_num}"
    metrics = generate_metrics(gen_num, is_good)
    gates_passed, gate_results = generate_gate_results(metrics)
    comments = generate_reviewer_comments(metrics, gates_passed)

    # Calculate weighted scores
    weights = {
        "test_pass_rate": 0.25,
        "security": 0.25,
        "performance": 0.15,
        "ux": 0.15,
        "coverage": 0.10,
        "style": 0.10
    }

    effective_score = (
        metrics.test_pass_rate * weights["test_pass_rate"] * 100 +
        metrics.security_score * weights["security"] +
        metrics.performance_score * weights["performance"] +
        metrics.ux_score * weights["ux"] +
        metrics.coverage * weights["coverage"] * 100 +
        metrics.style_score * weights["style"]
    )

    if not gates_passed:
        effective_score *= 0.5  # Heavy penalty for gate failures

    weighted_scores = {
        "test_pass_rate": metrics.test_pass_rate * weights["test_pass_rate"] * 100,
        "security": metrics.security_score * weights["security"],
        "performance": metrics.performance_score * weights["performance"],
        "ux": metrics.ux_score * weights["ux"],
        "coverage": metrics.coverage * weights["coverage"] * 100,
        "style": metrics.style_score * weights["style"]
    }

    survival_reason = None
    if selected:
        if is_pareto:
            survival_reason = "Pareto optimal (quality vs. performance trade-off)"
        else:
            survival_reason = "High effective score with all gates passed"

    prompt_dna = generate_prompt_dna(gen_num, cand_id, parent_ids)

    return Candidate(
        id=cand_id,
        generation=gen_num,
        parent_ids=parent_ids,
        prompt_dna_id=prompt_dna.id,
        prompt_dna_summary=prompt_dna.task_description[:100] + "...",
        metrics=metrics,
        effective_score=effective_score,
        weighted_scores=weighted_scores,
        gates_passed=gates_passed,
        gate_results=gate_results,
        is_pareto_front=is_pareto,
        selected_for_breeding=selected,
        survival_reason=survival_reason,
        branch=f"gad/gen{gen_num}/{cand_id}",
        commit_id=f"abc{gen_num}{candidate_num:03d}def",
        generator_agent_id=f"generator-{random.randint(1, 3):03d}",
        reviewer_comments=comments
    )


def generate_generation(gen_num: int, parent_candidates: List[str]) -> Generation:
    """Generate a complete generation."""
    # Generation 0: 8 initial candidates
    # Later generations: 6-8 candidates
    num_candidates = 8 if gen_num == 0 else random.randint(6, 8)

    candidates = []
    for i in range(num_candidates):
        # Quality improves over generations
        is_good = random.random() < (0.3 + gen_num * 0.15)

        # Select parents
        if gen_num == 0:
            parents = []
        else:
            parents = random.sample(parent_candidates, min(2, len(parent_candidates)))

        candidates.append(generate_candidate(
            gen_num, i, parents, is_good
        ))

    # Determine survivors (top 2-3 candidates with gates passed)
    passing_candidates = [c for c in candidates if c.gates_passed]
    passing_candidates.sort(key=lambda c: c.effective_score, reverse=True)

    survivors = []
    if len(passing_candidates) >= 2:
        survivors = passing_candidates[:3]
        for survivor in survivors:
            survivor.selected_for_breeding = True
            survivor.is_pareto_front = random.random() > 0.5
            survivor.survival_reason = (
                "Pareto optimal" if survivor.is_pareto_front
                else "High effective score"
            )

    # Generate Pareto front
    pareto_points = []
    for cand in candidates:
        pareto_points.append(ParetoPoint(
            candidate_id=cand.id,
            objective1=cand.metrics.security_score,
            objective2=cand.metrics.performance_score,
            label=cand.id
        ))

    # Generate UCB allocations
    ucb_allocations = [
        UCBStats(
            agent_id="generator-001",
            mean_reward=0.75 + random.random() * 0.2,
            confidence_interval=0.05,
            exploration_bonus=0.08,
            total_score=0.88,
            times_selected=gen_num * 3 + 5
        ),
        UCBStats(
            agent_id="generator-002",
            mean_reward=0.70 + random.random() * 0.15,
            confidence_interval=0.08,
            exploration_bonus=0.12,
            total_score=0.85,
            times_selected=gen_num * 2 + 4
        ),
        UCBStats(
            agent_id="generator-003",
            mean_reward=0.65 + random.random() * 0.15,
            confidence_interval=0.10,
            exploration_bonus=0.15,
            total_score=0.82,
            times_selected=gen_num * 2 + 2
        )
    ]

    # Breeding pairs
    breeding_pairs = []
    if len(survivors) >= 2:
        breeding_pairs = [(survivors[0].id, survivors[1].id)]
        if len(survivors) >= 3:
            breeding_pairs.append((survivors[0].id, survivors[2].id))

    summary = f"Generation {gen_num}: {len(candidates)} candidates, {len(survivors)} survivors"

    return Generation(
        number=gen_num,
        candidates=candidates,
        pareto_front=pareto_points,
        ucb_allocations=ucb_allocations,
        survivors=[s.id for s in survivors],
        breeding_pairs=breeding_pairs,
        summary=summary
    )


def generate_agents() -> List[AgentProfile]:
    """Generate agent profiles."""
    return [
        AgentProfile(
            id="generator-001",
            name="CodeCraft Pro",
            type="generator",
            specialization="Security-focused backend development",
            reliability_score=None,
            generations_participated=5,
            successful_candidates=8
        ),
        AgentProfile(
            id="generator-002",
            name="SwiftCode AI",
            type="generator",
            specialization="High-performance API design",
            reliability_score=None,
            generations_participated=5,
            successful_candidates=6
        ),
        AgentProfile(
            id="generator-003",
            name="CleanCode Engine",
            type="generator",
            specialization="Test-driven development",
            reliability_score=None,
            generations_participated=4,
            successful_candidates=4
        ),
        AgentProfile(
            id="reviewer-security-001",
            name="SecureGuard",
            type="reviewer",
            specialization="Security & vulnerability analysis",
            reliability_score=0.95,
            generations_participated=5,
            successful_candidates=None
        ),
        AgentProfile(
            id="reviewer-performance-001",
            name="SpeedChecker",
            type="reviewer",
            specialization="Performance & optimization",
            reliability_score=0.88,
            generations_participated=5,
            successful_candidates=None
        ),
        AgentProfile(
            id="reviewer-ux-001",
            name="UXValidator",
            type="reviewer",
            specialization="User experience & API design",
            reliability_score=0.92,
            generations_participated=5,
            successful_candidates=None
        ),
        AgentProfile(
            id="reviewer-quality-001",
            name="QualityGate",
            type="reviewer",
            specialization="Code quality & maintainability",
            reliability_score=0.90,
            generations_participated=5,
            successful_candidates=None
        )
    ]


def generate_rpg() -> RepositoryPlanningGraph:
    """Generate Repository Planning Graph."""
    nodes = [
        RPGNode(id="cap-auth", type="capability", name="User Authentication",
                description="Complete authentication system", status="implemented"),
        RPGNode(id="mod-api", type="module", name="API Module",
                description="REST API handlers", status="implemented"),
        RPGNode(id="mod-db", type="module", name="Database Module",
                description="Database access layer", status="implemented"),
        RPGNode(id="file-auth-py", type="file", name="auth.py",
                description="Authentication handlers", status="implemented"),
        RPGNode(id="file-models-py", type="file", name="models.py",
                description="Data models", status="implemented"),
        RPGNode(id="func-login", type="function", name="login()",
                description="Handle user login", status="tested"),
        RPGNode(id="func-verify-token", type="function", name="verify_token()",
                description="Verify JWT token", status="tested"),
        RPGNode(id="test-auth", type="test", name="test_auth.py",
                description="Authentication tests", status="implemented"),
        RPGNode(id="test-security", type="test", name="test_security.py",
                description="Security tests", status="implemented"),
    ]

    edges = [
        RPGEdge(source="cap-auth", target="mod-api", type="implements"),
        RPGEdge(source="cap-auth", target="mod-db", type="depends"),
        RPGEdge(source="mod-api", target="file-auth-py", type="implements"),
        RPGEdge(source="mod-api", target="file-models-py", type="depends"),
        RPGEdge(source="file-auth-py", target="func-login", type="implements"),
        RPGEdge(source="file-auth-py", target="func-verify-token", type="implements"),
        RPGEdge(source="func-login", target="func-verify-token", type="calls"),
        RPGEdge(source="test-auth", target="func-login", type="tested_by"),
        RPGEdge(source="test-security", target="func-verify-token", type="tested_by"),
    ]

    return RepositoryPlanningGraph(nodes=nodes, edges=edges)


def generate_dna_bundle(candidate: Candidate) -> DNABundle:
    """Generate DNA bundle for a candidate."""
    prompt_dna = generate_prompt_dna(
        candidate.generation,
        candidate.id,
        candidate.parent_ids
    )

    code_layer = CodeLayer(
        branch=candidate.branch,
        commit_id=candidate.commit_id,
        diff_summary="Added JWT authentication with bcrypt password hashing",
        files_changed=5,
        lines_added=234,
        lines_removed=12,
        diff_url=f"https://github.com/example/gad-run/commit/{candidate.commit_id}"
    )

    evaluator_layer = EvaluatorLayer(
        reviewer_reliabilities={
            "reviewer-security-001": 0.95,
            "reviewer-performance-001": 0.88,
            "reviewer-ux-001": 0.92,
            "reviewer-quality-001": 0.90
        },
        anti_cheat_seed=f"seed-{candidate.id}",
        ucb_stats=[
            UCBStats(
                agent_id=candidate.generator_agent_id,
                mean_reward=0.75,
                confidence_interval=0.05,
                exploration_bonus=0.08,
                total_score=0.88,
                times_selected=15
            )
        ],
        policy_version="v1.2.0",
        merkle_root=f"merkle-{candidate.commit_id}"
    )

    return DNABundle(
        id=f"bundle-{candidate.id}",
        candidate_id=candidate.id,
        code_layer=code_layer,
        prompt_layer=prompt_dna,
        evaluator_layer=evaluator_layer,
        provenance_hash=f"hash-{candidate.id}",
        parent_hashes=[f"hash-{pid}" for pid in candidate.parent_ids],
        timestamp=datetime.now().isoformat()
    )


def create_sample_run() -> GADRun:
    """Create a complete sample GAD run with 5 generations."""
    random.seed(42)  # Deterministic results

    requirement = (
        "Implement a secure REST API endpoint for user authentication with JWT tokens. "
        "The system must handle login, token validation, and refresh. "
        "All inputs must be validated, passwords must be hashed with bcrypt, "
        "and the system must be rate-limited to prevent abuse."
    )

    generations = []
    parent_ids = []

    # Generate 5 generations
    for i in range(5):
        gen = generate_generation(i, parent_ids)
        generations.append(gen)
        parent_ids = gen.survivors

    # The final candidate is the best from the last generation
    final_gen = generations[-1]
    final_candidate = None
    if final_gen.survivors:
        final_candidate = final_gen.survivors[0]

    return GADRun(
        id="sample-run-001",
        name="JWT Authentication API Implementation",
        requirement=requirement,
        total_generations=5,
        generations=generations,
        final_candidate_id=final_candidate,
        rpg=generate_rpg(),
        agents=generate_agents()
    )


# Global sample run instance
SAMPLE_RUN = create_sample_run()


def get_sample_run() -> GADRun:
    """Get the sample GAD run."""
    return SAMPLE_RUN


def get_generation(gen_num: int) -> Generation:
    """Get a specific generation."""
    if 0 <= gen_num < len(SAMPLE_RUN.generations):
        return SAMPLE_RUN.generations[gen_num]
    raise ValueError(f"Generation {gen_num} not found")


def get_dna_bundle(candidate_id: str) -> DNABundle:
    """Get DNA bundle for a candidate."""
    for gen in SAMPLE_RUN.generations:
        for cand in gen.candidates:
            if cand.id == candidate_id:
                return generate_dna_bundle(cand)
    raise ValueError(f"Candidate {candidate_id} not found")


def get_prompt_dna(candidate_id: str) -> PromptDNA:
    """Get prompt DNA for a candidate."""
    for gen in SAMPLE_RUN.generations:
        for cand in gen.candidates:
            if cand.id == candidate_id:
                return generate_prompt_dna(cand.generation, cand.id, cand.parent_ids)
    raise ValueError(f"Candidate {candidate_id} not found")
