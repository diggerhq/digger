import { AppConfig } from "../config.js";
import { SandboxRunner } from "./types.js";
import { E2BSandboxRunner } from "./e2bRunner.js";

export function createRunner(config: AppConfig): SandboxRunner {
  if (config.runner !== "e2b") {
    throw new Error("Only E2B runner is supported. Set SANDBOX_RUNNER=e2b");
  }
  return new E2BSandboxRunner(config.e2b);
}

