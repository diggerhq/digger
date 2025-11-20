import express from "express";
import cors from "cors";
import { loadConfig } from "./config.js";
import { logger } from "./logger.js";
import { JobStore } from "./jobs/jobStore.js";
import { createRunner } from "./runners/index.js";
import { JobRunner } from "./jobs/jobRunner.js";
import { createRunRouter } from "./routes/runRoutes.js";

const config = loadConfig();
const app = express();

app.use(cors());
app.use(express.json({ limit: "20mb" }));

app.get("/healthz", (_req, res) => res.json({ status: "ok" }));
app.get("/health", (_req, res) => res.json({ status: "ok" }));

const store = new JobStore();
const runner = createRunner(config);
const jobRunner = new JobRunner(store, runner);

app.use(createRunRouter(store, jobRunner));

app.use(
  (
    err: Error & { status?: number },
    _req: express.Request,
    res: express.Response,
    _next: express.NextFunction,
  ) => {
    logger.error({ err }, "unhandled error");
    res
      .status(err.status ?? 500)
      .json({ error: err.name || "error", message: err.message });
  },
);

app.listen(config.port, () => {
  logger.info(
    { port: config.port, runner: runner.name },
    "Sandbox sidecar listening",
  );
});

