FROM ghcr.io/diggerhq/tfrun-base:latest

COPY code /code
COPY entrypoint.sh /entrypoint.sh
COPY install-terraform.sh /install-terraform.sh
RUN /install-terraform.sh
