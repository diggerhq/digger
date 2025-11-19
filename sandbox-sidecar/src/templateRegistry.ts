// Template registry - maps engine + version to E2B template aliases
// This should match what's built in templates/manifest.ts

export interface TemplateInfo {
  engine: "terraform" | "tofu";
  version: string;
  alias: string;
}

// Template version - bump this when the build recipe changes
const TEMPLATE_VERSION = "0.1.0";

// Generate alias matching the build system
function aliasFor(engine: string, version: string, tplVersion: string): string {
  const engineVerSlug = version.replace(/\./g, "-");
  const tplVerSlug = tplVersion.replace(/\./g, "-");
  return `${engine}-${engineVerSlug}--tpl-${tplVerSlug}`;
}

// Registry of pre-built templates
// Keep this in sync with templates/manifest.ts
export const TEMPLATE_REGISTRY: TemplateInfo[] = [
  { engine: "terraform", version: "1.0.11", alias: aliasFor("terraform", "1.0.11", TEMPLATE_VERSION) },
  { engine: "terraform", version: "1.3.9",  alias: aliasFor("terraform", "1.3.9",  TEMPLATE_VERSION) },
  { engine: "terraform", version: "1.5.5",  alias: aliasFor("terraform", "1.5.5",  TEMPLATE_VERSION) },
  { engine: "tofu",      version: "1.6.0",  alias: aliasFor("tofu",      "1.6.0",  TEMPLATE_VERSION) },
  { engine: "tofu",      version: "1.10.0", alias: aliasFor("tofu",      "1.10.0", TEMPLATE_VERSION) },
];

/**
 * Find a pre-built template for the given engine and version
 * Returns the template alias if found, undefined otherwise
 */
export function findTemplate(engine: "terraform" | "tofu", version: string): string | undefined {
  const match = TEMPLATE_REGISTRY.find(
    t => t.engine === engine && t.version === version
  );
  return match?.alias;
}

/**
 * Get the fallback template ID for runtime installation
 * This should be a bare-bones template with just the base OS
 */
export function getFallbackTemplateId(fallbackId?: string): string {
  return fallbackId || "rki5dems9wqfm4r03t7g"; // Default E2B base template
}

/**
 * Validate that E2B templates exist and are accessible
 * Returns an array of validation results
 */
export async function validateTemplates(apiKey?: string): Promise<Array<{ templateId: string; valid: boolean; error?: string }>> {
  if (!apiKey) {
    return [{ templateId: "N/A", valid: false, error: "No E2B API key provided" }];
  }

  const results: Array<{ templateId: string; valid: boolean; error?: string }> = [];

  // Validate each template alias
  for (const template of TEMPLATE_REGISTRY) {
    try {
      // Try to resolve the template alias via E2B API
      const response = await fetch(`https://api.e2b.dev/templates/${template.alias}`, {
        method: "GET",
        headers: {
          "X-API-Key": apiKey,
        },
      });

      if (response.ok) {
        results.push({ templateId: template.alias, valid: true });
      } else {
        results.push({
          templateId: template.alias,
          valid: false,
          error: `HTTP ${response.status}: ${response.statusText}`,
        });
      }
    } catch (err) {
      results.push({
        templateId: template.alias,
        valid: false,
        error: err instanceof Error ? err.message : String(err),
      });
    }
  }

  return results;
}

