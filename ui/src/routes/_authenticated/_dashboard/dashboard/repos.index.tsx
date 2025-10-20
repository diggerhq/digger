import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Table, TableCell, TableBody, TableRow, TableHeader, TableHead } from '@/components/ui/table'
import { createFileRoute, Link, Outlet } from '@tanstack/react-router'
import { ArrowLeft, Github, Gitlab, GithubIcon as Bitbucket, ExternalLink, PlusCircle } from "lucide-react"
import { getReposFn } from '@/api/orchestrator_serverFunctions'
import { Repo } from '@/api/orchestrator_types'


export const Route = createFileRoute('/_authenticated/_dashboard/dashboard/repos/')({
  component: RouteComponent,
  loader: async ({ context }) => {
    const { user, organisationId } = context;
    const repos = await getReposFn({data: {organisationId, userId: user?.id || ''}})
    return { user, repos, organisationId };
  },
})

// Mock data for repos


  
function RouteComponent() {
  const iconMap = {
    github: Github,
    gitlab: Gitlab,
    bitbucket: Bitbucket,
  }
  const { repos } = Route.useLoaderData();
  return (
  <>
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
        <CardTitle>Repositories</CardTitle>
        <CardDescription>List of repositories Connected to digger and their latest runs</CardDescription>
      </CardHeader>
      <CardContent>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Type</TableHead>
              <TableHead>Name</TableHead>
              <TableHead>URL</TableHead>
              {/* <TableHead>Latest Run</TableHead> */}
              <TableHead>Details</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {repos.map((repo : Repo) => {
              const Icon = iconMap[repo.vcs]
              return (
                <TableRow key={repo.id}>
                  <TableCell>
                    <Icon className="h-5 w-5" />
                  </TableCell>
                  <TableCell>{repo.name}</TableCell>
                  <TableCell>
                    <a
                      href={repo.repo_url}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="text-blue-500 hover:underline"
                    >
                      {repo.repo_url}
                    </a>
                  </TableCell>
                  {/* <TableCell>{repo.latestRun}</TableCell> */}
                  <TableCell>
                    <Button variant="ghost" asChild>
                      <Link to="/dashboard/repos/$repoId" params={{ repoId: String(repo.id) }}>
                        View Details <ExternalLink className="ml-2 h-4 w-4" />
                      </Link>
                    </Button>
                  </TableCell>
                </TableRow>
              )
            })}
          </TableBody>
        </Table>
        <div className="mt-4">
          <ConnectMoreRepositoriesButton />
        </div>
      </CardContent>
    </Card>
    <Outlet />  
  </div>
  </>)
}

const ConnectMoreRepositoriesButton = () => {
  return (
    <Button variant="ghost" asChild>
      <Link to="/dashboard/onboarding">
        Connect More Repositories <PlusCircle className="ml-2 h-4 w-4" />
      </Link>
    </Button>
  )
}