"""
FastAPI backend for the GAD demo system.
Serves mock data endpoints for the frontend.
"""

from fastapi import FastAPI, HTTPException
from fastapi.middleware.cors import CORSMiddleware
from app.models import GADRun, Generation, DNABundle, PromptDNA, RepositoryPlanningGraph
from app.mock_data import (
    get_sample_run,
    get_generation,
    get_dna_bundle,
    get_prompt_dna,
    SAMPLE_RUN
)

app = FastAPI(
    title="GAD Demo API",
    description="Backend API for the Generative Adversarial Development demo system",
    version="1.0.0"
)

# Configure CORS
app.add_middleware(
    CORSMiddleware,
    allow_origins=["http://localhost:5173", "http://localhost:3000"],  # Vite default port
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)


@app.get("/")
def read_root():
    """Health check endpoint."""
    return {
        "status": "ok",
        "message": "GAD Demo API is running",
        "version": "1.0.0"
    }


@app.get("/api/run/sample", response_model=GADRun)
def get_run():
    """
    Get the complete sample GAD run.
    Returns all generations, candidates, agents, and RPG.
    """
    return get_sample_run()


@app.get("/api/run/sample/generation/{gen_num}", response_model=Generation)
def get_generation_data(gen_num: int):
    """
    Get a specific generation by number.

    Args:
        gen_num: Generation number (0-4 for the sample run)

    Returns:
        Complete generation data including candidates, Pareto front, and UCB allocations
    """
    try:
        return get_generation(gen_num)
    except ValueError as e:
        raise HTTPException(status_code=404, detail=str(e))


@app.get("/api/run/sample/dna/{candidate_id}", response_model=DNABundle)
def get_dna_bundle_data(candidate_id: str):
    """
    Get the complete DNA bundle for a candidate.

    Args:
        candidate_id: Candidate ID (e.g., "gen0-cand0")

    Returns:
        DNA bundle with code, prompt, and evaluator layers
    """
    try:
        return get_dna_bundle(candidate_id)
    except ValueError as e:
        raise HTTPException(status_code=404, detail=str(e))


@app.get("/api/run/sample/prompt/{candidate_id}", response_model=PromptDNA)
def get_prompt_dna_data(candidate_id: str):
    """
    Get the prompt DNA for a candidate.

    Args:
        candidate_id: Candidate ID (e.g., "gen0-cand0")

    Returns:
        Prompt DNA object with instructions and mutations
    """
    try:
        return get_prompt_dna(candidate_id)
    except ValueError as e:
        raise HTTPException(status_code=404, detail=str(e))


@app.get("/api/run/sample/rpg", response_model=RepositoryPlanningGraph)
def get_rpg():
    """
    Get the Repository Planning Graph.

    Returns:
        Complete RPG with nodes and edges
    """
    return SAMPLE_RUN.rpg


@app.get("/api/run/sample/summary")
def get_run_summary():
    """
    Get a summary of the GAD run.

    Returns:
        High-level statistics and overview
    """
    run = get_sample_run()

    total_candidates = sum(len(gen.candidates) for gen in run.generations)
    total_survivors = sum(len(gen.survivors) for gen in run.generations)

    final_gen = run.generations[-1]
    final_survivor = None
    if final_gen.survivors:
        final_survivor_id = final_gen.survivors[0]
        for cand in final_gen.candidates:
            if cand.id == final_survivor_id:
                final_survivor = {
                    "id": cand.id,
                    "effective_score": cand.effective_score,
                    "gates_passed": cand.gates_passed
                }
                break

    return {
        "run_id": run.id,
        "name": run.name,
        "total_generations": run.total_generations,
        "total_candidates": total_candidates,
        "total_survivors": total_survivors,
        "final_survivor": final_survivor,
        "requirement": run.requirement
    }


if __name__ == "__main__":
    import uvicorn
    uvicorn.run(app, host="0.0.0.0", port=8000)
