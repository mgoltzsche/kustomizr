IMAGE?=mgoltzsche/kustomizr

image:
	docker build --force-rm -t $(IMAGE) .

test: image
	./test.sh
