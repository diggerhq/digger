import { Sandbox } from "@e2b/code-interpreter";
import { SandboxRunner, RunnerOutput } from "./types.js";
import { SandboxRunRecord, SandboxRunResult } from "../jobs/jobTypes.js";
import { logger } from "../logger.js";

export interface E2BRunnerOptions {
  apiKey?: string;
  defaultTemplateId?: string; // Pre-built with TF 1.5.5
  bareBonesTemplateId?: string; // Base for custom versions
}

/**
 * E2B runner that executes Terraform commands inside an E2B sandbox.
 * Uses the official @e2b/sdk to create sandboxes, upload files, and run commands.
 */
export class E2BSandboxRunner implements SandboxRunner {
  readonly name = "e2b";

  constructor(private readonly options: E2BRunnerOptions) {
    if (!options.apiKey) {
      throw new Error("E2B_API_KEY is required when SANDBOX_RUNNER=e2b");
    }
    if (!options.defaultTemplateId) {
      throw new Error("E2B_DEFAULT_TEMPLATE_ID is required when SANDBOX_RUNNER=e2b");
    }
    if (!options.bareBonesTemplateId) {
      throw new Error("E2B_BAREBONES_TEMPLATE_ID is required when SANDBOX_RUNNER=e2b");
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
    const sandbox = await this.createSandbox(requestedVersion);
    try {
      // Install Terraform if not already present
      await this.ensureTerraform(sandbox);
      
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
    const sandbox = await this.createSandbox(requestedVersion);
    try {
      // Install Terraform if not already present
      await this.ensureTerraform(sandbox);
      
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

  private async createSandbox(requestedVersion?: string): Promise<Sandbox> {
    // Select template based on requested Terraform version
    let templateId: string;
    let needsInstall = false;
    const version = requestedVersion || "1.5.5";
    
    if (version === "1.5.5" && this.options.defaultTemplateId) {
      // Use pre-built template with TF 1.5.5
      templateId = this.options.defaultTemplateId;
      needsInstall = false;
      logger.info({ templateId, version: "1.5.5" }, "using pre-built template with Terraform 1.5.5");
    } else if (this.options.bareBonesTemplateId) {
      // Use bare-bones template for custom version
      templateId = this.options.bareBonesTemplateId;
      needsInstall = true;
      logger.info({ templateId, version }, "using bare-bones template for custom Terraform version");
    } else {
      throw new Error("E2B templates not configured. Set E2B_DEFAULT_TEMPLATE_ID and E2B_BAREBONES_TEMPLATE_ID");
    }
    
    logger.info({ templateId }, "creating E2B sandbox");
    const sandbox = await Sandbox.create(templateId, {
      apiKey: this.options.apiKey,
    });
    logger.info({ sandboxId: sandbox.sandboxId }, "E2B sandbox created");
    
    // Store whether we need to install TF
    (sandbox as any)._needsTerraformInstall = needsInstall;
    (sandbox as any)._requestedTerraformVersion = version;
    
    return sandbox;
  }

  private async ensureTerraform(sandbox: Sandbox): Promise<void> {
    // Always check if terraform is actually installed, even in pre-built templates
    logger.info("checking for Terraform installation");
    const checkResult = await sandbox.commands.run("which terraform 2>/dev/null || echo 'not-found'");
    if (!checkResult.stdout.includes("not-found")) {
      const versionCheck = await sandbox.commands.run("terraform version");
      logger.info({ 
        path: checkResult.stdout.trim(),
        version: versionCheck.stdout.split('\n')[0]
      }, "Terraform already installed");
      return;
    }
    
    // If we expected it to be pre-installed but it's not, log a warning
    if (!(sandbox as any)._needsTerraformInstall) {
      logger.warn("Terraform not found in pre-built template, installing at runtime");
    }

    // Use requested version or default
    const terraformVersion = (sandbox as any)._requestedTerraformVersion || "1.9.8";
    logger.info({ version: terraformVersion }, "installing Terraform in sandbox");
    
    // Download and install Terraform binary directly (faster and simpler)
    const installScript = `
      set -e
      cd /tmp
      wget -q https://releases.hashicorp.com/terraform/${terraformVersion}/terraform_${terraformVersion}_linux_amd64.zip
      unzip -q terraform_${terraformVersion}_linux_amd64.zip
      sudo mv terraform /usr/local/bin/
      sudo chmod +x /usr/local/bin/terraform
      terraform version
    `;

    const result = await sandbox.commands.run(installScript);
    logger.info({ 
      version: result.stdout.trim() 
    }, "Terraform installation complete");
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
    const cmdStr = `terraform ${args.join(" ")}`;
    logger.info({ cmd: cmdStr, cwd }, "running terraform command in E2B sandbox");

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
        `terraform ${args[0]} exited with code ${exitCode}\n${mergedLogs}`,
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

