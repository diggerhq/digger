import { Table } from "../ui/table"
import { useState } from "react"
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from "../ui/card"
import { TableHead, TableRow } from "../ui/table"
import { TableHeader, TableBody } from "../ui/table"
import { TableCell } from "../ui/table"
import { Check } from "lucide-react"
import { X } from "lucide-react"
import { Button } from "../ui/button"

interface Job {
    ID: number
    DiggerJobID: string
    PRNumber: number
    CreatedAt: string
    Status: string
  }
  
  interface JobsTableProps {
    jobs: Job[]
  }
  
  export default function JobsTable({ jobs }: JobsTableProps) {
    const ITEMS_PER_PAGE = 10
    const [currentPage, setCurrentPage] = useState(1)
    const totalPages = Math.ceil(jobs.length / ITEMS_PER_PAGE)
    const startIndex = (currentPage - 1) * ITEMS_PER_PAGE
    const endIndex = startIndex + ITEMS_PER_PAGE
    const currentJobs = jobs.slice(startIndex, endIndex)
  
    return (
      <Card>
        <CardHeader>
          <CardTitle>Recent Jobs</CardTitle>
          <CardDescription>List of jobs that have run for this repository, you can also view the details of each job and the terraform outputs for it</CardDescription>
        </CardHeader>
        <CardContent>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>JobID</TableHead>
                <TableHead>PR Number</TableHead>
                <TableHead>Triggered At</TableHead>
                <TableHead>Status</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {currentJobs.map((job) => (
                <TableRow key={job.ID}>
                  <TableCell>{job.DiggerJobID}</TableCell>
                  <TableCell>{job.PRNumber}</TableCell>
                  <TableCell>{new Date(job.CreatedAt).toLocaleString()}</TableCell>
                  <TableCell>
                    {job.Status === "failed" ? (
                      <span className="flex items-center text-red-500">
                      <X className="mr-2 h-4 w-4" /> Failed
                      </span>
                    ) : (
                      <span className="flex items-center text-green-500">
                        <Check className="mr-2 h-4 w-4" /> {job.Status}
                      </span>
                    )}
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
          <div className="flex justify-between items-center mt-4">
            <Button 
              onClick={() => setCurrentPage((prev) => Math.max(prev - 1, 1))} 
              disabled={currentPage === 1}
            >
              Previous
            </Button>
            <span>
              Page {currentPage} of {totalPages}
            </span>
            <Button
              onClick={() => setCurrentPage((prev) => Math.min(prev + 1, totalPages))}
              disabled={currentPage === totalPages}
            >
              Next
            </Button>
          </div>
        </CardContent>
      </Card>
    )
  } 