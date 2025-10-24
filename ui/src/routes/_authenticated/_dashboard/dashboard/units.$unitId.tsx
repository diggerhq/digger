import { createFileRoute, Link, useParams } from '@tanstack/react-router'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import {
  Tabs,
  TabsContent,
  TabsList,
  TabsTrigger,
} from "@/components/ui/tabs"
import { Badge } from "@/components/ui/badge"
import { ArrowLeft, Lock, Unlock, MoreVertical, History, Trash2, Download, Upload, RefreshCcw, Copy, Check, ArrowUpRight } from 'lucide-react'
import { useState } from 'react'
import { getUnitFn } from '@/api/statesman_serverFunctions'

function CopyButton({ content }: { content: string }) {
  const [copied, setCopied] = useState(false)

  const copy = () => {
    navigator.clipboard.writeText(content)
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
  }

  return (
    <Button
      size="icon"
      variant="ghost"
      className="absolute top-2 right-2 h-8 w-8"
      onClick={copy}
    >
      {copied ? (
        <Check className="h-4 w-4 text-green-500" />
      ) : (
        <Copy className="h-4 w-4" />
      )}
    </Button>
  )
}

export const Route = createFileRoute(
  '/_authenticated/_dashboard/dashboard/units/$unitId',
)({
  component: RouteComponent,
  loader: async ({ context, params: {unitId} }) => {
    const { user, organisationId } = context;
    const unitData = await getUnitFn({data: {organisationId: organisationId || '', userId: user?.id || '', email: user?.email || '', unitId: unitId}})
    console.log(unitData)
    return { unitData: unitData, user, organisationId }
  }
})

// Mock data - replace with actual data fetching
const mockUnit = {
  id: "prod-vpc-network",
  size: 2457600,
  updatedAt: new Date("2025-10-16T09:30:00"),
  locked: true,
  lockedBy: "john.doe@company.com",
  status: "up-to-date", // Can be "up-to-date" or "needs re-apply"
  version: "v12",
  versions: [
    { version: "v12", timestamp: new Date("2025-10-16T09:30:00"), author: "john.doe@company.com", isLatest: true },
    { version: "v11", timestamp: new Date("2025-10-15T16:45:00"), author: "jane.smith@company.com", isLatest: false },
    { version: "v10", timestamp: new Date("2025-10-14T14:20:00"), author: "john.doe@company.com", isLatest: false },
  ],
  dependencies: [
    { name: "shared-networking", status: "up-to-date" },
    { name: "security-groups", status: "needs re-apply" },
  ]
}

