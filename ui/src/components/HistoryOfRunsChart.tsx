"use client"

import { Bar, BarChart, ResponsiveContainer, XAxis, YAxis, Tooltip, Legend } from "recharts"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { ChartContainer, ChartTooltipContent } from "@/components/ui/chart"

const data = [
  { date: "2023-06-01", runs: 23, failed: 4 },
  { date: "2023-06-02", runs: 19, failed: 7 },
  { date: "2023-06-03", runs: 27, failed: 2 },
  { date: "2023-06-04", runs: 18, failed: 5 },
  { date: "2023-06-05", runs: 21, failed: 6 },
  { date: "2023-06-06", runs: 24, failed: 3 },
  { date: "2023-06-07", runs: 22, failed: 8 },
]

export function HistoryOfRunsChart() {
  return (
    <Card>
      <CardHeader>
        <CardTitle>History of Runs</CardTitle>
        <CardDescription>All the jobs that happened over the last week
          These include both successful and failed jobs and both plan and apply.</CardDescription>
      </CardHeader>
      <CardContent>
        <ChartContainer
          config={{
            runs: {
              label: "Runs",
              color: "hsl(var(--chart-2))",
            },
            failed: {
              label: "Failed",
              color: "hsl(var(--chart-1))",
            },
          }}
          className="h-[300px]"
        >
          <ResponsiveContainer width="100%" height="100%">
            <BarChart data={data}>
              <XAxis dataKey="date" />
              <YAxis />
              <Tooltip content={<ChartTooltipContent />} />
              <Legend />
              <Bar dataKey="runs" stackId="a" fill="var(--color-runs)" />
              {/* <Bar dataKey="failed" stackId="a" fill="var(--color-failed)" /> */}
            </BarChart>
          </ResponsiveContainer>
        </ChartContainer>
      </CardContent>
    </Card>
  )
}

