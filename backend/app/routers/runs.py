"""
API endpoints for GAD runs
"""
from fastapi import APIRouter, HTTPException
from app.models import Run, RunWithGenerations
from app.demo_data import get_run, SAMPLE_RUN

router = APIRouter()


@router.get("/runs", response_model=list[Run])
async def list_runs():
    """
    List all available GAD runs

    In a real implementation, this would query a database of runs.
    For demo purposes, returns the sample run.
    """
    return [SAMPLE_RUN]


@router.get("/runs/{run_id}", response_model=RunWithGenerations)
async def get_run_details(run_id: str):
    """
    Get complete details for a specific run including all generations

    In a real implementation, this would:
    - Query the run metadata from database
    - Load all generation data
    - Include final metrics and outcomes
    """
    run = get_run(run_id)
    if not run:
        raise HTTPException(status_code=404, detail=f"Run {run_id} not found")
    return run
