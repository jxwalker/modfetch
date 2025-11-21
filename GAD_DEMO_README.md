# Generative Adversarial Development (GAD) System - Interactive Demo

**Version 1.0.0**
**Author: James Walker, DXC Technology**

This is an interactive demonstration of the Generative Adversarial Development (GAD) system, designed for patent examiners and technically literate reviewers. The demo illustrates the key innovations in GAD through a polished web application with scripted data.

---

## Overview

GAD is an autonomous multi-agent coding loop that evolves AI prompts and code until all tests, policies, and user acceptance checks pass. This demo application provides:

1. **Interactive visual model** of the GAD loop and agents
2. **Simulated GAD runs** over several generations using scripted data
3. **Step-through capability** from requirement to final solution
4. **Clear highlighting** of the core inventions

### Core Innovations

1. **Multi-agent generative + adversarial loop** - Multiple specialized generator agents plus adversarial reviewers
2. **Composite scoring with hard gates** - Two-tier evaluation system (binary gates + weighted scores)
3. **GEPA selection & Pareto front diversity** - Preserves diversity while selecting optimal solutions
4. **Prompt DNA evolution with trust regions** - Evolution at the prompt level, not code level
5. **Agent economics (UCB & Expected Info Gain)** - Intelligent resource allocation
6. **DNA bundle & Repository Planning Graph** - Complete provenance and architectural coherence

---

## Architecture

### Technology Stack

**Backend**
- Python 3.11+
- FastAPI - Modern async web framework
- Pydantic - Data validation and modeling

**Frontend**
- TypeScript
- React 18
- Vite - Fast build tool
- Recharts - Visualization library
- React Router - Navigation

### Project Structure

```
.
├── backend/
│   ├── app/
│   │   ├── main.py              # FastAPI application
│   │   ├── models.py            # Pydantic data models
│   │   ├── demo_data.py         # Scripted demo data
│   │   └── routers/             # API endpoints
│   │       ├── runs.py
│   │       ├── generations.py
│   │       ├── dna.py
│   │       └── rpg.py
│   ├── requirements.txt
│   └── Dockerfile
├── frontend/
│   ├── src/
│   │   ├── main.tsx             # Application entry
│   │   ├── App.tsx              # Main app component
│   │   ├── types.ts             # TypeScript types
│   │   ├── api/                 # API client
│   │   ├── components/          # Reusable components
│   │   │   ├── DiagramGadLoop.tsx
│   │   │   ├── CandidateCard.tsx
│   │   │   ├── MetricBar.tsx
│   │   │   └── ParetoPlot.tsx
│   │   └── pages/               # Main views
│   │       ├── OverviewPage.tsx
│   │       ├── LoopExplorerPage.tsx
│   │       ├── AgentsPage.tsx
│   │       ├── SelectionPage.tsx
│   │       ├── DnaBundlePage.tsx
│   │       ├── RpgPage.tsx
│   │       └── ExaminerScriptPage.tsx
│   ├── package.json
│   └── Dockerfile
├── docker-compose.yml
├── Makefile.demo
├── start-demo.sh
└── GAD_DEMO_README.md (this file)
```

---

## Prerequisites

### Option 1: Docker (Recommended for Quick Start)

- Docker 20.10+
- Docker Compose 2.0+

### Option 2: Local Development

- Python 3.11+
- Node.js 18+
- npm 9+

---

## Quick Start

### Using Docker (Easiest)

```bash
# Build and start containers
docker-compose up -d

# The demo will be available at:
# Frontend: http://localhost:3000
# Backend API: http://localhost:8000
# API Documentation: http://localhost:8000/docs
```

To stop:
```bash
docker-compose down
```

### Using the Startup Script

```bash
# Make the script executable
chmod +x start-demo.sh

# Run the demo
./start-demo.sh
```

Press Ctrl+C to stop both servers.

### Using Makefile

```bash
# Install dependencies
make -f Makefile.demo install

# Start development servers
make -f Makefile.demo dev

# Or use Docker
make -f Makefile.demo docker-up
```

### Manual Setup

**Backend:**
```bash
cd backend
pip install -r requirements.txt
uvicorn app.main:app --reload --port 8000
```

**Frontend (in a new terminal):**
```bash
cd frontend
npm install
npm run dev
```

---

## Demo Walkthrough

### For Patent Examiners

Follow the guided demo script at: **http://localhost:3000/script**

The script provides:
- Step-by-step instructions (30-40 minute full demo)
- Key talking points for each section
- Innovation highlights to emphasize
- Common Q&A

### Quick Tour (10 minutes)

1. **Overview** (`/`) - High-level introduction to GAD (3 min)
2. **Loop Explorer** (`/loop`) - See generation-by-generation evolution (4 min)
3. **DNA Bundle** (`/dna`) - Core innovation: prompt DNA evolution (3 min)

### Comprehensive Tour (30-40 minutes)

