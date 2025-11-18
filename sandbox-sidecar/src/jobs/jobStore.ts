import { nanoid } from "nanoid";
import {
  JobStatus,
  SandboxRunPayload,
  SandboxRunRecord,
  SandboxRunResult,
} from "./jobTypes.js";

export class JobStore {
  private jobs = new Map<string, SandboxRunRecord>();

  create(payload: SandboxRunPayload): SandboxRunRecord {
    const id = `sbx_run_${nanoid(10)}`;
    const now = new Date();
    const job: SandboxRunRecord = {
      id,
      payload,
      status: "pending",
      logs: "",
      createdAt: now,
      updatedAt: now,
    };
    this.jobs.set(id, job);
    return job;
  }

  get(id: string): SandboxRunRecord | undefined {
    return this.jobs.get(id);
  }

  updateStatus(id: string, status: JobStatus, logs?: string, error?: string) {
    const job = this.jobs.get(id);
    if (!job) return;
    job.status = status;
    if (typeof logs === "string") {
      job.logs = logs;
    }
    job.error = error;
    job.updatedAt = new Date();
  }

  setResult(id: string, result: SandboxRunResult | undefined) {
    const job = this.jobs.get(id);
    if (!job) return;
    job.result = result;
    job.updatedAt = new Date();
  }
}

