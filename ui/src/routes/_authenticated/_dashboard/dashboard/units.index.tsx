import { Button } from '@/components/ui/button'
import { createFileRoute, Link } from '@tanstack/react-router'
import { ArrowLeft, Eye, Plus, Database } from 'lucide-react'
import {
  Table,
  TableBody,
  TableCaption,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"

export const Route = createFileRoute(
  '/_authenticated/_dashboard/dashboard/units/',
)({
  component: RouteComponent,
})

// Mock data - replace with actual data fetching
const units: Array<{
  id: string;
  size: number;
  updatedAt: Date;
  locked: boolean;
}> = [
  {
    id: "prod-vpc-network",
    size: 2457600, // ~2.4MB
    updatedAt: new Date("2025-10-16T09:30:00"),
    locked: true,
  },
  {
    id: "staging-eks-cluster",
    size: 5242880, // ~5MB
    updatedAt: new Date("2025-10-15T16:45:00"),
    locked: true,
  },
  {
    id: "shared-s3-buckets",
    size: 819200, // ~800KB
    updatedAt: new Date("2025-10-15T14:20:00"),
    locked: false,
  },
  {
    id: "dev-rds-instances",
    size: 1048576, // ~1MB
    updatedAt: new Date("2025-10-14T11:15:00"),
    locked: false,
  },
  {
    id: "monitoring-stack",
    size: 3145728, // ~3MB
    updatedAt: new Date("2025-10-13T17:30:00"),
    locked: true,
  }
]

function formatBytes(bytes: number) {
  if (bytes === 0) return '0 Bytes'
  const k = 1024
  const sizes = ['Bytes', 'KB', 'MB', 'GB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i]
}

function formatDate(date: Date) {
  return date.toLocaleDateString('en-US', {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit'
  })
}

function RouteComponent() {
  return (<>
    <div className="container mx-auto p-4">
      <div className="mb-6">
        <Button variant="ghost" asChild>
          <Link to="/dashboard/repos">
            <ArrowLeft className="mr-2 h-4 w-4" /> Back to Dashboard
          </Link>
        </Button>
      </div>

      <div className="flex justify-between items-center mb-6">
        <h1 className="text-2xl font-semibold">Units</h1>
        <Button>
          <Plus className="mr-2 h-4 w-4" />
          Create Unit
        </Button>
      </div>

      {units.length === 0 ? (
        <div className="text-center py-12">
          <div className="inline-flex h-12 w-12 items-center justify-center rounded-full bg-primary/10 mb-4">
            <Database className="h-6 w-6 text-primary" />
          </div>
          <h2 className="text-lg font-semibold mb-2">No Units Created Yet</h2>
          <p className="text-muted-foreground max-w-sm mx-auto mb-6">
            Units are equivalent to individual terraform statefiles - but they also include version history by default and you can rollback to a previous version of a unit's state.
          </p>
          <Button>
            <Plus className="mr-2 h-4 w-4" />
            Create Your First Unit
          </Button>
        </div>
      ) : (
        <Table>
        <TableCaption>Units are almost equivalent to individual terraform statefiles - but they also include version history by default and you can rollback to a previous version of a unit's state. </TableCaption>
        <TableHeader>
          <TableRow>
            <TableHead>ID</TableHead>
            <TableHead>Size</TableHead>
            <TableHead>Updated At</TableHead>
            <TableHead>Locked</TableHead>
            <TableHead className="text-right">Actions</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {units.map((unit) => (
            <TableRow key={unit.id}>
              <TableCell className="font-medium">{unit.id}</TableCell>
              <TableCell>{formatBytes(unit.size)}</TableCell>
              <TableCell>{formatDate(unit.updatedAt)}</TableCell>
              <TableCell>{unit.locked ? "Yes" : "No"}</TableCell>
              <TableCell className="text-right">
                <Button variant="ghost" size="sm" asChild className="ml-auto">
                  <Link to={`/dashboard/units/${unit.id}`} className="flex items-center justify-end w-full">
                    <Eye className="mr-2 h-4 w-4" />
                    View Details
                  </Link>
                </Button>
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
      )}
    </div>
</>)
}
