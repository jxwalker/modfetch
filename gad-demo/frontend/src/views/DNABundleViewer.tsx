import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/Card";
import { Badge } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { api } from "@/api/client";
import { Layers, Code, Cpu, Shield } from "lucide-react";

export function DNABundleViewer() {
  const [selectedCandidateId, setSelectedCandidateId] = useState<string | null>(null);
  const [activeLayer, setActiveLayer] = useState<"code" | "prompt" | "evaluator">("code");

  const { data: fullRun } = useQuery({
    queryKey: ["fullRun"],
    queryFn: () => api.getFullRun(),
  });

  const { data: dnaBundle } = useQuery({
    queryKey: ["dnaBundle", selectedCandidateId],
    queryFn: () => api.getDNABundle(selectedCandidateId!),
    enabled: !!selectedCandidateId,
  });

  if (!fullRun) {
    return <div>Loading...</div>;
  }

  // Get some candidates from generation 2
  const gen2Candidates = fullRun.generations[2]?.candidates.slice(0, 3) || [];
  const candidateToShow = gen2Candidates[0];

  if (!selectedCandidateId && candidateToShow) {
    setSelectedCandidateId(candidateToShow.id);
  }

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-3xl font-bold text-gray-900">DNA Bundle Viewer</h1>
        <p className="text-gray-600 mt-2">
          Explore the complete DNA bundle: code, prompt, and evaluator layers
        </p>
      </div>

      {/* Candidate Selector */}
      <Card>
        <CardContent className="pt-6">
          <div className="flex items-center space-x-2 flex-wrap gap-2">
            <span className="text-sm font-medium text-gray-700">
              Select Candidate:
            </span>
            {gen2Candidates.map((candidate) => (
              <Button
                key={candidate.id}
                variant={candidate.id === selectedCandidateId ? "primary" : "outline"}
                size="sm"
                onClick={() => setSelectedCandidateId(candidate.id)}
              >
                {candidate.id}
              </Button>
            ))}
          </div>
        </CardContent>
      </Card>

      {dnaBundle && (
        <>
          {/* DNA Bundle Overview */}
          <Card>
            <CardHeader>
              <CardTitle className="flex items-center">
                <Layers className="h-5 w-5 mr-2" />
                DNA Bundle Structure
              </CardTitle>
            </CardHeader>
            <CardContent>
              <div className="space-y-4">
                <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                  <InfoField label="Bundle ID" value={dnaBundle.id} />
                  <InfoField label="Candidate ID" value={dnaBundle.candidate_id} />
                  <InfoField label="Provenance Hash" value={dnaBundle.provenance_hash} />
                  <InfoField label="Timestamp" value={new Date(dnaBundle.timestamp).toLocaleString()} />
                </div>

                {dnaBundle.parent_hashes.length > 0 && (
                  <div>
                    <h4 className="text-sm font-semibold text-gray-700 mb-2">
                      Parent Hashes
                    </h4>
                    <div className="flex flex-wrap gap-2">
                      {dnaBundle.parent_hashes.map((hash, idx) => (
                        <Badge key={idx} variant="default">
                          {hash}
                        </Badge>
                      ))}
                    </div>
                  </div>
                )}

                <div className="bg-blue-50 border border-blue-200 rounded-lg p-4">
                  <h4 className="font-semibold text-blue-900 mb-2">
                    What is a DNA Bundle?
                  </h4>
                  <p className="text-sm text-blue-800">
                    A DNA Bundle is the complete genetic fingerprint of a candidate
                    solution. It contains three immutable layers: the code layer
                    (implementation), the prompt layer (instructions), and the evaluator
                    layer (assessment metadata). This ensures full reproducibility and
                    provenance tracking.
                  </p>
                </div>
              </div>
            </CardContent>
          </Card>

          {/* Layer Selector */}
          <Card>
            <CardContent className="pt-6">
              <div className="flex space-x-2">
                <Button
                  variant={activeLayer === "code" ? "primary" : "outline"}
                  onClick={() => setActiveLayer("code")}
                  className="flex-1"
                >
                  <Code className="h-4 w-4 mr-2" />
                  Code Layer
                </Button>
                <Button
                  variant={activeLayer === "prompt" ? "primary" : "outline"}
                  onClick={() => setActiveLayer("prompt")}
                  className="flex-1"
                >
                  <Cpu className="h-4 w-4 mr-2" />
                  Prompt Layer
                </Button>
                <Button
                  variant={activeLayer === "evaluator" ? "primary" : "outline"}
                  onClick={() => setActiveLayer("evaluator")}
                  className="flex-1"
                >
                  <Shield className="h-4 w-4 mr-2" />
                  Evaluator Layer
                </Button>
              </div>
            </CardContent>
          </Card>

          {/* Code Layer */}
          {activeLayer === "code" && (
            <Card>
              <CardHeader>
                <CardTitle className="flex items-center">
                  <Code className="h-5 w-5 mr-2" />
                  Code Layer
                </CardTitle>
              </CardHeader>
              <CardContent>
                <div className="space-y-4">
                  <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                    <InfoField label="Branch" value={dnaBundle.code_layer.branch} />
                    <InfoField label="Commit ID" value={dnaBundle.code_layer.commit_id} />
                    <InfoField
                      label="Files Changed"
                      value={dnaBundle.code_layer.files_changed.toString()}
                    />
                    <InfoField
                      label="Lines Added"
                      value={`+${dnaBundle.code_layer.lines_added}`}
                      valueClass="text-green-600"
                    />
                    <InfoField
                      label="Lines Removed"
                      value={`-${dnaBundle.code_layer.lines_removed}`}
                      valueClass="text-red-600"
                    />
                  </div>

                  <div>
                    <h4 className="text-sm font-semibold text-gray-700 mb-2">
                      Diff Summary
                    </h4>
                    <div className="bg-gray-50 border border-gray-200 rounded p-3 text-sm">
                      {dnaBundle.code_layer.diff_summary}
                    </div>
                  </div>

                  {dnaBundle.code_layer.diff_url && (
                    <div>
                      <a
                        href={dnaBundle.code_layer.diff_url}
                        target="_blank"
                        rel="noopener noreferrer"
                        className="text-blue-600 hover:text-blue-800 text-sm underline"
                      >
                        View full diff on GitHub →
                      </a>
                    </div>
                  )}
                </div>
              </CardContent>
            </Card>
          )}

          {/* Prompt Layer */}
          {activeLayer === "prompt" && (
            <Card>
              <CardHeader>
                <CardTitle className="flex items-center">
                  <Cpu className="h-5 w-5 mr-2" />
                  Prompt Layer (DNA)
                </CardTitle>
              </CardHeader>
              <CardContent>
                <div className="space-y-4">
                  <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                    <InfoField
                      label="Prompt ID"
                      value={dnaBundle.prompt_layer.id}
                    />
                    <InfoField
                      label="Generation"
                      value={dnaBundle.prompt_layer.generation.toString()}
                    />
                  </div>

                  <div>
                    <h4 className="text-sm font-semibold text-gray-700 mb-2">
                      System Prompt
                    </h4>
                    <div className="bg-purple-50 border border-purple-200 rounded p-3 text-sm font-mono">
                      {dnaBundle.prompt_layer.system_prompt}
                    </div>
                  </div>

                  <div>
                    <h4 className="text-sm font-semibold text-gray-700 mb-2">
                      Task Description
                    </h4>
                    <div className="bg-blue-50 border border-blue-200 rounded p-3 text-sm">
                      {dnaBundle.prompt_layer.task_description}
                    </div>
                  </div>

                  <div>
                    <h4 className="text-sm font-semibold text-gray-700 mb-2">
                      Constraints ({dnaBundle.prompt_layer.constraints.length})
                    </h4>
                    <ul className="space-y-1">
                      {dnaBundle.prompt_layer.constraints.map((constraint, idx) => (
                        <li key={idx} className="text-sm text-gray-700 flex items-start">
                          <span className="mr-2">•</span>
                          <span>{constraint}</span>
                        </li>
                      ))}
                    </ul>
                  </div>

                  <div className="grid grid-cols-2 gap-4">
                    <InfoField
                      label="Temperature"
                      value={dnaBundle.prompt_layer.temperature.toString()}
                    />
                    <InfoField
                      label="Top P"
                      value={dnaBundle.prompt_layer.top_p.toString()}
                    />
                  </div>

                  {dnaBundle.prompt_layer.mutations.length > 0 && (
                    <div>
                      <h4 className="text-sm font-semibold text-gray-700 mb-2">
                        Mutations ({dnaBundle.prompt_layer.mutations.length})
                      </h4>
                      <div className="space-y-2">
                        {dnaBundle.prompt_layer.mutations.map((mutation, idx) => (
                          <div
                            key={idx}
                            className="bg-green-50 border border-green-200 rounded p-2 text-sm"
                          >
                            <Badge variant="success" className="mb-1">
                              {mutation.type}
                            </Badge>
                            <p className="text-gray-700">{mutation.change}</p>
                          </div>
                        ))}
                      </div>
                    </div>
                  )}
                </div>
              </CardContent>
            </Card>
          )}

          {/* Evaluator Layer */}
          {activeLayer === "evaluator" && (
            <Card>
              <CardHeader>
                <CardTitle className="flex items-center">
                  <Shield className="h-5 w-5 mr-2" />
                  Evaluator Layer
                </CardTitle>
              </CardHeader>
              <CardContent>
                <div className="space-y-4">
                  <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                    <InfoField
                      label="Policy Version"
                      value={dnaBundle.evaluator_layer.policy_version}
                    />
                    <InfoField
                      label="Anti-Cheat Seed"
                      value={dnaBundle.evaluator_layer.anti_cheat_seed}
                    />
                    <InfoField
                      label="Merkle Root"
                      value={dnaBundle.evaluator_layer.merkle_root}
                    />
                  </div>

                  <div>
                    <h4 className="text-sm font-semibold text-gray-700 mb-3">
                      Reviewer Reliabilities
                    </h4>
                    <div className="space-y-2">
                      {Object.entries(dnaBundle.evaluator_layer.reviewer_reliabilities).map(
                        ([reviewerId, reliability]) => (
                          <div key={reviewerId} className="flex items-center justify-between">
                            <span className="text-sm text-gray-700">{reviewerId}</span>
                            <div className="flex items-center space-x-3">
                              <div className="w-32 bg-gray-200 rounded-full h-2">
                                <div
                                  className="bg-green-500 h-2 rounded-full"
                                  style={{ width: `${reliability * 100}%` }}
                                />
                              </div>
                              <span className="text-sm font-medium text-gray-900 w-12 text-right">
                                {(reliability * 100).toFixed(0)}%
                              </span>
                            </div>
                          </div>
                        )
                      )}
                    </div>
                  </div>

                  <div>
                    <h4 className="text-sm font-semibold text-gray-700 mb-3">
                      UCB Statistics
                    </h4>
                    <div className="space-y-3">
                      {dnaBundle.evaluator_layer.ucb_stats.map((ucb) => (
                        <div
                          key={ucb.agent_id}
                          className="bg-gray-50 border border-gray-200 rounded p-3"
                        >
                          <div className="flex items-center justify-between mb-2">
                            <span className="font-medium text-gray-900">{ucb.agent_id}</span>
                            <span className="text-lg font-bold text-blue-600">
                              {ucb.total_score.toFixed(2)}
                            </span>
                          </div>
                          <div className="grid grid-cols-2 gap-2 text-xs text-gray-600">
                            <div>Mean Reward: {ucb.mean_reward.toFixed(2)}</div>
                            <div>Times Selected: {ucb.times_selected}</div>
                          </div>
                        </div>
                      ))}
                    </div>
                  </div>

                  <div className="bg-purple-50 border border-purple-200 rounded-lg p-4">
                    <h4 className="font-semibold text-purple-900 mb-2">
                      Evaluator Layer Purpose
                    </h4>
                    <p className="text-sm text-purple-800">
                      The evaluator layer captures the assessment context: reviewer
                      reliability scores, UCB allocation statistics, policy version, and
                      cryptographic proofs (merkle root, anti-cheat seed). This ensures
                      evaluations are reproducible and tamper-proof.
                    </p>
                  </div>
                </div>
              </CardContent>
            </Card>
          )}
        </>
      )}
    </div>
  );
}

interface InfoFieldProps {
  label: string;
  value: string;
  valueClass?: string;
}

function InfoField({ label, value, valueClass = "" }: InfoFieldProps) {
  return (
    <div>
      <h4 className="text-sm font-semibold text-gray-700 mb-1">{label}</h4>
      <div className={`bg-gray-50 border border-gray-200 rounded px-3 py-2 text-sm ${valueClass}`}>
        {value}
      </div>
    </div>
  );
}
