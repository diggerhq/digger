import dotenv from "dotenv";

dotenv.config();

export type RunnerType = "local" | "e2b";

export interface AppConfig {
  port: number;
  runner: RunnerType;
  local: {
    terraformBinary: string;
  };
  e2b: {
    apiKey?: string;
    defaultTemplateId?: string; // Pre-built template with TF 1.5.5
    bareBonesTemplateId?: string; // Base template for custom versions
  };
}

const parsePort = (value: string | undefined, fallback: number) => {
  if (!value) {
    return fallback;
  }
  const parsed = Number(value);
  if (Number.isNaN(parsed) || parsed <= 0) {
    return fallback;
  }
  return parsed;
};

export function loadConfig(): AppConfig {
  const runnerEnv = (process.env.SANDBOX_RUNNER || "local").toLowerCase();
  const runner: RunnerType = runnerEnv === "e2b" ? "e2b" : "local";

  return {
    port: parsePort(process.env.PORT, 9100),
    runner,
    local: {
      terraformBinary: process.env.LOCAL_TERRAFORM_BIN || "terraform",
    },
    e2b: {
      apiKey: process.env.E2B_API_KEY,
      defaultTemplateId: process.env.E2B_DEFAULT_TEMPLATE_ID, // Pre-built with TF 1.5.5
      bareBonesTemplateId: process.env.E2B_BAREBONES_TEMPLATE_ID, // Base for custom versions
    },
  };
}

