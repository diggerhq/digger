FROM debian:bullseye-20220622-slim
RUN apt-get update \
    && DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends \
    apt-utils \
    bash \
    ca-certificates \
    curl \
    git \
    git-lfs \
    gnupg \
    groff \
    jq \
    less \
    libcap2 \
    openssh-client \
    openssl \
    python3 \
    python3-pycryptodome \
    python3-requests \
    python3-yaml \
    unzip \
    && rm -rf /var/lib/apt/lists/*


# awscli TODO install from packages
RUN curl "https://awscli.amazonaws.com/awscli-exe-linux-x86_64.zip" -o "awscliv2.zip"
RUN unzip -qq awscliv2.zip
RUN ./aws/install

# ENV DEFAULT_TERRAFORM_VERSION 1.3.8
# COPY ./install-terraform /install-terraform
# RUN /install-terraform latest
COPY bin/* /bin/

RUN pip3 install -q -r requirements.txt
COPY entrypoint.sh /entrypoint.sh
COPY code /code

ENTRYPOINT ["/entrypoint.sh"]