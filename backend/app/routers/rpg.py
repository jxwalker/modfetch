"""
API endpoints for Repository Planning Graph
"""
from fastapi import APIRouter, HTTPException
from app.models import RPG, RPGNode
from app.demo_data import get_rpg

router = APIRouter()


@router.get("/runs/{run_id}/rpg", response_model=RPG)
async def get_rpg_data(run_id: str):
    """
    Get Repository Planning Graph for a run

    The RPG maintains architectural coherence by tracking:
    - Capabilities, modules, files, functions, and tests as nodes
    - Dependencies, calls, and test relationships as edges
    - Implementation status and generation history

    In a real implementation, this would:
    - Parse the codebase structure
    - Extract dependencies and call graphs
    - Track changes across generations
    - Update node statuses based on CI results
    """
    rpg = get_rpg(run_id)
    if not rpg:
        raise HTTPException(
            status_code=404,
            detail=f"RPG not found for run {run_id}"
        )
    return rpg


@router.get("/runs/{run_id}/rpg/nodes/{node_id}", response_model=RPGNode)
async def get_rpg_node_details(run_id: str, node_id: str):
    """
    Get detailed information about a specific RPG node

    In a real implementation, this would:
    - Query node metadata
    - Load associated code and tests
    - Return generation history for this node
    """
    rpg = get_rpg(run_id)
    if not rpg:
        raise HTTPException(
            status_code=404,
            detail=f"RPG not found for run {run_id}"
        )

    node = next((n for n in rpg.nodes if n.id == node_id), None)
    if not node:
        raise HTTPException(
            status_code=404,
            detail=f"Node {node_id} not found in RPG"
        )

    return node
