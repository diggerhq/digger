import { AppConfig } from "../config.js";
import { SandboxRunner } from "./types.js";
import { LocalTerraformRunner } from "./localRunner.js";
import { E2BSandboxRunner } from "./e2bRunner.js";

export function createRunner(config: AppConfig): SandboxRunner {
  if (config.runner === "e2b") {
    return new E2BSandboxRunner(config.e2b);
  }
  return new LocalTerraformRunner({
    terraformBinary: config.local.terraformBinary,
  });
}

