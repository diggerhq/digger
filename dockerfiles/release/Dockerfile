FROM hashicorp/terraform as terraform_builder

FROM alpine:latest

ARG DIGGER_VERSION=latest
ARG OS=Linux
ARG ARCH=X64

COPY --from=terraform_builder /bin/terraform /bin/terraform

RUN apk add --no-cache curl
RUN curl -sL https://github.com/diggerhq/digger/releases/download/${DIGGER_VERSION}/digger-${OS}-${ARCH}  -o digger
RUN chmod +x digger
RUN mv digger /usr/local/bin/digger


CMD ["digger", "--version"]



