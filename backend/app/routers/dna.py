"""
API endpoints for DNA bundles
"""
from fastapi import APIRouter, HTTPException
from app.models import DNABundle
from app.demo_data import get_dna_bundle

router = APIRouter()


@router.get("/runs/{run_id}/dna/{line_id}", response_model=DNABundle)
async def get_dna_bundle_details(run_id: str, line_id: str):
    """
    Get DNA bundle for a specific candidate lineage

    The DNA bundle contains:
    - Prompt DNA: heritable prompt configuration
    - Feedback summary: accumulated review feedback
    - Evidence metrics: test and evaluation results
    - State snapshots: selector, evaluator, and policy states
    - Provenance: lineage and mutation history

    In a real implementation, this would:
    - Query the DNA bundle from persistent storage
    - Reconstruct lineage history
    - Load associated feedback and metrics
    """
    bundle = get_dna_bundle(run_id, line_id)
    if not bundle:
        raise HTTPException(
            status_code=404,
            detail=f"DNA bundle {line_id} not found in run {run_id}"
        )
    return bundle
