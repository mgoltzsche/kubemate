#!/bin/sh

docker rm -f `docker ps -qa --filter label=io.kubernetes.container.name`
docker volume rm `docker volume ls -q --filter label=com.docker.volume.anonymous`
