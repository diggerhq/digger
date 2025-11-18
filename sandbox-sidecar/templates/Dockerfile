FROM ubuntu:22.04

# Force rebuild - change this number to invalidate cache
ENV CACHE_BUST=20251118v3

# Install dependencies
RUN apt-get update && \
    apt-get install -y curl unzip wget ca-certificates && \
    rm -rf /var/lib/apt/lists/*

# Install Terraform 1.5.5
RUN cd /tmp && \
    curl -fsSL https://releases.hashicorp.com/terraform/1.5.5/terraform_1.5.5_linux_amd64.zip -o terraform.zip && \
    unzip terraform.zip && \
    mv terraform /usr/local/bin/terraform && \
    chmod +x /usr/local/bin/terraform && \
    rm terraform.zip && \
    echo "Terraform binary installed"

# Verify installation - this will fail the build if terraform is not installed
RUN terraform version && \
    ls -la /usr/local/bin/terraform && \
    which terraform

# Create user directory
RUN mkdir -p /home/user && chown -R 1000:1000 /home/user

# Set default user (E2B requirement)
USER 1000
WORKDIR /home/user

