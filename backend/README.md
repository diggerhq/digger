The repository for digger official API.

# Self-hosting as a Docker container (eg on ECS, Fly.io or similar)

> [!NOTE]
> Digger API currently only supports Frontegg as an authentication provider. Please refer to [Frontegg docs](https://docs.frontegg.com/docs/get-started) for configuration

### 1. Pull the official Digger API Docker image

```
docker pull diggerdevhq/backend:main
```
### 2. Run with Docker
The Digger Docker image requires several required environment variables. You'll need to add the required environment variables listed below to your docker run command
Once you have added the required environment variables to your docker run command, execute it in your terminal:
```
docker run -p 3000:3000  \
-e DATABASE_URL=<your_postgres_url> \
diggerdevhq/backend:latest
```

# Running for development

1. Create the environment files for local development:
   2. `echo "DATABASE_URL=postgres://postgres:23q4RSDFSDFS@127.0.0.1:5432/postgres" > .env`
   3. `echo "DATABASE_URL=postgres://postgres:23q4RSDFSDFS@postgres:5432/postgres" > .env.docker-compose`
2. Start the Docker containers for the database and the API via `docker-compose up` or `docker-compose up -d` which should make it available from http://localhost:3100   
3. You can also run the API by typing `make start` which should make it available from http://localhost:3000



