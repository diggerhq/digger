// Debug script to understand what's in the template
import { Sandbox } from "@e2b/code-interpreter";

async function debugTemplate() {
  console.log("=== E2B Template Debug ===\n");
  // Use the template ID directly to bypass alias caching
  const templateId = "vnjk0omiwu39qpbcsyf5"; // Template ID
  console.log(`Creating sandbox from template ID ${templateId}...`);
  
  const sandbox = await Sandbox.create({
    apiKey: process.env.E2B_API_KEY!,
    template: templateId,
  });
  
  console.log("✅ Sandbox created:", sandbox.sandboxId);
  console.log("\n--- Checking filesystem ---");
  
  // Check if /usr/local/bin exists
  const binCheck = await sandbox.commands.run("ls -la /usr/local/bin/ 2>&1 || echo 'Directory does not exist'");
  console.log("/usr/local/bin/ contents:");
  console.log(binCheck.stdout || binCheck.stderr);
  
  // Check PATH
  const pathCheck = await sandbox.commands.run("echo $PATH");
  console.log("\nPATH:");
  console.log(pathCheck.stdout);
  
  // Check if /usr/local/bin is in PATH
  const pathHasLocal = pathCheck.stdout.includes("/usr/local/bin");
  console.log("/usr/local/bin in PATH:", pathHasLocal);
  
  // Try to find terraform anywhere in the filesystem
  console.log("\n--- Searching entire filesystem for terraform ---");
  const findTf = await sandbox.commands.run("find / -name terraform -type f 2>/dev/null | head -20");
  console.log("Found terraform at:");
  console.log(findTf.stdout || "(none)");
  
  // Also search for any files in /usr/local/bin
  const localBinFiles = await sandbox.commands.run("find /usr/local/bin -type f 2>/dev/null");
  console.log("\nAll files in /usr/local/bin:");
  console.log(localBinFiles.stdout || "(none)");
  
  // Check which user we are
  const whoami = await sandbox.commands.run("whoami");
  console.log("\nCurrent user:");
  console.log(whoami.stdout);
  
  // Test if we can install something else (jq) to see if it's terraform-specific
  console.log("\n--- Testing installation of another package (jq) ---");
  const installJq = await sandbox.commands.run("sudo apt-get update -qq && sudo apt-get install -y jq 2>&1 | tail -5");
  console.log("jq installation output:");
  console.log(installJq.stdout);
  
  const jqCheck = await sandbox.commands.run("which jq && jq --version");
  console.log("\njq check:");
  console.log("Exit code:", jqCheck.exitCode);
  console.log("Output:", jqCheck.stdout || jqCheck.stderr);
  
  // Try running terraform
  console.log("\n--- Attempting to run terraform ---");
  const tfResult = await sandbox.commands.run("terraform version 2>&1 || echo 'Command failed'");
  console.log("Exit code:", tfResult.exitCode);
  console.log("Output:", tfResult.stdout || tfResult.stderr);
  
  // Check if curl/unzip are available (should be from our build)
  console.log("\n--- Checking installed packages ---");
  const curlCheck = await sandbox.commands.run("which curl");
  console.log("curl:", curlCheck.stdout.trim() || "NOT FOUND");
  
  const unzipCheck = await sandbox.commands.run("which unzip");
  console.log("unzip:", unzipCheck.stdout.trim() || "NOT FOUND");
  
  const wgetCheck = await sandbox.commands.run("which wget");
  console.log("wget:", wgetCheck.stdout.trim() || "NOT FOUND");
  
  await sandbox.close();
  console.log("\n✅ Debug complete");
}

debugTemplate().catch(console.error);

