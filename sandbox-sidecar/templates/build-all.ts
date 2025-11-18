import "dotenv/config";
import { Template, defaultBuildLogger } from "e2b";
import { terraformTemplate } from "./terraform-template.ts";
import { tofuTemplate } from "./tofu-template.ts";
import { TEMPLATES, aliasFor, TemplateSpec } from "./manifest.ts";

function buildTemplateObject(spec: TemplateSpec) {
  return spec.engine === "terraform"
    ? terraformTemplate(spec.engineVersion)
    : tofuTemplate(spec.engineVersion);
}

async function main() {
  const [, , maybeTplVersion] = process.argv;

  const specs = maybeTplVersion
    ? TEMPLATES.filter(t => t.tplVersion === maybeTplVersion)
    : TEMPLATES;

  if (specs.length === 0) {
    console.error("No templates match that tplVersion.");
    process.exit(1);
  }

  for (const spec of specs) {
    const alias = aliasFor(spec);
    console.log(`\n=== Building ${alias} ===`);

    await Template.build(buildTemplateObject(spec), {
      alias,
      cpuCount: 1,
      memoryMB: 1024,
      onBuildLogs: defaultBuildLogger(),
    });

    console.log(`✅ Built ${alias}`);
  }
}

main().catch(err => {
  console.error(err);
  process.exit(1);
});
