import { createFileRoute, Link, useLoaderData } from '@tanstack/react-router'
import { getProjectsFn } from '@/api/server_functions'
import { updateProjectFn } from '@/api/server_functions'
import { trackProjectDriftToggled } from '@/lib/analytics'
import { useToast } from "@/hooks/use-toast"
import { fetchBillingInfo } from '@/api/api'
import { useState } from 'react'
import { Project } from '@/api/types'
import { Dialog } from '@/components/ui/dialog'
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { Button } from '@/components/ui/button'
import { ArrowLeft, ExternalLink } from 'lucide-react'


export const Route = createFileRoute(
  '/_authenticated/_dashboard/dashboard/projects/',
)({
  component: RouteComponent,
  loader: async ({ context }) => {
    const { user, organisationId } = context;
    try {
      const projects = await getProjectsFn({data: {userId: user?.id || '', organisationId: organisationId || ''}})
      return { projects, user, organisationId }
    } catch (error) {
      console.error('Error loading projects:', error);
      throw error
    }

  }
})

function RouteComponent() {
    const { toast } = useToast()
    const { projects, user, organisationId } = Route.useLoaderData()
    const [projectList, setProjectList] = useState<Project[]>(projects)

    const handleDriftToggle = async (project: Project) => {
        trackProjectDriftToggled(user, organisationId, project.id.toString(), !project.drift_enabled ? 'enabled' : 'disabled')

        try {
            // Optimistically update UI
            const previousProjects = projectList
            const updatedProjects = projectList.map((p) => 
                p.id === project.id ? { ...p, drift_enabled: !p.drift_enabled } : p
            )
            setProjectList(updatedProjects)

            await updateProjectFn(
                {
                    data: {
                        projectId: project.id.toString(),
                        driftEnabled: !project.drift_enabled,
                        organisationId: organisationId || "",
                        userId: user?.id || ""
                    }
                }
            );
            
            toast({
            title: "Success",
            description: "Project drift enabled updated",
            });
        } catch (error) {
            console.error('Error updating project drift enabled:', error);
            // Revert optimistic update
            setProjectList((prev) => prev.map((p) => 
                p.id === project.id ? { ...p, drift_enabled: project.drift_enabled } : p
            ))
            toast({
            title: "Error",
            description: "Failed to update project drift status",
            variant: "destructive",
            });
        }

    };

  return (
    <div className="container mx-auto p-4">
      <div className="mb-6">
        <Button variant="ghost" asChild>
          <Link to="/dashboard/repos">
            <ArrowLeft className="mr-2 h-4 w-4" /> Back to Dashboard
          </Link>
        </Button>
      </div>
      <Card>
        <CardHeader>
          <CardTitle>Projects</CardTitle>
          <CardDescription>List of projects detected accross all repositories. Each project represents a statefile and is loaded from digger.yml in the root of the repository.</CardDescription>
        </CardHeader>
        <CardContent>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Repository</TableHead>
                <TableHead>Name</TableHead>
                <TableHead>Directory</TableHead>
                <TableHead>Drift enabled</TableHead>
                <TableHead>Drift status</TableHead>
                <TableHead>Details</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {projectList.map((project: Project) => {
                return (
                  <TableRow key={project.id}>
                    <TableCell>
                      <a href={`https://github.com/${project.repo_full_name}`} target="_blank" rel="noopener noreferrer">
                        {project.repo_full_name}
                      </a>
                    </TableCell>

                    <TableCell>{project.name}</TableCell>
                    <TableCell>
                        {project.directory}
                    </TableCell>
                    <TableCell>
                      <input
                        type="checkbox"
                        checked={project.drift_enabled}
                        onChange={() => handleDriftToggle(project)}
                        className="h-4 w-4 rounded border-gray-300"
                      />
                    </TableCell>
                    <TableCell>
                      {project.drift_status}
                    </TableCell>
                    <TableCell>
                      <Button variant="ghost" asChild size="sm">
                        <Link to="/dashboard/projects/$projectId" params={{ projectId: String(project.id) }}>
                          View Details <ExternalLink className="ml-2 h-4 w-4" />
                        </Link>
                      </Button>
                    </TableCell>
                  </TableRow>
                )
              })}
            </TableBody>
          </Table>
        </CardContent>
      </Card>

    </div>    
  )
}
