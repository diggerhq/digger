// template.ts
import { Template } from "e2b";

const dockerfileContent = `
FROM ubuntu:22.04

# Set Terraform version
ARG TF_VERSION=1.5.5

# Avoid interactive prompts
ENV DEBIAN_FRONTEND=noninteractive

# Install dependencies
RUN apt-get update && \\
    apt-get install -y --no-install-recommends \\
      curl \\
      unzip \\
      ca-certificates \\
      git \\
      bash && \\
    rm -rf /var/lib/apt/lists/*

# Download and install Terraform
RUN curl -fsSL https://releases.hashicorp.com/terraform/\${TF_VERSION}/terraform_\${TF_VERSION}_linux_amd64.zip \\
    -o terraform.zip && \\
    unzip terraform.zip && \\
    mv terraform /usr/local/bin/ && \\
    chmod +x /usr/local/bin/terraform && \\
    rm terraform.zip

# Verify installation
RUN terraform version

# Default working directory
WORKDIR /workspace
`;

export const template = Template()
  .fromDockerfile(dockerfileContent)
