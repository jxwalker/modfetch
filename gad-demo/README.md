# GAD Demo - Generative Adversarial Development

An interactive web-based demonstration of the **Generative Adversarial Development (GAD)** system, designed for patent examiners and technical audiences.

## Overview

GAD is an autonomous multi-agent software development engine that works like an adversarial coding loop:

- **Generator Agents** produce code candidates from prompt DNA instructions
- **Reviewer Agents** analyze each candidate for security, quality, performance, and UX
- **CI Pipeline** builds, tests, and scans each candidate
- **Scoring Engine** aggregates metrics with weighted scoring and hard gates
- **Selection Engine** applies Pareto diversity and UCB-based resource allocation
- **Breeding Engine** evolves prompts through crossover, mutation, and feedback shaping

The system iterates over multiple generations until a fully compliant solution emerges.

## Features

### 8 Interactive Views

1. **Overview** - Hero section with GAD pipeline diagram
2. **Loop Explorer** - Step-through multi-generation simulation
3. **Agents & Scoring** - Generator/reviewer profiles and composite scoring
4. **Selection & GEPA** - Pareto plot and UCB allocation visualization
5. **Prompt DNA & Trust Regions** - Diff viewer and mutation tracking
6. **DNA Bundle** - Three-layer bundle explorer (code, prompt, evaluator)
7. **Repository Planning Graph** - Visual graph of architecture
8. **Examiner Script** - Built-in presentation walkthrough

### Technical Stack

**Frontend:**
- React 18 with TypeScript
- Vite for fast development
- TailwindCSS for styling
- React Router for navigation
- TanStack Query for data fetching
- Recharts for data visualization

**Backend:**
- FastAPI with Python 3.11
- Pydantic for data validation
- Comprehensive mock data for 5-generation run
- RESTful API with automatic documentation

## Quick Start

### Option 1: Development Mode (Recommended)

**Requirements:**
- Python 3.11+
- Node.js 18+
- npm or yarn

**Run:**
```bash
cd gad-demo
./run.sh
```

The script will:
1. Create Python virtual environment
2. Install backend dependencies
3. Install frontend dependencies
4. Start backend on http://localhost:8000
5. Start frontend on http://localhost:5173

**Access:**
- Frontend: http://localhost:5173
- Backend API: http://localhost:8000
- API Documentation: http://localhost:8000/docs

### Option 2: Docker

**Requirements:**
- Docker
- Docker Compose

**Run:**
```bash
cd gad-demo
./run-docker.sh
```

**Access:**
- Frontend: http://localhost:5173
- Backend API: http://localhost:8000
- API Documentation: http://localhost:8000/docs

**Stop:**
```bash
docker-compose down
```

## Manual Setup

### Backend Setup

```bash
cd gad-demo/backend

# Create virtual environment
python3 -m venv venv
source venv/bin/activate  # On Windows: venv\Scripts\activate

# Install dependencies
pip install -r requirements.txt

# Run server
uvicorn app.main:app --reload --port 8000
```

### Frontend Setup

```bash
cd gad-demo/frontend

# Install dependencies
npm install

# Run development server
npm run dev

# Build for production
npm run build
```

## Project Structure

```
gad-demo/
├── backend/
│   ├── app/
│   │   ├── main.py           # FastAPI application
│   │   ├── models.py         # Pydantic models
│   │   └── mock_data.py      # Mock data generator
│   ├── requirements.txt
│   └── Dockerfile
├── frontend/
│   ├── src/
│   │   ├── components/       # Reusable UI components
│   │   ├── views/            # 8 main views
│   │   ├── api/              # API client
│   │   ├── types/            # TypeScript types
│   │   ├── lib/              # Utilities
│   │   ├── App.tsx           # Main app with routing
│   │   └── main.tsx          # Entry point
│   ├── package.json
│   ├── vite.config.ts
│   ├── tailwind.config.js
│   └── Dockerfile
├── docker-compose.yml
├── run.sh                    # Development run script
├── run-docker.sh             # Docker run script
└── README.md
```

## API Endpoints

The backend provides the following REST endpoints:

- `GET /` - Health check
- `GET /api/run/sample` - Complete GAD run with all generations
- `GET /api/run/sample/generation/{n}` - Specific generation data
- `GET /api/run/sample/dna/{candidate_id}` - DNA bundle for candidate
- `GET /api/run/sample/prompt/{candidate_id}` - Prompt DNA for candidate
- `GET /api/run/sample/rpg` - Repository Planning Graph
- `GET /api/run/sample/summary` - Run summary statistics

Full API documentation available at http://localhost:8000/docs

## Key Concepts

### Prompt DNA
Genetic instructions for code generation that evolve through:
- Feedback integration from reviewers
- Constraint refinement based on failures
- Mutation within trust region boundaries
- Crossover between successful parents