function formatBytes(bytes: number) {
  if (bytes === 0) return '0 Bytes'
  const k = 1024
  const sizes = ['Bytes', 'KB', 'MB', 'GB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i]
}

function formatDate(date: Date) {
  return date
  return date.toLocaleDateString('en-US', {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit'
  })
}

function RouteComponent() {
  const data = Route.useLoaderData()
  const { unitData, organisationId } = data
  console.log(unitData)
  const unit = unitData
  if (!unit.versions) {
    unit.versions = []
  }
  if (!unit.dependencies) {
    unit.dependencies = []
  }
  return (
    <div className="container mx-auto p-4">
      <div className="mb-6 flex items-center justify-between">
        <div className="flex items-center gap-4">
          <Button variant="ghost" asChild>
            <Link to="/dashboard/units">
              <ArrowLeft className="mr-2 h-4 w-4" /> Back to Units
            </Link>
          </Button>
          <div className="flex gap-2">
              <Badge variant={unit.status === "up-to-date" ? "secondary" : "destructive"}>
                {unit.status === "up-to-date" ? (
                  <Check className="mr-2 h-3 w-3" />
                ) : (
                  <RefreshCcw className="mr-2 h-3 w-3" />
                )}
                {unit.status === "up-to-date" ? "Up-to-date" : "Needs re-apply"}
              </Badge>
            <Badge variant={unit.locked ? "destructive" : "secondary"}>
              {unit.locked ? <Lock className="mr-2 h-3 w-3" /> : <Unlock className="mr-2 h-3 w-3" />}
              {unit.locked ? "Locked" : "Unlocked"}
            </Badge>
          </div>
        </div>
        
        <div className="flex items-center gap-2">
        <Button variant="outline" className="gap-2">
            <Download className="h-4 w-4" />
            Download Latest State
          </Button>
          <Button variant="outline" className="gap-2">
            <Unlock className="h-4 w-4" />
            Unlock
          </Button>

        </div>
      </div>

      <div className="grid gap-6">
        <Card>
          <CardHeader>
            <CardTitle className="text-2xl">{unit.name}</CardTitle>
            <CardDescription>
              ID: {unit.id}
            </CardDescription>
            <CardDescription>
              Version {unit.version} • Last updated {formatDate(unit.updated)} • {formatBytes(unit.size)}
            </CardDescription>
          </CardHeader>
        </Card>

        <Tabs defaultValue="setup">
          <TabsList>
            <TabsTrigger value="setup">Setup</TabsTrigger>
            <TabsTrigger value="versions">State versions</TabsTrigger>
            <TabsTrigger value="dependencies">Dependencies</TabsTrigger>
            <TabsTrigger value="settings">Settings</TabsTrigger>
          </TabsList>

          <TabsContent value="setup" className="mt-6">
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
                    <pre className="bg-muted p-4 rounded-lg overflow-x-auto font-mono text-sm">
{`terraform {
  cloud {
    hostname = "mo-opentaco-test.ngrok.app"
    organization = "${organisationId}"    
    workspaces {
      name = "${unit.id}"
    }
  }
}`}
                    </pre>
                    <CopyButton 
                      content={`terraform {
  cloud {
    hostname = "mo-opentaco-test.ngrok.app"
    organization = "opentaco"    
    workspaces {
      name = "momo"
    }
  }
}`} 
                    />
                  </div>
                </div>

                <div className="space-y-6">
                  <div>
                    <h3 className="font-semibold mb-2">1. Login to the remote backend</h3>
                    <p className="text-sm text-muted-foreground mb-2">
                      First, authenticate with the remote backend:
                    </p>
                    <div className="relative">
                      <pre className="bg-muted p-4 rounded-lg overflow-x-auto font-mono text-sm">terraform login mo-opentaco-test.ngrok.app</pre>
                      <CopyButton content="terraform login mo-opentaco-test.ngrok.app" />
                    </div>
                  </div>

                  <div>
                    <h3 className="font-semibold mb-2">2. Initialize Terraform</h3>
                    <p className="text-sm text-muted-foreground mb-2">
                      After adding the configuration block above, initialize your working directory:
                    </p>
                    <div className="relative">
                      <pre className="bg-muted p-4 rounded-lg overflow-x-auto font-mono text-sm">terraform init</pre>
                      <CopyButton content="terraform init" />
                    </div>
                  </div>

                  <div>
                    <h3 className="font-semibold mb-2">3. Review Changes</h3>
                    <p className="text-sm text-muted-foreground mb-2">
                      Preview any changes that will be made to your infrastructure:
                    </p>
                    <div className="relative">
                      <pre className="bg-muted p-4 rounded-lg overflow-x-auto font-mono text-sm">terraform plan</pre>
                      <CopyButton content="terraform plan" />
                    </div>
                  </div>

                  <div>
                    <h3 className="font-semibold mb-2">4. Apply Changes</h3>
                    <p className="text-sm text-muted-foreground mb-2">
                      Apply the changes to your infrastructure:
                    </p>
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
                </div>
              </CardContent>
            </Card>
          </TabsContent>
          
          <TabsContent value="versions" className="mt-6">
            <Card>
              <CardHeader>
                <CardTitle>Version History</CardTitle>
                <CardDescription>Previous versions of this unit</CardDescription>
              </CardHeader>
              <CardContent>
                <div className="space-y-4">
                  {unit.versions.map((version) => (
                    <div key={version.version} className="flex items-center justify-between border-b pb-4 last:border-0">
                      <div>
                        <div className="flex items-center gap-2">
                          <span className="font-medium">{version.version}</span>
                          {version.isLatest && (
                            <Badge variant="secondary" className="text-xs">
                              Latest
                            </Badge>
                          )}
                        </div>
                        <div className="text-sm text-muted-foreground">
                          {formatDate(version.timestamp)} by {version.author}
                        </div>
                      </div>
                      <div className="flex items-center gap-2">
                        {!version.isLatest && (
                          <Button variant="outline" size="sm">
                            <History className="mr-2 h-4 w-4" />
                            Restore
                          </Button>
                        )}
                        <Button variant="outline" size="sm">
                          <Download className="mr-2 h-4 w-4" />
                          Download
                        </Button>
                      </div>
                    </div>
                  ))}
                </div>
              </CardContent>
            </Card>
          </TabsContent>

          <TabsContent value="dependencies" className="mt-6">
            <Card>
              <CardHeader>
                <CardTitle>Dependencies</CardTitle>
                <CardDescription>Units this unit depends on</CardDescription>
              </CardHeader>
              <CardContent>
                <div className="space-y-4">
                  {unit.dependencies.map((dep) => (
                    <div key={dep.name} className="flex items-center justify-between border-b pb-4 last:border-0">
                      <div>
                        <Link 
                          to="/dashboard/units/$unitId"
                          params={{ unitId: dep.name }}
                          className="font-medium hover:underline inline-flex items-center gap-1"
                        >
                          {dep.name}
                          <ArrowUpRight className="h-4 w-4" />
                        </Link>
                        <div className="text-sm text-muted-foreground">
                          Status: {dep.status}
                        </div>
                      </div>
                      <Badge variant={dep.status === "up-to-date" ? "secondary" : "destructive"}>
                        {dep.status}
                      </Badge>
                    </div>
                  ))}
                </div>
              </CardContent>
            </Card>
          </TabsContent>

          <TabsContent value="settings" className="mt-6">
            <Card>
              <CardHeader>
                <CardTitle>Dangerous Operations</CardTitle>
                <CardDescription>These operations can potentially cause data loss. Use with caution.</CardDescription>
              </CardHeader>
              <CardContent>
                <div className="space-y-8">
                  <div>
                    <h3 className="text-sm font-medium mb-2">Force Push State</h3>
                    <p className="text-sm text-muted-foreground mb-4">
                      This will overwrite the remote state with your local state, ignoring any locks or version history.
                      Only use this if you are absolutely sure your local state is correct.
                    </p>
                    <Button variant="destructive" className="gap-2">
                      <Upload className="h-4 w-4" />
                      Force Push State
                    </Button>
                  </div>

                  <div className="pt-4 border-t">
                    <h3 className="text-sm font-medium mb-2">Delete Unit</h3>
                    <p className="text-sm text-muted-foreground mb-4">
                      This will permanently delete this unit and all of its version history. 
                      This action cannot be undone. Make sure to back up any important state before proceeding.
                    </p>
                    <Button variant="destructive" className="gap-2">
                      <Trash2 className="h-4 w-4" />
                      Delete Unit
                    </Button>
                  </div>
                </div>
              </CardContent>
            </Card>
          </TabsContent>
        </Tabs>
      </div>
    </div>
  )
}
