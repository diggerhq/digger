import { Sandbox } from "@e2b/code-interpreter";
import { SandboxRunner, RunnerOutput } from "./types.js";
import { SandboxRunRecord, SandboxRunResult } from "../jobs/jobTypes.js";
import { logger } from "../logger.js";
import { findTemplate, getFallbackTemplateId } from "../templateRegistry.js";

export interface E2BRunnerOptions {
  apiKey?: string;
  defaultTemplateId?: string; // Deprecated: kept for backward compatibility
  bareBonesTemplateId?: string; // Optional fallback template for runtime installation
}

/**
 * E2B runner that executes Terraform commands inside an E2B sandbox.
 * Automatically selects pre-built templates from the registry or falls back to runtime installation.
 */
export class E2BSandboxRunner implements SandboxRunner {
  readonly name = "e2b";

  constructor(private readonly options: E2BRunnerOptions) {
    if (!options.apiKey) {
      throw new Error("E2B_API_KEY is required when SANDBOX_RUNNER=e2b");
    }
  }

  async run(job: SandboxRunRecord): Promise<RunnerOutput> {
    if (job.payload.operation === "plan") {
      return this.runPlan(job);
    }
    return this.runApply(job);
  }

  private async runPlan(job: SandboxRunRecord): Promise<RunnerOutput> {
    const requestedVersion = job.payload.terraformVersion || "1.5.5";
    const requestedEngine = job.payload.engine || "terraform";
    const { sandbox, needsInstall } = await this.createSandbox(requestedVersion, requestedEngine);
    try {
      // Install IaC tool if using fallback template
      if (needsInstall) {
        await this.installIacTool(sandbox, requestedEngine, requestedVersion);
      }
      
      const workDir = await this.setupWorkspace(sandbox, job);
      const logs: string[] = [];

      // Run terraform init
      await this.runTerraformCommand(
        sandbox,
        workDir,
        ["init", "-input=false", "-no-color"],
        logs,
      );

      // Run terraform plan
      const planArgs = ["plan", "-input=false", "-no-color", "-out=tfplan.binary"];
      if (job.payload.isDestroy) {
        planArgs.splice(1, 0, "-destroy");
      }
      await this.runTerraformCommand(sandbox, workDir, planArgs, logs);

      // Get plan JSON
      const showResult = await this.runTerraformCommand(
        sandbox,
        workDir,
        ["show", "-json", "tfplan.binary"],
        logs,
      );

      const planJSON = showResult.stdout;
      const summary = this.summarizePlan(planJSON);
      const result: SandboxRunResult = {
        hasChanges: summary.hasChanges,
        resourceAdditions: summary.additions,
        resourceChanges: summary.changes,
        resourceDestructions: summary.destroys,
        planJSON: Buffer.from(planJSON, "utf8").toString("base64"),
      };

      return { logs: logs.join("\n"), result };
    } finally {
      await sandbox.kill();
    }
  }

  private async runApply(job: SandboxRunRecord): Promise<RunnerOutput> {
    const requestedVersion = job.payload.terraformVersion || "1.5.5";
    const requestedEngine = job.payload.engine || "terraform";
    const { sandbox, needsInstall } = await this.createSandbox(requestedVersion, requestedEngine);
    try {
      // Install IaC tool if using fallback template
      if (needsInstall) {
        await this.installIacTool(sandbox, requestedEngine, requestedVersion);
      }
      
      const workDir = await this.setupWorkspace(sandbox, job);
      const logs: string[] = [];

      // Run terraform init
      await this.runTerraformCommand(
        sandbox,
        workDir,
        ["init", "-input=false", "-no-color"],
        logs,
      );

      // Run terraform apply/destroy
      const applyCommand = job.payload.isDestroy ? "destroy" : "apply";
      await this.runTerraformCommand(
        sandbox,
        workDir,
        [applyCommand, "-auto-approve", "-input=false", "-no-color"],
        logs,
      );

      // Read the state file
      const statePath = `${workDir}/terraform.tfstate`;
      const stateContent = await sandbox.files.read(statePath);
      const result: SandboxRunResult = {
        state: Buffer.from(stateContent, "utf8").toString("base64"),
      };

      return { logs: logs.join("\n"), result };
    } finally {
      await sandbox.kill();
    }
  }

