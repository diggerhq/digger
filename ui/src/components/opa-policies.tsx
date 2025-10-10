"use client"

import { useState } from "react"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Textarea } from "@/components/ui/textarea"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import {
  ArrowLeft,
  Save,
  Play,
  Lock,
  Eye,
  Calendar,
  Shield,
  Code,
  CheckCircle,
  AlertTriangle,
  Plus,
} from "lucide-react"
import { DemoBookingModal } from "./demo-booking-modal"

export function OpaPolicies() {
  const [isModalOpen, setIsModalOpen] = useState(false)
  const [activeTab, setActiveTab] = useState("access")

  const sampleAccessPolicy = `package terraform.analysis

import rego.v1

# Allow S3 buckets only in specific regions
deny contains msg if {
    input.resource_type == "aws_s3_bucket"
    not input.config.region in ["us-east-1", "us-west-2"]
    msg := "S3 buckets must be created in approved regions"
}

# Require encryption for S3 buckets
deny contains msg if {
    input.resource_type == "aws_s3_bucket"
    not input.config.server_side_encryption_configuration
    msg := "S3 buckets must have encryption enabled"
}`

  const samplePlanPolicy = `package terraform.plan

import rego.v1

# Prevent deletion of production resources
deny contains msg if {
    input.resource_changes[_].change.actions[_] == "delete"
    input.resource_changes[_].address contains "production"
    msg := "Cannot delete production resources"
}`

  const handleLockedAction = () => {
    setIsModalOpen(true)
  }

  const policyTemplates = [
    {
      name: "AWS Security Baseline",
      description: "Enforce AWS security best practices",
      category: "Security",
      locked: false,
    },
    {
      name: "Cost Control Policies",
      description: "Prevent expensive resource creation",
      category: "Cost",
      locked: true,
    },
    {
      name: "Compliance Framework",
      description: "SOC 2 and GDPR compliance rules",
      category: "Compliance",
      locked: true,
    },
    {
      name: "Multi-Cloud Governance",
      description: "Policies for AWS, Azure, and GCP",
      category: "Governance",
      locked: true,
    },
  ]

  return (
    <div className="flex-1 flex flex-col">
      {/* Header */}
      

      {/* Demo Banner */}
      <div className="bg-gradient-to-r from-blue-600 to-purple-600 text-white p-4">
        <div className="max-w-6xl mx-auto flex items-center justify-between">
          <div className="flex items-center space-x-3">
            <Eye className="w-5 h-5" />
            <div>
              <p className="font-medium">You're viewing a demo of OPA Policy Management</p>
              <p className="text-sm text-blue-100">
                Book a demo to see advanced policy testing, validation, and compliance features
              </p>
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
          <div>
            <h1 className="text-3xl font-bold text-gray-900 mb-8">OPA Policies</h1>
          </div>

          {/* Policy Stats */}
          <div className="grid grid-cols-1 md:grid-cols-4 gap-4">
            <Card>
              <CardContent className="p-4">
                <div className="flex items-center space-x-2">
                  <Shield className="w-5 h-5 text-blue-500" />
                  <div>
                    <p className="text-sm font-medium text-gray-900">Active Policies</p>
                    <p className="text-2xl font-bold text-gray-900">12</p>
                  </div>
                </div>
              </CardContent>
            </Card>
            <Card>
              <CardContent className="p-4">
                <div className="flex items-center space-x-2">
                  <CheckCircle className="w-5 h-5 text-green-500" />
                  <div>
                    <p className="text-sm font-medium text-gray-900">Compliant Resources</p>
                    <p className="text-2xl font-bold text-gray-900">1,247</p>
                  </div>
                </div>
              </CardContent>
            </Card>
            <Card>
              <CardContent className="p-4">
                <div className="flex items-center space-x-2">
                  <AlertTriangle className="w-5 h-5 text-amber-500" />
                  <div>
                    <p className="text-sm font-medium text-gray-900">Policy Violations</p>
                    <p className="text-2xl font-bold text-gray-900">3</p>
                  </div>
                </div>
              </CardContent>
            </Card>
            <Card>
              <CardContent className="p-4">
                <div className="flex items-center space-x-2">
                  <Code className="w-5 h-5 text-purple-500" />
                  <div>
                    <p className="text-sm font-medium text-gray-900">Policy Templates</p>
                    <p className="text-2xl font-bold text-gray-900">24</p>
                  </div>
                </div>
              </CardContent>
            </Card>
          </div>

          {/* Policy Tabs */}
          <Card>
            <CardHeader>
              <CardTitle className="text-xl font-semibold text-gray-900">Policy Management</CardTitle>
            </CardHeader>
            <CardContent>
              <Tabs value={activeTab} onValueChange={setActiveTab}>
                <TabsList className="grid w-full grid-cols-2">
                  <TabsTrigger value="access">Access Policies</TabsTrigger>
                  <TabsTrigger value="plan" disabled className="opacity-50">
                    Plan Policies
                  </TabsTrigger>
                </TabsList>

                <TabsContent value="access" className="space-y-6">
                  <div>
                    <h3 className="text-lg font-semibold text-gray-900 mb-2">Access Policies</h3>
                    <p className="text-gray-600 mb-4">
                      Define who can access which resources. These policies control user permissions. You can see
                      additional documentation on official{" "}
                      <a href="#" className="text-blue-600 hover:underline">
                        digger library
                      </a>{" "}
                      or use the{" "}
                      <a href="#" className="text-blue-600 hover:underline" onClick={handleLockedAction}>
                        policy generator
                      </a>
                      .
                    </p>

                    <div className="space-y-4">
                      <Textarea
                        placeholder="Enter your access policy in Rego format..."
                        value={sampleAccessPolicy}
                        className="min-h-[300px] font-mono text-sm"
                        readOnly
                      />

                      {/* Locked Advanced Editor */}
                      <div className="relative">
                        <div className="filter blur-sm pointer-events-none">
                          <div className="bg-gray-900 rounded-lg p-4">
                            <div className="flex items-center justify-between mb-2">
                              <span className="text-green-400 text-sm">Advanced Policy Editor</span>
                              <div className="flex space-x-2">
                                <div className="w-3 h-3 bg-red-500 rounded-full"></div>
                                <div className="w-3 h-3 bg-yellow-500 rounded-full"></div>
                                <div className="w-3 h-3 bg-green-500 rounded-full"></div>
                              </div>
                            </div>
                            <pre className="text-green-400 text-sm">
                              {`# Advanced policy with auto-completion
package terraform.advanced

import rego.v1

# Multi-environment policy validation
validate_environment(env) if {
    env in ["dev", "staging", "prod"]
}

# Resource tagging enforcement
required_tags := ["Environment", "Owner", "Project"]`}
                            </pre>
                          </div>
                        </div>

                        <div className="absolute inset-0 bg-white/90 backdrop-blur-md flex items-center justify-center rounded-lg border-2 border-dashed border-blue-300">
                          <div className="text-center p-6">
                            <Lock className="w-10 h-10 text-blue-600 mx-auto mb-3" />
                            <p className="font-medium text-gray-900 mb-2">Advanced Policy Editor</p>
                            <p className="text-sm text-gray-600 mb-4">
                              Syntax highlighting, auto-completion, and policy validation
                            </p>


                          </div>
                        </div>
                      </div>
                    </div>
                  </div>
                </TabsContent>

                <TabsContent value="plan" className="space-y-6">
                  <div>
                    <h3 className="text-lg font-semibold text-gray-900 mb-2">Plan Policies</h3>
                    <p className="text-gray-600 mb-4">
                      Control what changes can be made to your infrastructure during terraform plan and apply
                      operations.
                    </p>
                    <Textarea
                      placeholder="Enter your plan policy in Rego format..."
                      value={samplePlanPolicy}
                      className="min-h-[300px] font-mono text-sm"
                      readOnly
                    />
                  </div>
                </TabsContent>
              </Tabs>
            </CardContent>
          </Card>

          {/* Policy Templates */}
          <Card>
            <CardHeader>
              <CardTitle className="text-lg font-semibold text-gray-900">Policy Templates</CardTitle>
            </CardHeader>
            <CardContent>
              <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                {policyTemplates.map((template, index) => (
                  <div key={index} className="relative">
                    {template.locked ? (
                      <>
                        <div className="filter blur-sm pointer-events-none">
                          <div className="p-4 border border-gray-200 rounded-lg hover:border-gray-300 transition-colors">
                            <div className="flex items-center justify-between mb-2">
                              <h4 className="font-medium text-gray-900">{template.name}</h4>
                              <Badge variant="outline">{template.category}</Badge>
                            </div>
                            <p className="text-sm text-gray-600">{template.description}</p>
                          </div>
                        </div>
                        
                      </>
                    ) : (
                      <div className="p-4 border border-gray-200 rounded-lg hover:border-gray-300 transition-colors cursor-pointer">
                        <div className="flex items-center justify-between mb-2">
                          <h4 className="font-medium text-gray-900">{template.name}</h4>
                          <Badge variant="outline">{template.category}</Badge>
                        </div>
                        <p className="text-sm text-gray-600">{template.description}</p>
                        
                      </div>
                    )}
                  </div>
                ))}
              </div>
            </CardContent>
          </Card>

          {/* Advanced Features - Locked */}
          <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
            <Card className="relative overflow-hidden">
              <div className="filter blur-sm">
                <CardHeader>
                  <CardTitle className="text-lg font-semibold text-gray-900">Policy Testing</CardTitle>
                </CardHeader>
                <CardContent>
                  <div className="space-y-3">
                    <div className="flex items-center justify-between">
                      <span className="text-sm">Unit Tests</span>
                      <Badge className="bg-green-100 text-green-800">12 PASSED</Badge>
                    </div>
                    <div className="flex items-center justify-between">
                      <span className="text-sm">Integration Tests</span>
                      <Badge className="bg-green-100 text-green-800">8 PASSED</Badge>
                    </div>
                    <div className="flex items-center justify-between">
                      <span className="text-sm">Coverage</span>
                      <Badge className="bg-blue-100 text-blue-800">94%</Badge>
                    </div>
                  </div>
                </CardContent>
              </div>
              
            </Card>

            <Card className="relative overflow-hidden">
              <div className="filter blur-sm">
                <CardHeader>
                  <CardTitle className="text-lg font-semibold text-gray-900">Policy Analytics</CardTitle>
                </CardHeader>
                <CardContent>
                  <div className="space-y-3">
                    <div className="flex items-center justify-between">
                      <span className="text-sm">Policy Violations</span>
                      <span className="text-2xl font-bold text-red-600">3</span>
                    </div>
                    <div className="flex items-center justify-between">
                      <span className="text-sm">Resources Evaluated</span>
                      <span className="text-2xl font-bold text-blue-600">1,247</span>
                    </div>
                    <div className="flex items-center justify-between">
                      <span className="text-sm">Compliance Score</span>
                      <span className="text-2xl font-bold text-green-600">97%</span>
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
