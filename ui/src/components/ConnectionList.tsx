
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import { Github, Gitlab, GithubIcon as Bitbucket, ExternalLink } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Link } from "@tanstack/react-router"

const connections = [
  { id: 1, name: "Github integration", type: "github" },
  // { id: 2, name: "GitLab Integration", type: "gitlab" },
  // { id: 3, name: "Bitbucket Integration", type: "bitbucket" },
]

const iconMap = {
  github: Github,
  gitlab: Gitlab,
  bitbucket: Bitbucket,
}

export default function ConnectionList() {
  return (
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead>Type</TableHead>
          <TableHead>Name</TableHead>
          <TableHead>Repos</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {connections.map((connection) => {
          const Icon = iconMap[connection.type]
          return (
            <TableRow key={connection.id}>
              <TableCell>
                <Icon className="h-5 w-5" />
              </TableCell>
              <TableCell>{connection.name}</TableCell>
              <TableCell>
                <Button variant="ghost" asChild>
                  <Link to="/dashboard/connections/$connectionId" params={{ connectionId: String(connection.id) }}>
                    View Repos <ExternalLink className="ml-2 h-4 w-4" />
                  </Link>
                </Button>
              </TableCell>
            </TableRow>
          )
        })}
      </TableBody>
    </Table>
  )
}