1. **Overview** (`/`) - Introduction and system architecture
2. **Loop Explorer** (`/loop`) - Generation progression
3. **Agents & Scoring** (`/agents`) - Multi-agent system and composite evaluation
4. **Selection & GEPA** (`/selection`) - Pareto optimization and agent economics
5. **DNA Bundle** (`/dna`) - Prompt DNA structure and evolution
6. **RPG** (`/rpg`) - Architectural coherence
7. **Demo Script** (`/script`) - Guided walkthrough for presenters

---

## Key Views Explained

### 1. Overview Page (`/`)

**Purpose:** Give non-specialists a 3-minute understanding of GAD

**Features:**
- Hero section with clear value proposition
- GAD loop diagram showing all stages
- Cards explaining each component
- Summary of 6 core innovations

**Best for:** First-time viewers, executives, initial patent review

---

### 2. Loop Explorer (`/loop`)

**Purpose:** Show how GAD evolves solutions over generations

**Features:**
- Generation timeline selector
- Detailed candidate metrics
- Pareto front visualization
- Evolution insights between generations

**Demo scenario:** Password reset feature, 4 generations
- Gen 1: Initial diverse exploration (2 failed gates, 2 survivors)
- Gen 2: First evolution (feedback integration)
- Gen 3: Convergence (all passed gates)
- Gen 4: Final refinement (near-optimal solution)

**Best for:** Understanding the evolutionary process

---

### 3. Agents & Scoring (`/agents`)

**Purpose:** Make multi-agent and scoring concrete

**Features:**
- Generator agent profiles and specializations
- Reviewer agent types and reliability scores
- Composite scoring explanation with weights
- Hard gates vs. soft scores
- Example candidate scoring breakdown

**Key innovations highlighted:**
- Multi-agent generation (not single LLM)
- Adversarial review with reliability tracking
- Two-tier evaluation system

**Best for:** Understanding quality control mechanisms

---

### 4. Selection & GEPA (`/selection`)

**Purpose:** Show Pareto optimization and agent economics

**Features:**
- Interactive Pareto scatter plots (2D projections)
- Selection decisions table with UCB scores
- GEPA algorithm explanation
- Children allocation based on expected information gain

**Key innovations highlighted:**
- Pareto-optimal selection (not just "best")
- UCB and exploration/exploitation tradeoff
- Resource allocation via agent economics

**Best for:** Understanding selection strategy and diversity preservation

---

### 5. DNA Bundle (`/dna`)

**Purpose:** Expose prompt-level evolution innovation

**Features:**
- Complete Prompt DNA structure display
- System preamble, exemplars, persona vectors
- Trust region constraints
- Feedback integration mechanism
- Multi-layered DNA bundle with provenance

**Key innovations highlighted:**
- **Evolution at the prompt level** (core innovation)
- Trust regions for controlled mutation
- Complete hereditary package with lineage

**Best for:** Understanding the fundamental mechanism of GAD

---

### 6. RPG - Repository Planning Graph (`/rpg`)

**Purpose:** Explain architectural coherence mechanism

**Features:**
- Interactive graph visualization
- Node details (capabilities, modules, files, functions, tests)
- Relationship explorer
- Generation tracking per node

**Key innovations highlighted:**
- Long-horizon architectural memory
- Dependency tracking and constraint generation
- How GAD prevents architectural drift

**Best for:** Understanding scalability to large codebases

---

### 7. Demo Script (`/script`)

**Purpose:** Guided walkthrough for presenters

**Features:**
- 8-step demo script with timing
- Actions to take on each page
- Talking points and key emphases
- Common Q&A with answers
- Tips for patent examiners

**Best for:** Preparing for live demos or patent hearings

---

## Demo Data

The application uses **deterministic scripted data** to ensure consistent demonstrations. All data is defined in `backend/app/demo_data.py`.

### Sample Run: Password Reset Feature

- **Requirement:** Implement secure password reset with email verification
- **Generations:** 4
- **Total Candidates:** 16 (4, 5, 4, 3 per generation)
- **Final Status:** Success (near-optimal solution achieved)

### Agents

**Generators:**
- Security-First Generator
- Performance-Optimized Generator
- UX-Centered Generator

**Reviewers:**
- Security Reviewer (reliability: 92%)
- Style Reviewer (reliability: 88%)
- Performance Reviewer (reliability: 85%)
- UX Reviewer (reliability: 90%)
- License Reviewer (reliability: 95%)

### Metrics Tracked

- Test Pass Rate
- Code Coverage
- Security Score
- Performance Score
- UX Score
- Functionality Score
- Style Compliance

---

## API Endpoints

The FastAPI backend exposes the following endpoints (all under `/api`):

### Runs
- `GET /runs` - List all runs
- `GET /runs/{run_id}` - Get complete run with all generations

### Generations
- `GET /runs/{run_id}/generations/{n}` - Get specific generation details

### DNA Bundles
- `GET /runs/{run_id}/dna/{line_id}` - Get DNA bundle for a lineage

### RPG
- `GET /runs/{run_id}/rpg` - Get Repository Planning Graph
- `GET /runs/{run_id}/rpg/nodes/{node_id}` - Get specific node details

