GIT_VER := $(shell git describe --tags)
DATE := $(shell date +%Y-%m-%dT%H:%M:%S%z)
export GO111MODULE := on

.PHONY: image release-image

image:
	docker build \
		--tag ghcr.io/kayac/mascaras:$(GIT_VER) \
		.

release-image: image
	docker push ghcr.io/kayac/mascaras:$(GIT_VER)
