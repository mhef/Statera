# Execute the statera target, that generates a production-ready image
all: statera
# Rebuilds the app, generates the tagged image and run the compose-up target
# This command is safe to be re-runned multiple times: it will replace the app image and restart the container
build-run: statera compose-up

# This target build the app and generate a production-ready, lightweight image with tag statera:latest.
.PHONY: statera
statera:
	DOCKER_BUILDKIT=1 docker build --progress=plain -t statera .

# This target conjure the docker dependencies and run the app on the background.
.PHONY: compose-up
compose-up:
	docker-compose up -d --no-deps --force-recreate
	