### Hard Gates
Binary pass/fail thresholds that candidates must satisfy:
- Minimum test pass rate (≥80%)
- Security threshold (≥70/100)
- Zero critical vulnerabilities
- License compliance

### Pareto Optimality
Multi-objective selection where candidates on the Pareto front cannot be improved in one dimension without degrading another. This maintains solution diversity.

### GEPA (Generator-Evaluator Planning Allocation)
Resource allocation strategy using:
- **UCB (Upper Confidence Bound)** for balancing exploitation vs exploration
- **EIG (Expected Information Gain)** for identifying informative experiments

### Trust Regions
Constraints that prevent catastrophic mutations:
- Mutations must maintain ≥75% similarity to successful parents
- Out-of-bounds mutations are projected back to boundary
- Prevents forgetting successful patterns

### DNA Bundle
Three-layer immutable record for each candidate:
1. **Code Layer** - Implementation, diffs, commit info
2. **Prompt Layer** - Instructions, constraints, mutations
3. **Evaluator Layer** - Reviewer reliability, UCB stats, proofs

## Mock Data

The demo includes realistic mock data for a 5-generation run implementing a JWT authentication API:

- **40 total candidates** across 5 generations
- **3 generator agents** with different specializations
- **4 reviewer agents** (security, performance, UX, quality)
- **8-10 candidates per generation** with varying quality
- **2-3 survivors per generation** selected for breeding
- **Realistic metrics** including tests, security scores, vulnerabilities
- **Evolution tracking** showing improvement over generations

## Presentation Guide

The **Examiner Script** view provides a complete walkthrough for presenting GAD:

1. **Introduction** (2 min) - Overview and core concept
2. **Pipeline** (3 min) - Explain each stage
3. **Live Demo** (4 min) - Step through generations
4. **Agents & Scoring** (3 min) - Show agent profiles and scoring
5. **Selection** (3 min) - Explain Pareto and GEPA
6. **Prompt DNA** (2 min) - Show evolution and trust regions
7. **DNA Bundle** (2 min) - Explore three-layer structure
8. **Q&A** (1 min) - Closing and questions

**Total time:** 15-20 minutes

## Development

### Frontend Development

```bash
cd frontend
npm run dev          # Start dev server
npm run build        # Build for production
npm run preview      # Preview production build
npm run lint         # Run linter
```

### Backend Development

```bash
cd backend
source venv/bin/activate
uvicorn app.main:app --reload  # Start with auto-reload
python -m pytest              # Run tests (if added)
```

### Adding New Features

1. **New View** - Add to `frontend/src/views/` and register route in `App.tsx`
2. **New API Endpoint** - Add to `backend/app/main.py`
3. **New Mock Data** - Extend `backend/app/mock_data.py`
4. **New Component** - Add to `frontend/src/components/`

## Troubleshooting

### Backend won't start
- Ensure Python 3.11+ is installed: `python3 --version`
- Check if port 8000 is available: `lsof -i :8000`
- Verify dependencies: `pip install -r requirements.txt`

### Frontend won't start
- Ensure Node.js 18+ is installed: `node --version`
- Check if port 5173 is available: `lsof -i :5173`
- Clear cache: `rm -rf node_modules && npm install`

### API requests fail
- Verify backend is running: http://localhost:8000
- Check CORS settings in `backend/app/main.py`
- Inspect browser console for errors

### Docker issues
- Ensure Docker is running: `docker ps`
- Rebuild containers: `docker-compose up --build`
- View logs: `docker-compose logs -f`

## Production Deployment

### Frontend
```bash
cd frontend
npm run build
# Deploy dist/ folder to static hosting (Vercel, Netlify, S3, etc.)
```

### Backend
```bash
cd backend
# Use Dockerfile or deploy to:
# - AWS Lambda with Mangum
# - Google Cloud Run
# - Heroku
# - Any container platform
```

### Environment Variables

**Frontend:**
- `VITE_API_BASE_URL` - Backend API URL (default: `/api`)

**Backend:**
- No environment variables required for demo

## License

This is a demonstration system for patent examination purposes.

## Support

For questions or issues:
1. Check the Examiner Script view for presentation guidance
2. Review API documentation at `/docs` endpoint
3. Inspect browser console for frontend errors
4. Check backend logs for API errors

## Acknowledgments

Built with:
- React, TypeScript, Vite
- FastAPI, Pydantic
- TailwindCSS
- Recharts
- TanStack Query

---

**Note:** This is a simulation system with mock data. All generated data is deterministic and designed for demonstration purposes. To integrate with real GAD pipelines, replace mock data endpoints with actual API calls to your GAD infrastructure.
