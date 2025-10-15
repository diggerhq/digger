"use client"

import { useState } from "react"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { ArrowLeft, Play, RefreshCw, AlertTriangle, CheckCircle, Lock, Eye, Calendar } from "lucide-react"
import { DemoBookingModal } from "./demo-booking-modal"

export function DriftStatus() {
  const [isModalOpen, setIsModalOpen] = useState(false)

  const terraformCode = `# aws_s3_bucket.static_assets will be updated in-place
~ resource "aws_s3_bucket" "static_assets" {
    id                          = "static-assets-bucket"
  ~ versioning_enabled          = true -> false
  ~ server_side_encryption      = "AES256" -> null
    # (8 unchanged attributes hidden)
  }

# aws_route53_record.www will be updated in-place
~ resource "aws_route53_record" "www" {
    id                          = "Z1234567890_www.example.com_A"
  ~ ttl                         = 300 -> 3600
    # (4 unchanged attributes hidden)
  }`

  const handleLockedAction = () => {
    setIsModalOpen(true)
  }

  return (
    <div className="flex-1 flex flex-col">
      {/* Header */}
      

      {/* Demo Banner */}
      <div className="bg-gradient-to-r from-blue-600 to-purple-600 text-white p-4">
        <div className="max-w-6xl mx-auto flex items-center justify-between">
          <div className="flex items-center space-x-3">
            <Eye className="w-5 h-5" />
            <div>
              <p className="font-medium">You're viewing a demo of Drift</p>
              <p className="text-sm text-blue-100">Book a personalized demo to see the full platform in action</p>
            </div>
          </div>
          <Button
            variant="secondary"
            className="bg-white text-blue-600 hover:bg-gray-100"
          >
            <Calendar className="w-4 h-4 mr-2" />
            <a href="https://calendly.com/diggerdev/diggerdemo" target="_blank">Book Demo</a>
          </Button>
        </div>
      </div>

      {/* Main Content */}
      <div className="flex-1 p-6 bg-gray-50">
        <div className="max-w-6xl mx-auto space-y-6">
          {/* Status Overview */}
          <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
            <Card>
              <CardContent className="p-4">
                <div className="flex items-center space-x-2">
                  <AlertTriangle className="w-5 h-5 text-amber-500" />
                  <div>
                    <p className="text-sm font-medium text-gray-900">Drift Detected</p>
                    <p className="text-xs text-gray-500">3 resources affected</p>
                  </div>
                </div>
              </CardContent>
            </Card>
            <Card>
              <CardContent className="p-4">
                <div className="flex items-center space-x-2">
                  <CheckCircle className="w-5 h-5 text-green-500" />
                  <div>
                    <p className="text-sm font-medium text-gray-900">Last Scan</p>
                    <p className="text-xs text-gray-500">2 minutes ago</p>
                  </div>
                </div>
              </CardContent>
            </Card>
            <Card>
              <CardContent className="p-4">
                <div className="flex items-center space-x-2">
                  <div className="w-5 h-5 bg-blue-500 rounded-full flex items-center justify-center">
                    <span className="text-xs text-white font-bold">P</span>
                  </div>
                  <div>
                    <p className="text-sm font-medium text-gray-900">Environment</p>
                    <p className="text-xs text-gray-500">production</p>
                  </div>
                </div>
              </CardContent>
            </Card>
          </div>

          {/* Drift Status */}
          <Card>
            <CardHeader>
              <div className="flex items-center justify-between">
                <CardTitle className="text-xl font-semibold text-gray-900">Drift status</CardTitle>
                <div className="flex items-center space-x-2">
                  <Badge variant="secondary" className="bg-amber-100 text-amber-800">
                    Changes Required
                  </Badge>
                  <Badge variant="outline">Plan: +1 ~2 -0</Badge>
                </div>
              </div>
            </CardHeader>
            <CardContent>
              <div className="bg-gray-900 rounded-lg p-4 overflow-x-auto">
                <pre className="text-sm text-gray-100 font-mono leading-relaxed whitespace-pre-wrap">
                  {terraformCode}
                </pre>
              </div>
            </CardContent>
          </Card>

          {/* Resource Summary - Partially Blurred */}
          <Card className="relative">
            <CardHeader>
              <CardTitle className="text-lg font-semibold text-gray-900">Resource Changes</CardTitle>
            </CardHeader>
            <CardContent>
              <div className="space-y-3">
                <div className="flex items-center justify-between p-3 bg-green-50 rounded-lg border border-green-200">
                  <div className="flex items-center space-x-3">
                    <div className="w-2 h-2 bg-green-500 rounded-full"></div>
                    <span className="font-medium text-gray-900">aws_instance.web_server</span>
                    <Badge variant="outline" className="text-green-700 border-green-300">
                      CREATE
                    </Badge>
                  </div>
                  <span className="text-sm text-gray-600">t3.micro instance</span>
                </div>

                

                {/* Blurred section */}
                <div className="relative">
                  <div className="filter blur-md pointer-events-none">
                    <div className="flex items-center justify-between p-3 bg-amber-50 rounded-lg border border-amber-200">
                      <div className="flex items-center space-x-3">
                        <div className="w-2 h-2 bg-amber-500 rounded-full"></div>
                        <span className="font-medium text-gray-900">aws_route53_record.www</span>
                        <Badge variant="outline" className="text-amber-700 border-amber-300">
                          UPDATE
                        </Badge>
                      </div>
                      <span className="text-sm text-gray-600">TTL change: 300 → 3600</span>
                    </div>
                  </div>

                  {/* Overlay */}
                  <div className="absolute inset-0 bg-white/90 backdrop-blur-md flex items-center justify-center rounded-lg border-2 border-dashed border-blue-300">
                    <div className="text-center p-4">
                      <Lock className="w-8 h-8 text-blue-600 mx-auto mb-2" />
                      <p className="text-sm font-medium text-gray-900 mb-1">More resources available</p>
                      <p className="text-xs text-gray-600 mb-3">See detailed analysis of all infrastructure changes</p>
                      <Button size="sm"className="bg-blue-600 hover:bg-blue-700">
                        <Calendar className="w-3 h-3 mr-1" />
                        <a href="https://calendly.com/diggerdev/diggerdemo" target="_blank">Book Demo</a>
                      </Button>
                    </div>
                  </div>
                </div>
              </div>
            </CardContent>
          </Card>

          {/* Advanced Features - Locked */}
          <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
            <Card className="relative overflow-hidden">
              <div className="filter blur-sm">
                <CardHeader>
                  <CardTitle className="text-lg font-semibold text-gray-900">Compliance Checks</CardTitle>
                </CardHeader>
                <CardContent>
                  <div className="space-y-2">
                    <div className="flex items-center justify-between">
                      <span className="text-sm">Security Groups</span>
                      <Badge className="bg-green-100 text-green-800">PASS</Badge>
                    </div>
                    <div className="flex items-center justify-between">
                      <span className="text-sm">Encryption at Rest</span>
                      <Badge className="bg-red-100 text-red-800">FAIL</Badge>
                    </div>
                    <div className="flex items-center justify-between">
                      <span className="text-sm">IAM Policies</span>
                      <Badge className="bg-green-100 text-green-800">PASS</Badge>
                    </div>
                  </div>
                </CardContent>
              </div>
              
            </Card>

            <Card className="relative overflow-hidden">
              <div className="filter blur-sm">
                <CardHeader>
                  <CardTitle className="text-lg font-semibold text-gray-900">Deployment History</CardTitle>
                </CardHeader>
                <CardContent>
                  <div className="space-y-3">
                    <div className="flex items-center space-x-3">
                      <div className="w-2 h-2 bg-green-500 rounded-full"></div>
                      <div className="flex-1">
                        <p className="text-sm font-medium">Deploy #1247</p>
                        <p className="text-xs text-gray-500">2 hours ago</p>
                      </div>
                      <Badge className="bg-green-100 text-green-800">SUCCESS</Badge>
                    </div>
                    <div className="flex items-center space-x-3">
                      <div className="w-2 h-2 bg-amber-500 rounded-full"></div>
                      <div className="flex-1">
                        <p className="text-sm font-medium">Deploy #1246</p>
                        <p className="text-xs text-gray-500">1 day ago</p>
                      </div>
                      <Badge className="bg-amber-100 text-amber-800">PARTIAL</Badge>
                    </div>
                  </div>
                </CardContent>
              </div>
              
            </Card>
          </div>
        </div>
      </div>

      <DemoBookingModal isOpen={isModalOpen} onClose={() => setIsModalOpen(false)} />
    </div>
  )
}
