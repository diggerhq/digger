import { SandboxOperation } from "../types/runTypes.js";

export type JobStatus = "pending" | "running" | "succeeded" | "failed";

export interface SandboxRunPayload {
  operation: SandboxOperation;
  runId: string;
  planId?: string;
  orgId: string;
  unitId: string;
  configurationVersionId: string;
  isDestroy: boolean;
  terraformVersion?: string;
  engine?: "terraform" | "tofu";
  workingDirectory?: string;
  configArchive: string;
  state?: string;
  metadata?: Record<string, string>;
}

export interface SandboxRunResult {
  hasChanges?: boolean;
  resourceAdditions?: number;
  resourceChanges?: number;
  resourceDestructions?: number;
  planJSON?: string; // base64 encoded terraform show -json result
  state?: string; // base64 encoded terraform.tfstate
}

export interface SandboxRunRecord {
  id: string;
  status: JobStatus;
  logs: string;
  error?: string;
  payload: SandboxRunPayload;
  result?: SandboxRunResult;
  createdAt: Date;
  updatedAt: Date;
}

