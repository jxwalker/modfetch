import { BrowserRouter as Router, Routes, Route } from "react-router-dom";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { Layout } from "./components/Layout";
import { Overview } from "./views/Overview";
import { LoopExplorer } from "./views/LoopExplorer";
import { AgentsAndScoring } from "./views/AgentsAndScoring";
import { SelectionAndGEPA } from "./views/SelectionAndGEPA";
import { PromptDNAAndTrustRegions } from "./views/PromptDNAAndTrustRegions";
import { DNABundleViewer } from "./views/DNABundleViewer";
import { RepositoryPlanningGraph } from "./views/RepositoryPlanningGraph";
import { ExaminerScript } from "./views/ExaminerScript";

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      refetchOnWindowFocus: false,
      retry: 1,
      staleTime: 5 * 60 * 1000, // 5 minutes
    },
  },
});

function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <Router>
        <Layout>
          <Routes>
            <Route path="/" element={<Overview />} />
            <Route path="/loop" element={<LoopExplorer />} />
            <Route path="/agents" element={<AgentsAndScoring />} />
            <Route path="/selection" element={<SelectionAndGEPA />} />
            <Route path="/prompt-dna" element={<PromptDNAAndTrustRegions />} />
            <Route path="/dna-bundle" element={<DNABundleViewer />} />
            <Route path="/rpg" element={<RepositoryPlanningGraph />} />
            <Route path="/script" element={<ExaminerScript />} />
          </Routes>
        </Layout>
      </Router>
    </QueryClientProvider>
  );
}

export default App;
