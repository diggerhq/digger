echo 'version: '"'"'3.7'"'"'

services:
  postgres:
    image: postgres:alpine
    ports:
      - "5432:5432"
    environment:
      - POSTGRES_PASSWORD=root
    healthcheck:
      test: [ "CMD-SHELL", "pg_isready -U postgres" ]
      interval: 5s
      timeout: 5s
      retries: 5

  web:
    links:
      - postgres
    depends_on:
      postgres:
        condition: service_healthy
    image: "registry.digger.dev/diggerhq/digger_backend:latest"
    env_file:
      - .env
    ports:
      - "3000:3000"' > docker-compose.yml && \
echo "GITHUB_ORG=${GITHUB_ORG:-your_github_org}
HOSTNAME=http://DIGGER_HOSTNAME
BEARER_AUTH_TOKEN=$(openssl rand -base64 12)
DATABASE_URL=postgres://postgres:root@postgres:5432/postgres?sslmode=disable
HTTP_BASIC_AUTH=1
HTTP_BASIC_AUTH_USERNAME=myorg
HTTP_BASIC_AUTH_PASSWORD=$(openssl rand -base64 12)
ALLOW_DIRTY=false" > .env && \
echo -e "\033[1;32m✔ docker-compose\033[0m and \033[1;34m.env\033[0m files stored successfully! \n\033[1;36m🚀 launching services...\033[0m\n\n\033[1;33mℹ️  For next steps visit:\033[0m \033[4;36mhttps://docs.digger.dev/ce/self-host/deploy-docker-compose#setup-docker-compose-file\033[0m" && \
docker-compose up