  private async createSandbox(requestedVersion?: string, requestedEngine?: string): Promise<{ sandbox: Sandbox; needsInstall: boolean }> {
    const version = requestedVersion || "1.5.5";
    const engine = requestedEngine === "tofu" ? "tofu" : "terraform";
    
    // Try to find a pre-built template for this version
    const prebuiltAlias = findTemplate(engine, version);
    
    let templateId: string;
    let needsInstall: boolean;
    
    if (prebuiltAlias) {
      // Use pre-built template with this version already installed
      templateId = prebuiltAlias;
      needsInstall = false;
      logger.info({ templateId, engine, version }, "using pre-built template");
    } else {
      // Fall back to bare-bones template and install at runtime
      templateId = getFallbackTemplateId(this.options.bareBonesTemplateId);
      needsInstall = true;
      logger.warn({ templateId, engine, version }, "no pre-built template found, will install at runtime");
    }
    
    logger.info({ templateId }, "creating E2B sandbox");
    const sandbox = await Sandbox.create(templateId, {
      apiKey: this.options.apiKey,
    });
    logger.info({ sandboxId: sandbox.sandboxId }, "E2B sandbox created");
    
    // Store engine metadata for command execution
    (sandbox as any)._requestedEngine = engine;
    
    return { sandbox, needsInstall };
  }


  private async installIacTool(sandbox: Sandbox, engine: string, version: string): Promise<void> {
    logger.info({ engine, version }, "installing IaC tool at runtime");
    
    let installScript: string;
    
    if (engine === "tofu") {
      // Download and install OpenTofu binary
      installScript = `
        set -e
        cd /tmp
        wget -q -O tofu.zip https://github.com/opentofu/opentofu/releases/download/v${version}/tofu_${version}_linux_amd64.zip
        unzip -q tofu.zip
        sudo mv tofu /usr/local/bin/
        sudo chmod +x /usr/local/bin/tofu
        tofu version
      `;
    } else {
      // Download and install Terraform binary
      installScript = `
        set -e
        cd /tmp
        wget -q https://releases.hashicorp.com/terraform/${version}/terraform_${version}_linux_amd64.zip
        unzip -q terraform_${version}_linux_amd64.zip
        sudo mv terraform /usr/local/bin/
        sudo chmod +x /usr/local/bin/terraform
        terraform version
      `;
    }

    const result = await sandbox.commands.run(installScript);
    logger.info({ 
      engine,
      version: result.stdout.trim() 
    }, "IaC tool installation complete");
  }

  private async setupWorkspace(
    sandbox: Sandbox,
    job: SandboxRunRecord,
  ): Promise<string> {
    // Use /home/user which is writable in E2B sandboxes
    const workDir = "/home/user/workspace";
    await sandbox.commands.run(`mkdir -p ${workDir}`);

    // Write the config archive
    const archivePath = `${workDir}/bundle.tar.gz`;
    const archiveBuffer = Buffer.from(job.payload.configArchive, "base64");
    await sandbox.files.write(archivePath, archiveBuffer.buffer);

    // Extract the archive
    await sandbox.commands.run(`cd ${workDir} && tar -xzf bundle.tar.gz`);

    // Determine the execution directory
    const execDir = job.payload.workingDirectory
      ? `${workDir}/${job.payload.workingDirectory}`
      : workDir;

    // Write the state file if provided
    if (job.payload.state) {
      const statePath = `${execDir}/terraform.tfstate`;
      const stateBuffer = Buffer.from(job.payload.state, "base64");
      await sandbox.files.write(statePath, stateBuffer.buffer);
    }

    return execDir;
  }

  private async runTerraformCommand(
    sandbox: Sandbox,
    cwd: string,
    args: string[],
    logBuffer?: string[],
  ): Promise<{ stdout: string; stderr: string }> {
    const engine = (sandbox as any)._requestedEngine || "terraform";
    const binaryName = engine === "tofu" ? "tofu" : "terraform";
    const cmdStr = `${binaryName} ${args.join(" ")}`;
    logger.info({ cmd: cmdStr, cwd, engine }, "running IaC command in E2B sandbox");

    const result = await sandbox.commands.run(cmdStr, {
      cwd,
      envs: {
        TF_IN_AUTOMATION: "1",
      },
    });

    const stdout = result.stdout;
    const stderr = result.stderr;
    const exitCode = result.exitCode;

    const mergedLogs = `${stdout}\n${stderr}`.trim();
    if (logBuffer && mergedLogs.length > 0) {
      logBuffer.push(mergedLogs);
    }

    if (exitCode !== 0) {
      throw new Error(
        `${binaryName} ${args[0]} exited with code ${exitCode}\n${mergedLogs}`,
      );
    }

    return { stdout, stderr };
  }

  private summarizePlan(planJSON: string) {
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
}

