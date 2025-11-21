"""
FastAPI backend for GAD System Demo
Serves demonstration data for the Generative Adversarial Development system
"""
from fastapi import FastAPI
from fastapi.middleware.cors import CORSMiddleware
from app.routers import runs, generations, dna, rpg

app = FastAPI(
    title="GAD System Demo API",
    description="API for Generative Adversarial Development System Demonstration",
    version="1.0.0"
)

# CORS middleware for frontend communication
app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],  # In production, specify frontend URL
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)

# Include routers
app.include_router(runs.router, prefix="/api", tags=["runs"])
app.include_router(generations.router, prefix="/api", tags=["generations"])
app.include_router(dna.router, prefix="/api", tags=["dna"])
app.include_router(rpg.router, prefix="/api", tags=["rpg"])


@app.get("/")
async def root():
    """Health check endpoint"""
    return {
        "status": "healthy",
        "service": "GAD System Demo API",
        "version": "1.0.0"
    }


@app.get("/health")
async def health():
    """Health check endpoint"""
    return {"status": "healthy"}
