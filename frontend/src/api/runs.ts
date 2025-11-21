/**
 * API functions for GAD runs
 */

import { fetchAPI } from './client';
import { Run, RunWithGenerations, Generation, DNABundle, RPG } from '../types';

export async function getRuns(): Promise<Run[]> {
  return fetchAPI<Run[]>('/runs');
}

export async function getRun(runId: string): Promise<RunWithGenerations> {
  return fetchAPI<RunWithGenerations>(`/runs/${runId}`);
}

export async function getGeneration(runId: string, generationNumber: number): Promise<Generation> {
  return fetchAPI<Generation>(`/runs/${runId}/generations/${generationNumber}`);
}

export async function getDNABundle(runId: string, lineId: string): Promise<DNABundle> {
  return fetchAPI<DNABundle>(`/runs/${runId}/dna/${lineId}`);
}

export async function getRPG(runId: string): Promise<RPG> {
  return fetchAPI<RPG>(`/runs/${runId}/rpg`);
}
