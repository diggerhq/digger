
echo $CR_PAT | docker login ghcr.io -u USERNAME --password-stdin
docker build -t ghcr.io/diggerhq/tfrun:latest . -f Dockerfile.base
docker push ghcr.io/diggerhq/tfrun:latest
