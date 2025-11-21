import { SandboxRunner } from "../runners/types.js";
import { JobStore } from "./jobStore.js";
import { SandboxRunRecord } from "./jobTypes.js";
import { logger } from "../logger.js";

export class JobRunner {
  constructor(
    private readonly store: JobStore,
    private readonly runner: SandboxRunner,
  ) {}

  schedule(job: SandboxRunRecord) {
    setImmediate(() => this.execute(job.id));
  }

  private async execute(jobId: string) {
    const job = this.store.get(jobId);
    if (!job) {
      return;
    }

    this.store.updateStatus(job.id, "running");
    logger.info({ job: job.id, runner: this.runner.name }, "sandbox job started");

    try {
      const result = await this.runner.run(job, (chunk) => this.store.appendLogs(job.id, chunk));
      this.store.updateStatus(job.id, "succeeded", result.logs);
      this.store.setResult(job.id, result.result);
      logger.info({ job: job.id }, "sandbox job succeeded");
    } catch (err) {
      const message =
        err instanceof Error ? err.message : "sandbox execution failed";
      logger.error({ job: job.id, err }, "sandbox job failed");
      this.store.updateStatus(job.id, "failed", job.logs, message);
      this.store.setResult(job.id, undefined);
    }
  }
}
