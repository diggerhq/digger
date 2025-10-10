
export interface Organisation {
    ID: number
    CreatedAt: string
    UpdatedAt: string
    DeletedAt: null | string
    Name: string
    ExternalSource: string
    ExternalId: string
  }




export interface Project {
    id: number
    name: string
    directory: string
    repo_full_name: string
    drift_enabled: boolean
    drift_status: string
  }
  
  
  export interface Repo {
    id: number
    created_at: string
    updated_at: string
    deleted_at: null | string
    name: string
    repo_full_name: string
    repo_name: string
    repo_url: string
    vcs: string
    organisation_id: number
    Organisation: Organisation | undefined
  }

export interface Job {
    ID: number
    CreatedAt: string
    UpdatedAt: string
    DeletedAt: null | string
    DiggerJobID: string
    Status: string
    WorkflowRunURL: string
    WorkflowFile: string
    TerraformOutput: string
    PRNumber: number
    RepoFullName: string
    BranchName: string
  }
  

  interface BillingInfo {
    billing_plan: "free" | "pro" | "unlimited"
    remaining_free_projects: number
    monitored_projects_limit?: number
    monitored_projects_count?: number
    billable_projects_count?: number
  }
