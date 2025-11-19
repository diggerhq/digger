// templates/manifest.ts
export type Engine = "terraform" | "tofu";

export interface TemplateSpec {
  engine: Engine;
  engineVersion: string;  
  tplVersion: string;      
}

export const TEMPLATE_VERSION = "0.1.2";  // bump this when recipe changes

export const TEMPLATES: TemplateSpec[] = [
  { engine: "terraform", engineVersion: "1.0.11", tplVersion: TEMPLATE_VERSION },
  { engine: "terraform", engineVersion: "1.3.9",  tplVersion: TEMPLATE_VERSION },
  { engine: "terraform", engineVersion: "1.5.7",  tplVersion: TEMPLATE_VERSION },
  { engine: "tofu",      engineVersion: "1.6.0",  tplVersion: TEMPLATE_VERSION },
  { engine: "tofu",      engineVersion: "1.10.0", tplVersion: TEMPLATE_VERSION },
];


export function aliasFor(spec: TemplateSpec) {
    const engineVerSlug = spec.engineVersion.replace(/\./g, "-");
    const tplVerSlug = spec.tplVersion.replace(/\./g, "-");
    return `${spec.engine}-${engineVerSlug}--tpl-${tplVerSlug}`;
  }