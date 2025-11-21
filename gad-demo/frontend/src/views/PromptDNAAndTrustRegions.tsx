import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/Card";
import { Badge } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { api } from "@/api/client";
import { formatScore } from "@/lib/utils";
import { Code, GitCompare, Shield, AlertTriangle } from "lucide-react";

export function PromptDNAAndTrustRegions() {
  const [selectedCandidateId, setSelectedCandidateId] = useState<string | null>(null);

  const { data: fullRun } = useQuery({
    queryKey: ["fullRun"],
    queryFn: () => api.getFullRun(),
  });

  const { data: promptDNA } = useQuery({
    queryKey: ["promptDNA", selectedCandidateId],
    queryFn: () => api.getPromptDNA(selectedCandidateId!),
    enabled: !!selectedCandidateId,
  });

  if (!fullRun) {
    return <div>Loading...</div>;
  }

  // Get candidates from generation 2 and 3 for comparison
  const gen2Candidates = fullRun.generations[2]?.candidates.slice(0, 3) || [];
  const candidateToShow = gen2Candidates[0];

  if (!selectedCandidateId && candidateToShow) {
    setSelectedCandidateId(candidateToShow.id);
  }

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-3xl font-bold text-gray-900">
          Prompt DNA & Trust Regions
        </h1>
        <p className="text-gray-600 mt-2">
          Explore prompt evolution, mutations, and trust-region constraints
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

      {promptDNA && (
        <>
          {/* Trust Region Status */}
          <Card>
            <CardHeader>
              <CardTitle className="flex items-center">
                <Shield className="h-5 w-5 mr-2" />
                Trust Region Analysis
              </CardTitle>
            </CardHeader>
            <CardContent>
              <div className="space-y-4">
                {promptDNA.trust_region_similarity !== undefined && (
                  <div>
                    <div className="flex items-center justify-between mb-2">
                      <span className="text-sm font-medium text-gray-700">
                        Similarity to Parent
                      </span>
                      <span className="text-2xl font-bold text-gray-900">
                        {formatScore(promptDNA.trust_region_similarity * 100)}%
                      </span>
                    </div>
                    <div className="w-full bg-gray-200 rounded-full h-3">
                      <div
                        className={`h-3 rounded-full transition-all ${
                          promptDNA.trust_region_similarity >= 0.75
                            ? "bg-green-500"
                            : "bg-yellow-500"
                        }`}
                        style={{
                          width: `${promptDNA.trust_region_similarity * 100}%`,
                        }}
                      />
                    </div>
                    <div className="mt-2 flex items-center space-x-2">
                      {promptDNA.trust_region_similarity >= 0.75 ? (
                        <>
                          <Badge variant="success">Within Trust Region</Badge>
                          <span className="text-sm text-gray-600">
                            Mutation is conservative and safe
                          </span>
                        </>
                      ) : (
                        <>
                          <Badge variant="warning">Outside Trust Region</Badge>
                          <span className="text-sm text-gray-600">
                            Projected back to boundary
                          </span>
                        </>
                      )}
                    </div>
                  </div>
                )}

                <div className="bg-blue-50 border border-blue-200 rounded-lg p-4">
                  <div className="flex items-start">
                    <AlertTriangle className="h-5 w-5 text-blue-600 mt-0.5 mr-3 flex-shrink-0" />
                    <div>
                      <h4 className="font-semibold text-blue-900">
                        Trust Region Policy
                      </h4>
                      <p className="text-sm text-blue-800 mt-1">
                        Trust regions constrain how far prompt DNA can mutate from
                        successful parents. If a mutation is too radical (similarity
                        &lt; 75%), it is projected back to the trust region boundary.
                        This prevents catastrophic forgetting while allowing controlled
                        exploration.
                      </p>
                    </div>
                  </div>
                </div>
              </div>
            </CardContent>
          </Card>

          {/* Prompt DNA Viewer */}
          <Card>
            <CardHeader>
              <CardTitle className="flex items-center">
                <Code className="h-5 w-5 mr-2" />
                Complete Prompt DNA
              </CardTitle>
            </CardHeader>
            <CardContent>
              <div className="space-y-4">
                <DNAField label="ID" value={promptDNA.id} />
                <DNAField label="Generation" value={promptDNA.generation.toString()} />
                <DNAField
                  label="Parent IDs"
                  value={promptDNA.parent_ids.join(", ") || "None (initial generation)"}
                />

                <div>
                  <h4 className="text-sm font-semibold text-gray-700 mb-2">
                    System Prompt
                  </h4>
                  <div className="bg-gray-50 border border-gray-200 rounded p-3 text-sm font-mono">
                    {promptDNA.system_prompt}
                  </div>
                </div>

                <div>
                  <h4 className="text-sm font-semibold text-gray-700 mb-2">
                    Task Description
                  </h4>
                  <div className="bg-gray-50 border border-gray-200 rounded p-3 text-sm">
                    {promptDNA.task_description}
                  </div>
                </div>

                <div>
                  <h4 className="text-sm font-semibold text-gray-700 mb-2">
                    Constraints
                  </h4>
                  <ul className="space-y-1">
                    {promptDNA.constraints.map((constraint, idx) => (
                      <li key={idx} className="text-sm text-gray-700 flex items-start">
                        <span className="mr-2">•</span>
                        <span>{constraint}</span>
                      </li>
                    ))}
                  </ul>
                </div>

                <div className="grid grid-cols-2 gap-4">
                  <DNAField label="Temperature" value={promptDNA.temperature.toString()} />
                  <DNAField label="Top P" value={promptDNA.top_p.toString()} />
                </div>
              </div>
            </CardContent>
          </Card>

          {/* Mutations */}
          {promptDNA.mutations.length > 0 && (
            <Card>
              <CardHeader>
                <CardTitle className="flex items-center">
                  <GitCompare className="h-5 w-5 mr-2" />
                  Mutations Applied
                </CardTitle>
              </CardHeader>
              <CardContent>
                <div className="space-y-3">
                  {promptDNA.mutations.map((mutation, idx) => (
                    <div
                      key={idx}
                      className="border-l-4 border-green-500 bg-green-50 p-3 rounded"
                    >
                      <div className="flex items-center space-x-2 mb-1">
                        <Badge variant="success">{mutation.type}</Badge>
                      </div>
                      <p className="text-sm text-gray-800">{mutation.change}</p>
                    </div>
                  ))}
                </div>
              </CardContent>
            </Card>
          )}

          {/* Feedback History */}
          {promptDNA.feedback_history.length > 0 && (
            <Card>
              <CardHeader>
                <CardTitle>Feedback History</CardTitle>
              </CardHeader>
              <CardContent>
                <div className="space-y-2">
                  {promptDNA.feedback_history.map((feedback, idx) => (
                    <div
                      key={idx}
                      className="bg-yellow-50 border border-yellow-200 rounded p-3"
                    >
                      <p className="text-sm text-gray-800">{feedback}</p>
                    </div>
                  ))}
                </div>
              </CardContent>
            </Card>
          )}

          {/* Diff Viewer (Simulated) */}
          {promptDNA.parent_ids.length > 0 && (
            <Card>
              <CardHeader>
                <CardTitle className="flex items-center">
                  <GitCompare className="h-5 w-5 mr-2" />
                  Changes from Parent
                </CardTitle>
              </CardHeader>
              <CardContent>
                <div className="bg-gray-50 border border-gray-200 rounded overflow-hidden">
                  <div className="bg-gray-100 px-4 py-2 border-b border-gray-300">
                    <span className="text-xs font-mono text-gray-600">
                      Comparing {promptDNA.parent_ids[0]} → {promptDNA.id}
                    </span>
                  </div>
                  <div className="p-4 font-mono text-xs space-y-1">
                    <div className="text-gray-600">
                      @@ Task Description @@
                    </div>
                    <div className="bg-red-100 text-red-800 px-2 py-1">
                      - Implement a secure REST API endpoint for user authentication
                      with JWT tokens
                    </div>
                    <div className="bg-green-100 text-green-800 px-2 py-1">
                      + Implement a secure REST API endpoint for user authentication
                      with JWT tokens [Gen {promptDNA.generation} refinement]
                    </div>
                    <div className="mt-3 text-gray-600">
                      @@ Constraints @@
                    </div>
                    <div className="bg-green-100 text-green-800 px-2 py-1">
                      + All inputs must be validated and sanitized
                    </div>
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

interface DNAFieldProps {
  label: string;
  value: string;
}

function DNAField({ label, value }: DNAFieldProps) {
  return (
    <div>
      <h4 className="text-sm font-semibold text-gray-700 mb-1">{label}</h4>
      <div className="bg-gray-50 border border-gray-200 rounded px-3 py-2 text-sm">
        {value}
      </div>
    </div>
  );
}