**Interactive API Documentation:** http://localhost:8000/docs

---

## Extension Points for Real Implementation

This demo uses mock data. To connect a real GAD engine:

### Backend Changes

1. **Replace `demo_data.py`** with database connections
2. **Implement real LLM calls** in generator endpoints
3. **Add CI/CD integration** for test execution
4. **Connect to code repository** for RPG parsing
5. **Add authentication** for multi-user support

### Key Integration Points (marked in code with comments)

```python
# In app/routers/runs.py
# REAL IMPLEMENTATION: Query database of runs
run = get_run(run_id)

# In app/demo_data.py
# REAL IMPLEMENTATION: Generate candidates using LLM agents
candidates = generator_agents.generate(prompt_dna)

# REAL IMPLEMENTATION: Run CI pipeline
test_results = ci_runner.execute(candidate.code)

# REAL IMPLEMENTATION: Execute reviewer agents
reviews = reviewer_agents.review(candidate.code)

# REAL IMPLEMENTATION: Parse codebase for RPG
rpg = code_parser.extract_graph(repository)
```

---

## Troubleshooting

### Backend Issues

**Error: `ModuleNotFoundError: No module named 'app'`**
```bash
# Ensure you're in the backend directory
cd backend
python -m pip install -r requirements.txt
```

**Error: Port 8000 already in use**
```bash
# Change port in start command
uvicorn app.main:app --port 8001
```

### Frontend Issues

**Error: `Cannot find module 'react'`**
```bash
# Install dependencies
cd frontend
npm install
```

**Error: Port 3000 already in use**
```bash
# Vite will automatically use next available port (3001, 3002, etc.)
```

### Docker Issues

**Error: `Cannot connect to the Docker daemon`**
```bash
# Start Docker Desktop or Docker daemon
sudo systemctl start docker  # Linux
```

**Error: `port is already allocated`**
```bash
# Stop conflicting containers or change ports in docker-compose.yml
docker-compose down
```

---

## For Patent Examiners

### Key Novelty Areas

1. **Prompt DNA Evolution**
   - Prior art: Genetic programming evolves code directly
   - GAD novelty: Evolves prompts to LLMs, not code
   - Impact: Leverages LLM capabilities while evolving strategy

2. **Multi-Agent Generative-Adversarial Loop**
   - Prior art: Single LLM code generation
   - GAD novelty: Multiple generators + adversarial reviewers
   - Impact: Creates diversity and quality control

3. **GEPA Selection with Pareto Diversity**
   - Prior art: Fitness-based selection (greedy)
   - GAD novelty: Pareto-optimal with diversity preservation
   - Impact: Prevents premature convergence

4. **Repository Planning Graph**
   - Prior art: Stateless code generation
   - GAD novelty: Persistent architectural memory
   - Impact: Maintains coherence over long horizons

5. **Agent Economics**
   - Prior art: Equal resource allocation
   - GAD novelty: UCB-based allocation with info gain
   - Impact: Efficient use of expensive LLM calls

### Suggested Prior Art Search Areas

- Multi-agent systems for software development
- Genetic programming with LLMs
- Pareto-optimal evolutionary algorithms
- Prompt engineering and optimization
- Software architecture extraction and maintenance

### Key Differentiators from Prior Art

- **Not single-LLM:** Multiple specialized agents
- **Not code evolution:** Prompt DNA evolution
- **Not greedy selection:** Pareto diversity preservation
- **Not stateless:** RPG maintains architectural memory
- **Not ad-hoc:** Systematic agent economics
- **Not human-in-loop:** Fully autonomous until success

---

## Performance Notes

### Demo Performance

- **Backend startup:** ~2 seconds
- **Frontend build:** ~5 seconds
- **API response time:** <100ms (mock data)
- **Page load time:** <1 second

### Real Implementation Considerations

- **LLM calls:** 2-30 seconds per candidate
- **CI pipeline:** 30 seconds - 5 minutes per candidate
- **Generation cycle:** 5-30 minutes depending on parallelization
- **Database queries:** <100ms with proper indexing

---

## License

This demonstration code is provided for evaluation purposes only. The GAD system and its innovations are subject to patent protection.

© 2024 DXC Technology. All rights reserved.

---

## Support and Questions

For technical questions about the demo:
- Check the troubleshooting section above
- Review the code comments for implementation details
- Consult the Demo Script page within the application

For questions about the GAD system and patents:
- Contact: James Walker, DXC Technology

---

## Appendix: Running Offline

The demo is fully self-contained and can run completely offline:

1. Install all dependencies while online:
   ```bash
   make -f Makefile.demo install
   ```

2. Disconnect from network

3. Run the demo:
   ```bash
   ./start-demo.sh
   ```

All data is embedded in `backend/app/demo_data.py` - no external API calls are made.

---

## Version History

**v1.0.0** (2024)
- Initial release
- Complete implementation of all 7 views
- Scripted demo data for password reset feature
- Docker support
- Comprehensive documentation

---

**End of README**
