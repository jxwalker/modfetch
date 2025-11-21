/**
 * API client for the GAD demo backend
 */

import axios from 'axios';
import type {
  GADRun,
  Generation,
  DNABundle,
  PromptDNA,
  RepositoryPlanningGraph,
  RunSummary,
} from '@/types';

const API_BASE_URL = import.meta.env.VITE_API_BASE_URL || '/api';

const apiClient = axios.create({
  baseURL: API_BASE_URL,
  headers: {
    'Content-Type': 'application/json',
  },
});

export const api = {
  /**
   * Get the complete sample GAD run
   */
  getFullRun: async (): Promise<GADRun> => {
    const response = await apiClient.get<GADRun>('/run/sample');
    return response.data;
  },

  /**
   * Get a specific generation by number
   */
  getGeneration: async (genNum: number): Promise<Generation> => {
    const response = await apiClient.get<Generation>(`/run/sample/generation/${genNum}`);
    return response.data;
  },

  /**
   * Get DNA bundle for a candidate
   */
  getDNABundle: async (candidateId: string): Promise<DNABundle> => {
    const response = await apiClient.get<DNABundle>(`/run/sample/dna/${candidateId}`);
    return response.data;
  },

  /**
   * Get prompt DNA for a candidate
   */
  getPromptDNA: async (candidateId: string): Promise<PromptDNA> => {
    const response = await apiClient.get<PromptDNA>(`/run/sample/prompt/${candidateId}`);
    return response.data;
  },

  /**
   * Get the Repository Planning Graph
   */
  getRPG: async (): Promise<RepositoryPlanningGraph> => {
    const response = await apiClient.get<RepositoryPlanningGraph>('/run/sample/rpg');
    return response.data;
  },

  /**
   * Get run summary
   */
  getRunSummary: async (): Promise<RunSummary> => {
    const response = await apiClient.get<RunSummary>('/run/sample/summary');
    return response.data;
  },
};

export default api;
