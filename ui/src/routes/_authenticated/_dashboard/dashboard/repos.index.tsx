import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Table, TableCell, TableBody, TableRow, TableHeader, TableHead } from '@/components/ui/table'
import { createFileRoute, Link, Outlet } from '@tanstack/react-router'
import { ArrowLeft, Github, Gitlab, GithubIcon as Bitbucket, ExternalLink, PlusCircle } from "lucide-react"
import { getReposFn } from '@/api/orchestrator_serverFunctions'
import { Repo } from '@/api/orchestrator_types'
import { trackConnectMoreRepositories } from '@/lib/analytics'


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
  const { repos, user, organisationId } = Route.useLoaderData();
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
        {repos.length === 0 ? (
          <div className="text-center py-12">
            <div className="inline-flex h-12 w-12 items-center justify-center rounded-full bg-primary/10 mb-4">
              <Github className="h-6 w-6 text-primary" />
            </div>
            <h2 className="text-lg font-semibold mb-2">No Repositories Connected</h2>
            <p className="text-muted-foreground max-w-sm mx-auto mb-6">
              Connect your first repository to start running Terraform with Digger.
            </p>
            <Button asChild>
              <Link
                to="/dashboard/onboarding"
                search={{ step: 'github' } as any}
                onClick={() => trackConnectMoreRepositories(user, organisationId)}
              >
                Connect your first repository <PlusCircle className="ml-2 h-4 w-4" />
              </Link>
            </Button>
          </div>
        ) : (
          <>
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
              <ConnectMoreRepositoriesButton user={user} organisationId={organisationId} />
            </div>
          </>
        )}
      </CardContent>
    </Card>
    <Outlet />  
  </div>
  </>)
}

const ConnectMoreRepositoriesButton = ({ user, organisationId }: { user: any, organisationId: string }) => {
  return (
    <Button variant="ghost" asChild>
      <Link
        to="/dashboard/onboarding"
        search={{ step: 'github' } as any}
        onClick={() => trackConnectMoreRepositories(user, organisationId)}
      >
        Connect More Repositories <PlusCircle className="ml-2 h-4 w-4" />
      </Link>
    </Button>
  )
}