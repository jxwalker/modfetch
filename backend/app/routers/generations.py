"""
API endpoints for GAD generations
"""
from fastapi import APIRouter, HTTPException
from app.models import Generation
from app.demo_data import get_generation

router = APIRouter()


@router.get("/runs/{run_id}/generations/{generation_number}", response_model=Generation)
async def get_generation_details(run_id: str, generation_number: int):
    """
    Get detailed data for a specific generation within a run

    In a real implementation, this would:
    - Query generation metadata
    - Load all candidates with their metrics
    - Calculate Pareto front
    - Return selection decisions
    """
    generation = get_generation(run_id, generation_number)
    if not generation:
        raise HTTPException(
            status_code=404,
            detail=f"Generation {generation_number} not found in run {run_id}"
        )
    return generation
