import { fetchRepos } from '@/api/backend'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Table, TableCell, TableBody, TableRow, TableHeader, TableHead } from '@/components/ui/table'
import { createFileRoute, Link, Outlet } from '@tanstack/react-router'
import { ArrowLeft, Github, Gitlab, GithubIcon as Bitbucket, ExternalLink, PlusCircle } from "lucide-react"


export const Route = createFileRoute('/_authenticated/_dashboard/dashboard/repos')({
  component: () => <Outlet />,
})

