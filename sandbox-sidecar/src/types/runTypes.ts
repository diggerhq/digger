import { z } from "zod";

export const sandboxOperationSchema = z.enum(["plan", "apply"]);
export type SandboxOperation = z.infer<typeof sandboxOperationSchema>;

export const runRequestSchema = z.object({
  operation: sandboxOperationSchema,
  run_id: z.string().min(1),
  plan_id: z.string().optional(),
  org_id: z.string().min(1),
  unit_id: z.string().min(1),
  configuration_version_id: z.string().min(1),
  is_destroy: z.boolean(),
  terraform_version: z.string().optional(),
  working_directory: z.string().optional(),
  config_archive: z.string().min(1),
  state: z.string().optional(),
  metadata: z.record(z.string()).optional(),
});

export type RunRequestSchema = z.infer<typeof runRequestSchema>;

export interface ApiRunResponse {
  id: string;
}

export interface ApiRunStatusResponse {
  id: string;
  operation: SandboxOperation;
  status: string;
  logs: string;
  error?: string;
  metadata: Record<string, string>;
  result?: {
    has_changes?: boolean;
    resource_additions?: number;
    resource_changes?: number;
    resource_destructions?: number;
    plan_json?: string;
    state?: string;
  };
  created_at: string;
  updated_at: string;
}

