FROM diggerhq/tfrun-base:latest

COPY entrypoint.sh /entrypoint.sh
# COPY install-terraform.sh /install-terraform.sh
COPY code /code
RUN pip install -q -r /code/requirements.txt
RUN /install-terraform.sh
ENTRYPOINT ["/entrypoint.sh"]
