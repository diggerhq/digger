import dotenv from "dotenv";

dotenv.config();

export type RunnerType = "e2b";

export interface AppConfig {
  port: number;
  runner: RunnerType;
  e2b: {
    apiKey?: string;
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
  const runnerEnv = (process.env.SANDBOX_RUNNER || "e2b").toLowerCase();
  
  if (runnerEnv !== "e2b") {
    throw new Error("Only E2B runner is supported. Set SANDBOX_RUNNER=e2b");
  }

  return {
    port: parsePort(process.env.PORT, 9100),
    runner: "e2b",
    e2b: {
      apiKey: process.env.E2B_API_KEY,
      bareBonesTemplateId: process.env.E2B_BAREBONES_TEMPLATE_ID,
    },
  };
}

