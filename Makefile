VERSION ?= latest
IMAGE ?= docker.io/mgoltzsche/kustomizr:$(VERSION)

BUILD_DIR := $(shell pwd)/build
BIN_DIR := $(BUILD_DIR)/bin
BATS_DIR = $(BUILD_DIR)/tools/bats
BATS = $(BIN_DIR)/bats
BATS_VERSION = v1.3.0
KPT = $(BIN_DIR)/kpt
KPT_VERSION = v0.39.2

image:
	docker build --force-rm -t $(IMAGE) .

test: image $(BATS) $(KPT)
	export PATH="`pwd`/build/bin:$$PATH"; \
	$(BATS) -T .

clean:
	rm -rf $(BUILD_DIR)
	find examples -name generated-manifest.yaml | xargs -r rm

docker-push:
	docker push ${IMAGE}

check-repo-unchanged:
	@[ -z "`git status --untracked-files=no --porcelain`" ] || (\
		echo 'ERROR: the build changed files tracked by git:'; \
		git status --untracked-files=no --porcelain | sed -E 's/^/  /'; \
		echo 'Please call `make static-manifests` and commit the resulting changes.'; \
		false) >&2

release:
	make image test check-repo-unchanged docker-push VERSION=latest
	make image docker-push VERSION=$(VERSION)

$(KPT): kpt
kpt:
	$(call download-bin,$(KPT),"https://github.com/GoogleContainerTools/kpt/releases/download/$(KPT_VERSION)/kpt_$$(uname | tr '[:upper:]' '[:lower:]')_amd64")

$(BATS):
	@echo Downloading bats
	@{ \
	set -e ;\
	mkdir -p $(BIN_DIR) ;\
	TMP_DIR=$$(mktemp -d) ;\
	cd $$TMP_DIR ;\
	git clone -c 'advice.detachedHead=false' --branch $(BATS_VERSION) https://github.com/bats-core/bats-core.git . >/dev/null;\
	./install.sh $(BATS_DIR) ;\
	ln -s $(BATS_DIR)/bin/bats $(BATS) ;\
	}

# download-bin downloads a binary into the location given as first argument
define download-bin
@[ -f $(1) ] || { \
set -e ;\
mkdir -p `dirname $(1)` ;\
TMP_FILE=$$(mktemp) ;\
echo "Downloading $(2)" ;\
curl -fsSLo $$TMP_FILE $(2) ;\
chmod +x $$TMP_FILE ;\
mv $$TMP_FILE $(1) ;\
}
endef
