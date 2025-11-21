import { BrowserRouter as Router, Routes, Route, NavLink } from 'react-router-dom';
import './App.css';

// Pages
import OverviewPage from './pages/OverviewPage';
import LoopExplorerPage from './pages/LoopExplorerPage';
import AgentsPage from './pages/AgentsPage';
import SelectionPage from './pages/SelectionPage';
import DnaBundlePage from './pages/DnaBundlePage';
import RpgPage from './pages/RpgPage';
import ExaminerScriptPage from './pages/ExaminerScriptPage';

function App() {
  return (
    <Router>
      <div className="app">
        <nav className="nav">
          <div className="nav-content">
            <NavLink to="/" className="nav-brand">
              GAD System Demo
            </NavLink>
            <ul className="nav-links">
              <li>
                <NavLink to="/" className={({ isActive }) => isActive ? 'nav-link active' : 'nav-link'}>
                  Overview
                </NavLink>
              </li>
              <li>
                <NavLink to="/loop" className={({ isActive }) => isActive ? 'nav-link active' : 'nav-link'}>
                  Loop Explorer
                </NavLink>
              </li>
              <li>
                <NavLink to="/agents" className={({ isActive }) => isActive ? 'nav-link active' : 'nav-link'}>
                  Agents & Scoring
                </NavLink>
              </li>
              <li>
                <NavLink to="/selection" className={({ isActive }) => isActive ? 'nav-link active' : 'nav-link'}>
                  Selection & GEPA
                </NavLink>
              </li>
              <li>
                <NavLink to="/dna" className={({ isActive }) => isActive ? 'nav-link active' : 'nav-link'}>
                  DNA Bundle
                </NavLink>
              </li>
              <li>
                <NavLink to="/rpg" className={({ isActive }) => isActive ? 'nav-link active' : 'nav-link'}>
                  RPG
                </NavLink>
              </li>
              <li>
                <NavLink to="/script" className={({ isActive }) => isActive ? 'nav-link active' : 'nav-link'}>
                  Demo Script
                </NavLink>
              </li>
            </ul>
          </div>
        </nav>

        <main className="main-content">
          <Routes>
            <Route path="/" element={<OverviewPage />} />
            <Route path="/loop" element={<LoopExplorerPage />} />
            <Route path="/agents" element={<AgentsPage />} />
            <Route path="/selection" element={<SelectionPage />} />
            <Route path="/dna" element={<DnaBundlePage />} />
            <Route path="/rpg" element={<RpgPage />} />
            <Route path="/script" element={<ExaminerScriptPage />} />
          </Routes>
        </main>
      </div>
    </Router>
  );
}

export default App;
