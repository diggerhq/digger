import { SandboxRunRecord, SandboxRunResult } from "../jobs/jobTypes.js";

export interface RunnerOutput {
  logs: string;
  result?: SandboxRunResult;
}

export interface SandboxRunner {
  readonly name: string;
  run(job: SandboxRunRecord, appendLog?: (chunk: string) => void): Promise<RunnerOutput>;
}
