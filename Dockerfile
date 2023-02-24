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

ENV TERRAGRUNT_VERSION v0.36.10
RUN curl -fsSL -o /usr/local/bin/terragrunt "https://github.com/terrateamio/packages/raw/main/terragrunt/terragrunt_${TERRAGRUNT_VERSION}_linux_amd64" \
    && chmod +x /usr/local/bin/terragrunt

ENV AWSCLI_VERSION 2.9.22
RUN mkdir /tmp/awscli \
    && curl -fsSL -o /tmp/awscli/awscli.zip "https://github.com/terrateamio/packages/raw/main/aws/awscli-exe-linux-x86_64-${AWSCLI_VERSION}.zip" \
    && unzip -q /tmp/awscli/awscli.zip -d /tmp/awscli/ \
    && /tmp/awscli/aws/install > /dev/null \
    && rm -rf /tmp/awscli

COPY ./bin/ /usr/local/bin
ENV DEFAULT_TERRAFORM_VERSION 1.3.8
COPY ./install-terraform /install-terraform
RUN /install-terraform latest

RUN pip3 install -q -r requirements.txt
COPY entrypoint.sh /entrypoint.sh
COPY code /code

ENTRYPOINT ["/entrypoint.sh"]