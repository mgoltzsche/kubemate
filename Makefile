all: image

k3spi:
	go build .

image:
	docker build --force-rm -t k3spi .
