import os from "os";
import path from "path";
import { promises as fs } from "fs";
import { spawn } from "child_process";
import tar from "tar";
import { SandboxRunner, RunnerOutput } from "./types.js";
import { SandboxRunRecord, SandboxRunResult } from "../jobs/jobTypes.js";
import { logger } from "../logger.js";

interface LocalRunnerOptions {
  terraformBinary: string;
}

interface CommandResult {
  code: number;
  stdout: string;
  stderr: string;
}

export class LocalTerraformRunner implements SandboxRunner {
  readonly name = "local";

  constructor(private readonly options: LocalRunnerOptions) {}

  async run(job: SandboxRunRecord): Promise<RunnerOutput> {
    if (job.payload.operation === "plan") {
      return this.runPlan(job);
    }
    return this.runApply(job);
  }

  private async runPlan(job: SandboxRunRecord): Promise<RunnerOutput> {
    const workspace = await this.createWorkspace(job);
    try {
      const logs: string[] = [];
      await this.runTerraformCommand(
        workspace.execCwd,
        ["init", "-input=false", "-no-color"],
        logs,
      );

      const planArgs = ["plan", "-input=false", "-no-color", "-out=tfplan.binary"];
      if (job.payload.isDestroy) {
        planArgs.splice(1, 0, "-destroy");
      }
      await this.runTerraformCommand(workspace.execCwd, planArgs, logs);

      const show = await this.runTerraformCommand(
        workspace.execCwd,
        ["show", "-json", "tfplan.binary"],
      );
      logs.push(show.stdout);

      const planJSON = show.stdout;
      const summary = summarizePlan(planJSON);
      const result: SandboxRunResult = {
        hasChanges: summary.hasChanges,
        resourceAdditions: summary.additions,
        resourceChanges: summary.changes,
        resourceDestructions: summary.destroys,
        planJSON: Buffer.from(planJSON, "utf8").toString("base64"),
      };

      return { logs: logs.join("\n"), result };
    } finally {
      await fs.rm(workspace.root, { recursive: true, force: true });
    }
  }

  private async runApply(job: SandboxRunRecord): Promise<RunnerOutput> {
    const workspace = await this.createWorkspace(job);
    try {
      const logs: string[] = [];
      await this.runTerraformCommand(
        workspace.execCwd,
        ["init", "-input=false", "-no-color"],
        logs,
      );

      const applyCommand = job.payload.isDestroy ? "destroy" : "apply";
      await this.runTerraformCommand(
        workspace.execCwd,
        [applyCommand, "-auto-approve", "-input=false", "-no-color"],
        logs,
      );

      const statePath = path.join(workspace.execCwd, "terraform.tfstate");
      const stateBuffer = await fs.readFile(statePath);
      const result: SandboxRunResult = {
        state: stateBuffer.toString("base64"),
      };

      return { logs: logs.join("\n"), result };
    } finally {
      await fs.rm(workspace.root, { recursive: true, force: true });
    }
  }

  private async createWorkspace(job: SandboxRunRecord) {
    const root = await fs.mkdtemp(path.join(os.tmpdir(), "taco-sbx-"));
    const archivePath = path.join(root, "bundle.tar.gz");
    await fs.writeFile(
      archivePath,
      Buffer.from(job.payload.configArchive, "base64"),
    );
    await tar.x({
      file: archivePath,
      cwd: root,
    });

    const workingDirectory = job.payload.workingDirectory
      ? path.join(root, job.payload.workingDirectory)
      : root;
    const execCwd = path.resolve(workingDirectory);
    const exists = await pathExists(execCwd);
    if (!exists) {
      throw new Error(
        `Working directory ${job.payload.workingDirectory} not found in archive`,
      );
    }

    if (job.payload.state) {
      const statePath = path.join(execCwd, "terraform.tfstate");
      await fs.writeFile(statePath, Buffer.from(job.payload.state, "base64"));
    }

    return { root, execCwd };
  }

  private async runTerraformCommand(
    cwd: string,
    args: string[],
    logBuffer?: string[],
  ): Promise<CommandResult> {
    const result = await runCommand(this.options.terraformBinary, args, cwd);
    const mergedLogs = `${result.stdout}\n${result.stderr}`.trim();
    if (logBuffer && mergedLogs.length > 0) {
      logBuffer.push(mergedLogs);
    }
    if (result.code !== 0) {
      throw new Error(
        `terraform ${args[0]} exited with code ${result.code}\n${mergedLogs}`,
      );
    }
    return result;
  }
}

async function runCommand(
  command: string,
  args: string[],
  cwd: string,
): Promise<CommandResult> {
  return new Promise((resolve, reject) => {
    const child = spawn(command, args, {
      cwd,
      env: {
        ...process.env,
        TF_IN_AUTOMATION: "1",
      },
    });

    let stdout = "";
    let stderr = "";

    child.stdout.on("data", (data) => {
      stdout += data.toString();
    });
    child.stderr.on("data", (data) => {
      stderr += data.toString();
    });

    child.on("error", (error) => {
      reject(error);
    });

    child.on("close", (code) => {
      resolve({ code: code ?? 0, stdout, stderr });
    });
  });
}

function summarizePlan(planJSON: string) {
  try {
    const parsed = JSON.parse(planJSON);
    const changes = parsed?.resource_changes ?? [];
    let additions = 0;
    let updates = 0;
    let destroys = 0;

    for (const change of changes) {
      const actions: string[] = change?.change?.actions ?? [];
      if (actions.includes("create")) additions += 1;
      if (actions.includes("update")) updates += 1;
      if (actions.includes("delete") || actions.includes("destroy"))
        destroys += 1;
      if (actions.includes("replace")) {
        additions += 1;
        destroys += 1;
      }
    }

    return {
      hasChanges: additions + updates + destroys > 0,
      additions,
      changes: updates,
      destroys,
    };
  } catch (error) {
    logger.warn({ error }, "failed to parse terraform plan JSON");
    return {
      hasChanges: false,
      additions: 0,
      changes: 0,
      destroys: 0,
    };
  }
}

async function pathExists(target: string) {
  try {
    await fs.access(target);
    return true;
  } catch {
    return false;
  }
}

