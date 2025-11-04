import { Card, CardTitle, CardHeader, CardContent, CardDescription } from '@/components/ui/card'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { createFileRoute, Link, Outlet } from '@tanstack/react-router'
import { Button } from '@/components/ui/button'
import { ArrowLeft } from 'lucide-react'
import JobsTable from '@/components/dashboard/JobsTable'
import { getRepoDetailsFn } from '@/api/orchestrator_serverFunctions'
import { PageLoading } from '@/components/LoadingSkeleton'

export const Route = createFileRoute('/_authenticated/_dashboard/dashboard/repos/$repoId')({
  component: RouteComponent,
  pendingComponent: PageLoading,
  loader: async ({ params: {repoId}, context }) => {
    const { user, organisationId } = context;
    const { repo, allJobs } = await getRepoDetailsFn({data: {repoId, organisationId, userId: user?.id || ''}})
    return { repo, allJobs }
  }
})

function RouteComponent() {
  const { repo } = Route.useLoaderData()
  const { allJobs } = Route.useLoaderData()
  return (
    <div className="container mx-auto p-4">
      
      <div className="mb-6">
        <Button variant="ghost" asChild>
          <Link to={`/dashboard/repos`}>
            <ArrowLeft className="mr-2 h-4 w-4" /> Back to Repos
          </Link>
        </Button>
      </div>
      <Card className="mb-6">
        <CardHeader>
          <CardTitle>{repo.repo_full_name}</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="flex gap-8">
            <div>
              <Link to={repo.repo_url} target="_blank" className="text-blue-600 hover:underline">{repo.name}</Link>
            </div>
            <div>
              <span><b>VCS:</b> {repo.vcs}</span>
            </div>
            <div>
              <span><b>Connected on:</b> {new Date(repo.created_at).toLocaleString()}</span>
            </div>
          </div>
        </CardContent>
      </Card>
      <JobsTable jobs={allJobs} />
    </div>
  )   
}


