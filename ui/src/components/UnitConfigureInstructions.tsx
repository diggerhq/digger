import * as React from 'react'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import { Copy, Check, Github, TerminalSquare } from 'lucide-react'

function CopyButton({ content }: { content: string }) {
  const [copied, setCopied] = React.useState(false)
  const copy = () => {
    navigator.clipboard.writeText(content)
    setCopied(true)
    setTimeout(() => setCopied(false), 1500)
  }
  return (
    <Button size="icon" variant="ghost" className="absolute top-2 right-2 h-8 w-8" onClick={copy}>
      {copied ? <Check className="h-4 w-4 text-green-500" /> : <Copy className="h-4 w-4" />}
    </Button>
  )
}

type Props = {
  unitId: string
  organisationId: string
  publicHostname: string
  onGoToGithub?: () => void
  onGoToLocal?: () => void
  showNextActions?: boolean
}

export default function UnitConfigureInstructions({ unitId, organisationId, publicHostname, onGoToGithub, onGoToLocal, showNextActions = true }: Props) {
  const tfBlock = `terraform {\n  cloud {\n    hostname = "${publicHostname}"\n    organization = "${organisationId}"    \n    workspaces {\n      name = "${unitId}"\n    }\n  }\n}`
  return (
    <>
      <Card>
        <CardHeader>
          <CardTitle>Terraform Configuration</CardTitle>
          <CardDescription>Add this configuration block to your Terraform code to use this unit</CardDescription>
        </CardHeader>
        <CardContent>
          <div className="mb-4">
            <p className="text-sm text-muted-foreground mb-4">
              To use this unit in your Terraform configuration, add the following block to your Terraform code:
            </p>
            <div className="relative">
              <pre className="bg-muted p-4 rounded-lg overflow-x-auto font-mono text-sm">{tfBlock}</pre>
              <CopyButton content={tfBlock} />
            </div>
          </div>

          <div className="space-y-6">
            <div>
              <h3 className="font-semibold mb-2">1. Login to the remote backend</h3>
              <p className="text-sm text-muted-foreground mb-2">First, authenticate with the remote backend:</p>
              <div className="relative">
                <pre className="bg-muted p-4 rounded-lg overflow-x-auto font-mono text-sm">terraform login {publicHostname}</pre>
                <CopyButton content={`terraform login ${publicHostname}`} />
              </div>
            </div>

            <div>
              <h3 className="font-semibold mb-2">2. Initialize Terraform</h3>
              <p className="text-sm text-muted-foreground mb-2">After adding the configuration block above, initialize your working directory:</p>
              <div className="relative">
                <pre className="bg-muted p-4 rounded-lg overflow-x-auto font-mono text-sm">terraform init</pre>
                <CopyButton content="terraform init" />
              </div>
            </div>

            <div>
              <h3 className="font-semibold mb-2">3. Review Changes</h3>
              <p className="text-sm text-muted-foreground mb-2">Preview any changes that will be made to your infrastructure:</p>
              <div className="relative">
                <pre className="bg-muted p-4 rounded-lg overflow-x-auto font-mono text-sm">terraform plan</pre>
                <CopyButton content="terraform plan" />
              </div>
            </div>

            <div>
              <h3 className="font-semibold mb-2">4. Apply Changes</h3>
              <p className="text-sm text-muted-foreground mb-2">Apply the changes to your infrastructure:</p>
              <div className="relative">
                <pre className="bg-muted p-4 rounded-lg overflow-x-auto font-mono text-sm">terraform apply</pre>
                <CopyButton content="terraform apply" />
              </div>
            </div>

            <div className="mt-6 bg-blue-50 dark:bg-blue-950 p-4 rounded-lg">
              <h3 className="font-semibold text-blue-700 dark:text-blue-300 mb-2">Note</h3>
              <p className="text-sm text-blue-600 dark:text-blue-400">
                After completing these steps, your Terraform state will be managed by this unit. All state operations will be automatically versioned and you can roll back to previous versions if needed.
              </p>
            </div>

            {showNextActions && (
            <div className="pt-4">
              <Label className="text-right">What would you like to do next?</Label>
              <div className="mt-3 grid grid-cols-1 md:grid-cols-2 gap-3">
                <div
                  onClick={onGoToLocal}
                  className={
                    `relative flex cursor-pointer items-start gap-4 rounded-lg border p-4 md:p-5 transition-colors hover:bg-muted/50 border-muted`
                  }
                >
                  <div className="flex h-10 w-10 items-center justify-center rounded-md bg-muted text-muted-foreground">
                    <TerminalSquare className="h-5 w-5" />
                  </div>
                  <div className="space-y-1">
                    <div className="flex items-center gap-2">
                      <span className="text-base font-semibold">Run locally for now</span>
                    </div>
                    <p className="text-sm text-muted-foreground">
                      Stick to local <code className="font-mono">terraform plan</code>/<code className="font-mono">apply</code>
                      using the configuration above. You can enable PR automation later.
                    </p>
                  </div>
                </div>

                <div
                  onClick={onGoToGithub}
                  className={
                    `relative flex cursor-pointer items-start gap-4 rounded-lg border p-4 md:p-5 transition-colors hover:bg-muted/50 border-muted`
                  }
                >
                  <div className="flex h-10 w-10 items-center justify-center rounded-md bg-primary/10 text-primary">
                    <Github className="h-5 w-5" />
                  </div>
                  <div className="space-y-1">
                    <div className="flex items-center gap-2">
                      <span className="text-base font-semibold">Use PR automation</span>
                    </div>
                    <p className="text-sm text-muted-foreground">
                      Create a pull request that touches your Terraform directories. Digger will
                      comment with plans and manage runs automatically.
                    </p>
                  </div>
                </div>
              </div>
            </div>
            )}
          </div>
        </CardContent>
      </Card>
    </>
  )
}


