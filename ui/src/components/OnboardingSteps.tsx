import { useState, useEffect } from "react"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Github, CheckCircle2, FileCode2, FileText, Copy, Database, Settings, Cloud, CloudRain, CloudSun } from "lucide-react"
import { GithubConnectButton } from "./GithubConnectButton"
import { WorkflowFileButton } from "./WorkflowFileButton"
import { DiggerYmlButton } from "./DiggerYmlButton"
// import { PRCreatedButton } from "./PRCreatedButton"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
// import { useToast } from "@/components/ui/use-toast"
import { useToast } from "@/hooks/use-toast"
import { Textarea } from "@/components/ui/textarea"
import { Label } from "@/components/ui/label"
import { Input } from "@/components/ui/input"
import { RadioGroup, RadioGroupItem } from "@/components/ui/radio-group"
import { ToastAction } from "./ui/toast"
import UnitCreateForm from "./UnitCreateForm"
import UnitConfigureInstructions from "./UnitConfigureInstructions"

interface Repo {
  id: number
  name: string
  repo_url: string
  vcs: string
}

interface OnboardingStepsProps {
  repoName?: string
  repoOwner?: string
  onComplete?: () => void
  userId: string
  email: string
  organisationId: string
  publicHostname: string
}

interface WorkflowConfig {
  cloudProvider: "aws" | "gcp" | "azure"
  connection: string
  iacType: "terraform" | "opentofu"
  iacVersion: string
}

