import { Router } from "express";
import {
  runRequestSchema,
  ApiRunResponse,
  ApiRunStatusResponse,
} from "../types/runTypes.js";
import { JobStore } from "../jobs/jobStore.js";
import { JobRunner } from "../jobs/jobRunner.js";
import { SandboxRunPayload } from "../jobs/jobTypes.js";

export function createRunRouter(
  store: JobStore,
  runner: JobRunner,
): Router {
  const router = Router();

  router.post("/api/v1/sandboxes/runs", (req, res, next) => {
    try {
      const parsed = runRequestSchema.parse(req.body);
      const payload: SandboxRunPayload = {
        operation: parsed.operation,
        runId: parsed.run_id,
        planId: parsed.plan_id,
        orgId: parsed.org_id,
        unitId: parsed.unit_id,
        configurationVersionId: parsed.configuration_version_id,
        isDestroy: parsed.is_destroy,
        terraformVersion: parsed.terraform_version,
        workingDirectory: parsed.working_directory,
        configArchive: parsed.config_archive,
        state: parsed.state,
        metadata: parsed.metadata,
      };

      const job = store.create(payload);
      runner.schedule(job);

      const response: ApiRunResponse = { id: job.id };
      res.status(202).json(response);
    } catch (error) {
      next(error);
    }
  });

  router.get("/api/v1/sandboxes/runs/:id", (req, res) => {
    const job = store.get(req.params.id);
    if (!job) {
      return res.status(404).json({
        error: "not_found",
        message: `Run ${req.params.id} does not exist`,
      });
    }

    const response: ApiRunStatusResponse = {
      id: job.id,
      operation: job.payload.operation,
      status: job.status,
      logs: job.logs,
      error: job.error,
      metadata: job.payload.metadata ?? {},
      result: job.result
        ? {
            has_changes: job.result.hasChanges,
            resource_additions: job.result.resourceAdditions,
            resource_changes: job.result.resourceChanges,
            resource_destructions: job.result.resourceDestructions,
            plan_json: job.result.planJSON,
            state: job.result.state,
          }
        : undefined,
      created_at: job.createdAt.toISOString(),
      updated_at: job.updatedAt.toISOString(),
    };

    res.json(response);
  });

  return router;
}

