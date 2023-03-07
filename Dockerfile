FROM ghcr.io/diggerhq/tfrun-base:latest

COPY entrypoint.sh /entrypoint.sh
# COPY install-terraform.sh /install-terraform.sh
RUN /install-terraform.sh
ENTRYPOINT ["/entrypoint.sh"]
