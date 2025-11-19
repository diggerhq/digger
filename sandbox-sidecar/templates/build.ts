// build.ts
import { Template, defaultBuildLogger } from "e2b";
import { template } from "./test-template.ts";

async function main() {
  const buildInfo = await Template.build(template, {
    alias: "terraform-prebuilt-new",           // template name / alias
    cpuCount: 4,
    memoryMB: 2048,
    onBuildLogs: defaultBuildLogger(),
  });

  console.log("Template built:");
  console.log("Template ID:", buildInfo.templateId);
  console.log("Build ID:", buildInfo.buildId);
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
