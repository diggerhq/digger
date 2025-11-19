import "dotenv/config";
import { Template, defaultBuildLogger } from "e2b";

async function main() {
  const alias = "opentaco-barebones";
  
  console.log(`\n=== Building ${alias} (2 vCPU / 4GB) ===`);
  console.log("This is a minimal Ubuntu 22.04 template with no IaC tools pre-installed.");
  console.log("Use this as the fallback template for custom/unsupported versions.\n");

  // Create a minimal Ubuntu template with just basic tools
  const template = Template()
    .fromUbuntuImage("22.04")
    .setUser("root")
    .runCmd("apt-get update && apt-get install -y wget unzip ca-certificates curl")
    .setUser("user");

  await Template.build(template, {
    alias,
    cpuCount: 2,
    memoryMB: 4096,
    onBuildLogs: defaultBuildLogger(),
  });

  console.log(`\n✅ Built ${alias}`);
  console.log(`\nTo use this template, set in your backend:`);
  console.log(`OPENTACO_E2B_BAREBONES_TEMPLATE_ID="${alias}"`);
}

main().catch(err => {
  console.error(err);
  process.exit(1);
});

