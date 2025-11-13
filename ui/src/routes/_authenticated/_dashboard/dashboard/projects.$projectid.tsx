import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { createFileRoute, useLoaderData, Link } from '@tanstack/react-router'
import { AlertTriangle, CheckCircle, Clock, FolderOpen, GitBranch, Calendar, ArrowLeft } from 'lucide-react'
import { getProjectFn } from '@/api/orchestrator_serverFunctions'
import { Button } from '@/components/ui/button'
import { DetailsSkeleton } from '@/components/LoadingSkeleton'

const getDriftStatusBadge = (status: string) => {
  switch (status?.toLowerCase()) {
    case 'new drift':
      return <Badge variant="destructive" className="bg-red-100 text-red-800">Drifted</Badge>
    case 'no drift':
      return <Badge variant="secondary" className="bg-green-100 text-green-800">No Drift</Badge>
    case 'pending':
      return <Badge variant="outline" className="bg-yellow-100 text-yellow-800">Pending</Badge>
    case 'error':
      return <Badge variant="destructive">Error</Badge>
    default:
      return <Badge variant="outline">Unknown</Badge>
  }
}

const getDriftIcon = (status: string) => {
  switch (status?.toLowerCase()) {
    case 'drifted':
      return <AlertTriangle className="h-5 w-5 text-red-500" />
    case 'no_drift':
      return <CheckCircle className="h-5 w-5 text-green-500" />
    case 'pending':
      return <Clock className="h-5 w-5 text-yellow-500" />
    default:
      return <AlertTriangle className="h-5 w-5 text-gray-500" />
  }
}

export const Route = createFileRoute(
  '/_authenticated/_dashboard/dashboard/projects/$projectid',
)({
  component: RouteComponent,
  pendingComponent: () => (
    <div className="container mx-auto p-4">
      <div className="mb-6"><div className="h-10 w-32" /></div>
      <DetailsSkeleton />
    </div>
  ),
  loader: async ({ context, params: {projectid} }) => {
    const { user, organisationId } = context;
    const project = await getProjectFn({data: {projectId: projectid, organisationId, userId: user?.id || ''}})

    return { project }
  }
})


function RouteComponent() {
  const { project } = Route.useLoaderData()

  return (
    <div className="container mx-auto p-4">
      
      <div className="mb-6">
        <Button variant="ghost" asChild>
          <Link to="/dashboard/projects">
            <ArrowLeft className="mr-2 h-4 w-4" /> Back to Projects
          </Link>
        </Button>
      </div>

      <div className="space-y-6">
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-3">
            <FolderOpen className="h-6 w-6" />
            {project.name}
          </CardTitle>
          <CardDescription>
            Project details and drift status monitoring
          </CardDescription>
        </CardHeader>
        <CardContent>
          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            <div className="flex items-center gap-2">
              <GitBranch className="h-4 w-4 text-gray-500" />
              <span className="text-sm font-medium">Repository:</span>
              <a 
                href={`https://github.com/${project.repo_full_name}`} 
                target="_blank" 
                rel="noopener noreferrer"
                className="text-blue-600 hover:underline"
              >
                {project.repo_full_name}
              </a>
            </div>
            <div className="flex items-center gap-2">
              <FolderOpen className="h-4 w-4 text-gray-500" />
              <span className="text-sm font-medium">Directory:</span>
              <code className="bg-gray-100 px-2 py-1 rounded text-sm">
                {project.directory}
              </code>
            </div>
          </div>
        </CardContent>
      </Card>

      {/* Drift Status Overview */}
      <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
        <Card>
          <CardContent className="p-4">
            <div className="flex items-center space-x-3">
              {getDriftIcon(project.drift_status)}
              <div>
                <p className="text-sm font-medium text-gray-900">Drift Status</p>
                <div className="mt-1">
                  {getDriftStatusBadge(project.drift_status)}
                </div>
              </div>
            </div>
          </CardContent>
        </Card>
        
        <Card>
          <CardContent className="p-4">
            <div className="flex items-center space-x-3">
              <Calendar className="h-5 w-5 text-blue-500" />
              <div>
                <p className="text-sm font-medium text-gray-900">Last Check</p>
                <p className="text-xs text-gray-500">
                  {project.latest_drift_check 
                    ? new Date(project.latest_drift_check).toLocaleString()
                    : 'Never'
                  }
                </p>
              </div>
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardContent className="p-4">
            <div className="flex items-center space-x-3">
              <div className="w-5 h-5 bg-blue-500 rounded-full flex items-center justify-center">
                <span className="text-xs text-white font-bold">M</span>
              </div>
              <div>
                <p className="text-sm font-medium text-gray-900">Monitoring</p>
                <p className="text-xs text-gray-500">
                  {project.drift_enabled ? 'Enabled' : 'Disabled'}
                </p>
              </div>
            </div>
          </CardContent>
        </Card>
      </div>

      {/* Terraform Plan */}
      {project.drift_terraform_plan && (
        <Card>
          <CardHeader>
            <CardTitle className="text-lg">Terraform Plan</CardTitle>
            <CardDescription>
              Latest drift detection plan output
            </CardDescription>
          </CardHeader>
          <CardContent>
            <div className="bg-gray-900 rounded-lg p-4 overflow-x-auto">
              <pre className="text-sm text-gray-100 font-mono leading-relaxed whitespace-pre-wrap">
                {project.drift_terraform_plan}
              </pre>
            </div>
          </CardContent>
        </Card>
      )}

      {/* No Terraform Plan Available */}
      {!project.drift_terraform_plan && (
        <Card>
          <CardHeader>
            <CardTitle className="text-lg">Terraform Plan</CardTitle>
            <CardDescription>
              Latest drift detection plan output
            </CardDescription>
          </CardHeader>
          <CardContent>
            <div className="text-center py-8 text-gray-500">
              <AlertTriangle className="h-12 w-12 mx-auto mb-4" />
              <p className="text-lg font-medium mb-2">No plan available</p>
              <p className="text-sm">
                {project.drift_enabled 
                  ? 'Terraform plan will appear here after the next drift check'
                  : 'Enable drift monitoring to see terraform plans'
                }
              </p>
            </div>
          </CardContent>
        </Card>
      )}
    </div>
  </div>
  )
}