export default function OnboardingSteps({ repoName, repoOwner, onComplete, userId, email, organisationId, publicHostname }: OnboardingStepsProps) {
  const [currentStep, setCurrentStep] = useState(() => {
    if (typeof window !== 'undefined') {
      const step = new URLSearchParams(window.location.search).get('step')
      if (step) return step
    }
    return "create-unit"
  })
  const [steps, setSteps] = useState({
    githubConnected: false,
    workflowCreated: false,
    diggerConfigCreated: false,
    unitCreated: false,
    unitConfigured: false,
    terraformPRCreated: false
  })
  const [createdUnit, setCreatedUnit] = useState<{ id: string; name: string } | null>(null)
  const [onboardingMode, setOnboardingMode] = useState<'unit' | 'pr_only' | null>(null)
  const [repos, setRepos] = useState<Repo[]>([])
  const [selectedRepo, setSelectedRepo] = useState<string>("")
  const [workflowConfig, setWorkflowConfig] = useState<WorkflowConfig>({
    cloudProvider: "aws",
    connection: "",
    iacType: "terraform",
    iacVersion: "1.5.6"
  })
  const getDiggerConfig = (iacType: "terraform" | "opentofu") => `projects:
  - name: my-dev
    dir: path/to/dev
    opentofu: ${iacType === 'opentofu' ? 'true' : 'false'}
    terraform: ${iacType === 'terraform' ? 'true' : 'false'}
`
  const [diggerConfig, setDiggerConfig] = useState(getDiggerConfig("terraform"))
  const { toast } = useToast()

  // Keep digger.yml engine toggles in sync with chosen IAC type until user reaches digger step
  useEffect(() => {
    if (!steps.workflowCreated) {
      setDiggerConfig(getDiggerConfig(workflowConfig.iacType))
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [workflowConfig.iacType])

  useEffect(() => {
    if (typeof window === 'undefined') return
    try {
      const m = window.localStorage.getItem('onboardingMode') as any
      if (m === 'unit' || m === 'pr_only') setOnboardingMode(m)
      const cu = window.localStorage.getItem('onboardingUnit')
      if (cu) setCreatedUnit(JSON.parse(cu))
    } catch {}
  }, [])

  useEffect(() => {
    if (typeof window === 'undefined') return
    try {
      if (onboardingMode) window.localStorage.setItem('onboardingMode', onboardingMode)
      if (createdUnit) window.localStorage.setItem('onboardingUnit', JSON.stringify(createdUnit))
    } catch {}
  }, [onboardingMode, createdUnit])

  const generateWorkflowContent = (config: WorkflowConfig) => {
    const iacVersion = config.iacVersion || (config.iacType === "terraform" ? "1.5.6" : "1.9.1")
    const iacName = config.iacType === "terraform" ? "Terraform" : "OpenTofu"
    const iacCommand = config.iacType === "terraform" ? "terraform" : "tofu"
    const remoteStateEnv = createdUnit
      ? `\n          DGR_UNIT_ID: ${createdUnit.id}\n          DGR_UNIT_NAME: ${createdUnit.name}\n          DGR_REMOTE_STATE: true`
      : onboardingMode === 'pr_only'
        ? `\n          # Using local state for now\n          DGR_REMOTE_STATE: false`
        : ''

    return `name: Digger Workflow

on:
  workflow_dispatch:
    inputs:
      spec:
        required: true
      run_name:
        required: false

run-name: '\${{inputs.run_name}}'

jobs:
  digger-job:
    runs-on: ubuntu-latest
    permissions:
      contents: write      # required to merge PRs
      actions: write       # required for plan persistence
      id-token: write      # required for workload-identity-federation
      pull-requests: write # required to post PR comments
      issues: read         # required to check if PR number is an issue or not
      statuses: write      # required to validate combined PR status

    steps:
      - uses: actions/checkout@v4
      - name: \${{ fromJSON(github.event.inputs.spec).job_id }}
        run: echo "job id \${{ fromJSON(github.event.inputs.spec).job_id }}"
      - uses: diggerhq/digger@vLatest
        with:
          digger-spec: \${{ inputs.spec }}
          ${iacCommand == "terraform" ? `setup-terraform: true
          terraform-version: ${iacVersion}${onboardingMode === 'unit' ? `
          terraform-tfe-token: \${{ secrets.TFE_TOKEN }}` : ""}${onboardingMode === 'unit' ? `
          terraform-tfe-hostname: ${publicHostname}` : ""}` : ""}${iacCommand == "tofu" ? `setup-opentofu: true
          opentofu-version: ${iacVersion}${onboardingMode === 'unit' ? `
          opentofu-tfe-token: \${{ secrets.TFE_TOKEN }}` : ""}${onboardingMode === 'unit' ? `
          opentofu-tfe-hostname: ${publicHostname}` : ""}` : ""}
          ${config.cloudProvider == "aws" ? `setup-aws: true
          aws-access-key-id: \${{ secrets.AWS_ACCESS_KEY_ID }}
          aws-secret-access-key: \${{ secrets.AWS_SECRET_ACCESS_KEY }}` : ""}${config.cloudProvider == "gcp" ? `setup-google-cloud: true
          google-cloud-credentials: \${{ secrets.GOOGLE_CLOUD_CREDENTIALS }}` : ""}${config.cloudProvider == "azure" ? `setup-azure: true
          azure-client-id: \${{ secrets.AZURE_CLIENT_ID }}
          azure-tenant-id: \${{ secrets.AZURE_TENANT_ID }}
          azure-subscription-id: \${{ secrets.AZURE_SUBSCRIPTION_ID }}` : ""}
        env:
          GITHUB_CONTEXT: \${{ toJson(github) }}
          GITHUB_TOKEN: \${{ secrets.GITHUB_TOKEN }}
`
  }

  const handleCopyWorkflow = () => {
    navigator.clipboard.writeText(generateWorkflowContent(workflowConfig))  
    toast({
      title: "Success",
      description: "Workflow content copied to clipboard",
      action: <ToastAction altText="OK">OK</ToastAction>,
    })
  }

  const handleCopyDigger = () => {
    navigator.clipboard.writeText(diggerConfig)
    toast({
      title: "Success",
      description: "digger.yml content copied to clipboard",
      action: <ToastAction altText="OK">OK</ToastAction>,
    })
  }


  const handleGithubConnect = () => {
    window.open("https://github.com/apps/digger-pro", "_blank")
    setCurrentStep("workflow")
    setSteps(prev => ({ ...prev, githubConnected: true }))
  }

  const handleWorkflowCreate = async () => {
    setCurrentStep("digger")
    setSteps(prev => ({ ...prev, workflowCreated: true }))
  }

  const handleDiggerConfigCreate = async () => {
    setCurrentStep("complete")
    setSteps(prev => ({ ...prev, diggerConfigCreated: true }))
  }

  const handleCreateUnit = (unit?: { id: string; name: string }) => {
    console.log('handleCreateUnit', unit)
    if (unit) setCreatedUnit({ id: unit.id, name: unit.name })
    setOnboardingMode('unit')
    setSteps(prev => ({ ...prev, unitCreated: true }))
    setCurrentStep("configure-unit")
  }

  const handleConfigureUnit = async () => {
    setCurrentStep("terraform")
    setSteps(prev => ({ ...prev, unitConfigured: true }))
  }

  // Finalize onboarding

  return (
    <div className="max-w-5xl mx-auto">
      <Card>
        <CardHeader>
          <CardTitle>Repository Setup</CardTitle>
          <CardDescription>Follow these steps to set up your repository with Digger</CardDescription>
        </CardHeader>
        <CardContent>
          <Tabs value={currentStep} onValueChange={setCurrentStep} className="w-full">
            <TabsList className="grid w-full grid-cols-6">
              <TabsTrigger value="create-unit" disabled={steps.unitCreated}>
                <Database className="mr-2 h-4 w-4" /> Create Unit
              </TabsTrigger>
              <TabsTrigger value="configure-unit" disabled={!steps.unitCreated || steps.unitConfigured}>
                <Settings className="mr-2 h-4 w-4" /> Configure Unit
              </TabsTrigger>              
              <TabsTrigger value="github" disabled={steps.githubConnected}>
                <Github className="mr-2 h-4 w-4" /> GitHub
              </TabsTrigger>
              <TabsTrigger value="workflow" disabled={!steps.githubConnected || steps.workflowCreated}>
                <FileCode2 className="mr-2 h-4 w-4" /> Workflow
              </TabsTrigger>
              <TabsTrigger value="digger" disabled={!steps.workflowCreated || steps.diggerConfigCreated}>
                <FileText className="mr-2 h-4 w-4" /> digger.yml
              </TabsTrigger>

              <TabsTrigger value="complete" disabled={!steps.unitCreated}>
                <CheckCircle2 className="mr-2 h-4 w-4" /> Complete
              </TabsTrigger>
            </TabsList>

            <TabsContent value="github" className="mt-6">
              <div className="space-y-4">
                <h3 className="font-medium">Connect GitHub Repository</h3>
                <p className="text-sm text-gray-500">
                  Install the Digger GitHub App to connect your repository
                </p>

                  <GithubConnectButton source="onboarding" onClick={handleGithubConnect} />
              </div>
            </TabsContent>

            <TabsContent value="workflow" className="mt-6">
              <div className="space-y-6">
                <h3 className="font-medium">Create Workflow Files</h3>
                <p className="text-sm text-gray-500">
                  Pick your settings and then copy the workflow file to your repository
                </p>

                  <>
                    <div className="space-y-4">
                      

                      <div className="space-y-4">
                        <div>
                          <Label>Cloud Provider</Label>
                          <RadioGroup
                            value={workflowConfig.cloudProvider}
                            onValueChange={(value: "aws" | "gcp" | "azure") =>
                              setWorkflowConfig(prev => ({ ...prev, cloudProvider: value }))
                            }
                            className="mt-3 grid grid-cols-1 md:grid-cols-3 gap-3"
                          >
                            <label
                              htmlFor="aws"
                              className={`relative flex cursor-pointer items-start gap-4 rounded-lg border p-4 md:p-5 transition-colors hover:bg-muted/50 ${workflowConfig.cloudProvider === 'aws' ? 'ring-2 ring-primary border-primary' : 'border-muted'}`}
                              onClick={() => setWorkflowConfig(prev => ({ ...prev, cloudProvider: 'aws' }))}
                            >
                              <RadioGroupItem value="aws" id="aws" className="sr-only" />
                              <div className="flex h-10 w-10 items-center justify-center rounded-md bg-primary/10 text-primary">
                                <Cloud className="h-5 w-5" />
                              </div>
                              <div className="space-y-1">
                                <span className="text-base font-semibold">AWS</span>
                                <p className="text-sm text-muted-foreground">Use AWS credentials or OIDC in your workflow.</p>
                              </div>
                            </label>

                            <label
                              htmlFor="gcp"
                              className={`relative flex cursor-pointer items-start gap-4 rounded-lg border p-4 md:p-5 transition-colors hover:bg-muted/50 ${workflowConfig.cloudProvider === 'gcp' ? 'ring-2 ring-primary border-primary' : 'border-muted'}`}
                              onClick={() => setWorkflowConfig(prev => ({ ...prev, cloudProvider: 'gcp' }))}
                            >
                              <RadioGroupItem value="gcp" id="gcp" className="sr-only" />
                              <div className="flex h-10 w-10 items-center justify-center rounded-md bg-primary/10 text-primary">
                                <CloudRain className="h-5 w-5" />
                              </div>
                              <div className="space-y-1">
                                <span className="text-base font-semibold">GCP</span>
                                <p className="text-sm text-muted-foreground">Provide service account JSON or workload identity.</p>
                              </div>
                            </label>

                            <label
                              htmlFor="azure"
                              className={`relative flex cursor-pointer items-start gap-4 rounded-lg border p-4 md:p-5 transition-colors hover:bg-muted/50 ${workflowConfig.cloudProvider === 'azure' ? 'ring-2 ring-primary border-primary' : 'border-muted'}`}
                              onClick={() => setWorkflowConfig(prev => ({ ...prev, cloudProvider: 'azure' }))}
                            >
                              <RadioGroupItem value="azure" id="azure" className="sr-only" />
                              <div className="flex h-10 w-10 items-center justify-center rounded-md bg-primary/10 text-primary">
                                <CloudSun className="h-5 w-5" />
                              </div>
                              <div className="space-y-1">
                                <span className="text-base font-semibold">Azure</span>
                                <p className="text-sm text-muted-foreground">Use federated credentials or client secret.</p>
                              </div>
                            </label>
                          </RadioGroup>
                        </div>

                        <div>
                          <Label>IAC Type</Label>
                          <RadioGroup
                            value={workflowConfig.iacType}
                            onValueChange={(value: "terraform" | "opentofu") =>
                              setWorkflowConfig(prev => ({
                                ...prev,
                                iacType: value,
                                iacVersion: value === "terraform" ? "1.5.6" : "1.9.1"
                              }))
                            }
                            className="mt-3 grid grid-cols-1 md:grid-cols-2 gap-3"
                          >
                            <label
                              htmlFor="terraform"
                              className={`relative flex cursor-pointer items-start gap-4 rounded-lg border p-4 md:p-5 transition-colors hover:bg-muted/50 ${workflowConfig.iacType === 'terraform' ? 'ring-2 ring-primary border-primary' : 'border-muted'}`}
                              onClick={() => setWorkflowConfig(prev => ({ ...prev, iacType: 'terraform', iacVersion: '1.5.6' }))}
                            >
                              <RadioGroupItem value="terraform" id="terraform" className="sr-only" />
                              <div className="flex h-10 w-10 items-center justify-center rounded-md bg-primary/10 text-primary">
                                <FileText className="h-5 w-5" />
                              </div>
                              <div className="space-y-1">
                                <span className="text-base font-semibold">Terraform</span>
                                <p className="text-sm text-muted-foreground">Use HashiCorp Terraform runtime.</p>
                              </div>
                            </label>

                            <label
                              htmlFor="opentofu"
                              className={`relative flex cursor-pointer items-start gap-4 rounded-lg border p-4 md:p-5 transition-colors hover:bg-muted/50 ${workflowConfig.iacType === 'opentofu' ? 'ring-2 ring-primary border-primary' : 'border-muted'}`}
                              onClick={() => setWorkflowConfig(prev => ({ ...prev, iacType: 'opentofu', iacVersion: '1.9.1' }))}
                            >
                              <RadioGroupItem value="opentofu" id="opentofu" className="sr-only" />
                              <div className="flex h-10 w-10 items-center justify-center rounded-md bg-primary/10 text-primary">
                                <FileText className="h-5 w-5" />
                              </div>
                              <div className="space-y-1">
                                <span className="text-base font-semibold">OpenTofu</span>
                                <p className="text-sm text-muted-foreground">Use OpenTofu community runtime.</p>
                              </div>
                            </label>
                          </RadioGroup>
                        </div>

                        <div>
                          <Label htmlFor="iacVersion">IAC Version</Label>
                          <Input
                            id="iacVersion"
                            value={workflowConfig.iacVersion}
                            onChange={(e) =>
                              setWorkflowConfig(prev => ({ ...prev, iacVersion: e.target.value }))
                            }
                            placeholder={workflowConfig.iacType === "terraform" ? "1.5.6" : "1.9.1"}
                          />
                        </div>
                      </div>

                      <div className="space-y-2">
                        <Label>Workflow Content</Label>
                        <p className="text-sm text-gray-500 mb-2">Copy and paste this content into .github/workflows/digger_workflow.yml (must be in main or master branch)</p>
                        <div className="relative">
                          <Textarea
                            value={generateWorkflowContent(workflowConfig)}
                            readOnly
                            className="font-mono h-[440px]"
                          />
                          <Button
                            size="sm"
                            variant="ghost"
                            className="absolute top-2 right-2"
                            onClick={handleCopyWorkflow}
                          >
                            <Copy className="h-4 w-4" />
                          </Button>
                        </div>
                        {/* Required secrets section */}
                        <div className="mt-4 rounded-md border bg-muted/30 p-4">
                          <h4 className="text-sm font-medium mb-2">Set these GitHub secrets</h4>
                          <div className="text-sm text-muted-foreground">
                            <p className="mb-2">Add the following secrets in your repository settings → Secrets and variables → Actions:</p>
                            {workflowConfig.cloudProvider === 'aws' && (
                              <ul className="list-disc pl-5 space-y-1">
                                <li><code className="font-mono">AWS_ACCESS_KEY_ID</code></li>
                                <li><code className="font-mono">AWS_SECRET_ACCESS_KEY</code></li>
                              </ul>
                            )}
                            {workflowConfig.cloudProvider === 'gcp' && (
                              <ul className="list-disc pl-5 space-y-1">
                                <li><code className="font-mono">GOOGLE_CLOUD_CREDENTIALS</code></li>
                              </ul>
                            )}
                            {workflowConfig.cloudProvider === 'azure' && (
                              <ul className="list-disc pl-5 space-y-1">
                                <li><code className="font-mono">AZURE_CLIENT_ID</code></li>
                                <li><code className="font-mono">AZURE_TENANT_ID</code></li>
                                <li><code className="font-mono">AZURE_SUBSCRIPTION_ID</code></li>
                              </ul>
                            )}
                            {onboardingMode === 'unit' && (
                              <div className="mt-3">
                                <p className="font-medium">For remote state</p>
                                <ul className="list-disc pl-5 space-y-1">
                                  <li>
                                    <code className="font-mono">TFE_TOKEN</code> — API token for remote state access, generate from{" "}
                                    <a
                                      href="/dashboard/settings/tokens"
                                      target="_blank"
                                      rel="noopener noreferrer"
                                      className="text-blue-500 hover:underline"
                                    >
                                      token settings page
                                    </a>
                                  </li>
                                </ul>
                              </div>
                            )}
                          </div>
                        </div>
                      </div>

                      <p className="text-sm text-gray-500 mt-4">
                        For more advanced workflow configuration options such as OIDC, check out the <a href="https://docs.digger.dev/" className="text-blue-500 hover:underline" target="_blank" rel="noopener noreferrer">Digger documentation</a>.
                      </p>
                      <div className="flex justify-end">
                        <WorkflowFileButton onClick={handleWorkflowCreate} />
                      </div>
                    </div>
                  </>

              </div>
            </TabsContent>

            <TabsContent value="digger" className="mt-6">
              <div className="space-y-4">
                <h3 className="font-medium">Configure digger.yml</h3>
                <p className="text-sm text-gray-500">
                  Configure your digger.yml file for the selected repository
                </p>

                  <>
                    <div className="relative">
                      <Textarea
                        value={diggerConfig}
                        onChange={(e) => setDiggerConfig(e.target.value)}
                        className="font-mono h-[200px]"
                        placeholder="Enter your digger.yml configuration"
                      />
                      <Button
                        size="sm"
                        variant="ghost"
                        className="absolute top-2 right-2"
                        onClick={handleCopyDigger}
                      >
                        <Copy className="h-4 w-4" />
                      </Button>
                    </div>
                    <div className="flex justify-end mt-2">
                      <DiggerYmlButton onClick={handleDiggerConfigCreate} />
                    </div>
                  </>

              </div>
            </TabsContent>

            <TabsContent value="create-unit" className="mt-6">
              <div className="space-y-4">
                <h3 className="font-medium">Create a Unit</h3>
                <p className="text-sm text-gray-500">
                  A unit is a deployable piece of Terraform that you plan and apply. You can also
                  use it to run remote jobs. If you are coming from Terraform Cloud, this is very
                  similar to a "Workspace".
                </p>

                {!steps.unitCreated && (
                  <UnitCreateForm
                    userId={userId}
                    email={email}
                    organisationId={organisationId}
                    onCreated={handleCreateUnit}
                    onBringOwnState={() => { setOnboardingMode('pr_only'); setCurrentStep('github') }}
                  />
                )}
              </div>
            </TabsContent>

            <TabsContent value="configure-unit" className="mt-6">
              <div className="space-y-4">
                {!steps.unitCreated ? (
                  <div className="text-sm text-gray-500">
                    Please create a unit first.
                    <div className="mt-4">
                      <Button onClick={() => setCurrentStep('create-unit')}>Go to Create Unit</Button>
                    </div>
                  </div>
                ) : createdUnit ? (
                  <UnitConfigureInstructions
                    unitId={createdUnit.id}
                    organisationId={organisationId}
                    publicHostname={publicHostname}
                    onGoToGithub={() => setCurrentStep('github')}
                    onGoToLocal={() => setCurrentStep('complete')}
                  />
                ) : (
                  <div className="text-sm text-gray-500">Preparing configuration…</div>
                )}
              </div>
            </TabsContent>

            <TabsContent value="complete" className="mt-6">
              <div className="space-y-4">
                <h3 className="font-medium">All set!</h3>
                <p className="text-sm text-gray-500">
                  Your unit is created and configured. If you want to test Terraform automation, create a PR
                  that touches one of your Terraform directories defined in <code className="font-mono">digger.yml</code>.
                  Digger will comment with a plan and manage runs automatically.
                  Alternatively, you can run <code className="font-mono">terraform plan</code> and <code className="font-mono">terraform apply</code> locally using the instructions in the previous step.
                </p>
                <div className="flex items-center justify-end gap-2">
                  <Button variant="outline" onClick={() => setCurrentStep('configure-unit')}>Back</Button>
                  {onComplete && (
                    <Button onClick={onComplete}>Finish</Button>
                  )}
                </div>
              </div>
            </TabsContent>
          </Tabs>
        </CardContent>
      </Card>
    </div>
  )
} 