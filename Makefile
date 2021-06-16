GIT_VER := $(shell git describe --tags)
DATE := $(shell date +%Y-%m-%dT%H:%M:%S%z)
export GO111MODULE := on

.PHONY: image release-image

image:
	docker build \
		--tag ghcr.io/kayac/mascaras:$(GIT_VER) \
		--tag ghcr.io/kayac/mascaras:latest \
		.

release-image: image
	docker push ghcr.io/kayac/mascaras:$(GIT_VER)
	docker push ghcr.io/kayac/mascaras:latest
