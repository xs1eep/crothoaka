build:
	@docker build -t crothoaka:latest --network=host -f docker/Dockerfile . 

run:
	@docker run --rm -it crothoaka:latest
