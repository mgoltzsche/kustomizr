kustomizr
=

A [kpt function](https://googlecontainertools.github.io/kpt/guides/producer/functions/#declaring-functions)
container image to run kustomize builds.

It supports a declarative, idempotent build and deployment workflow
with [kpt](https://github.com/GoogleContainerTools/kpt)
and [kustomize](https://github.com/kubernetes-sigs/kustomize).

## Usage example

The `./examples/hello-world` example within this repository specifies a
[Kustomization](./examples/hello-world/kustomization.yaml) and a
[function ConfigMap](./examples/hello-world/functions.yaml) using the image `mgoltzsche/kustomizr`.  

Render the kustomization into the output file `./examples/hello-world/deploy/generated.yaml` that the function specifies
_(could be run each commit)_:
```
kpt fn run ./examples/hello-world
```
_**Note**: If the kustomization accesses non-yaml files within its repository you may need to `--mount` its directory and specify an absolute path as the function's `path` parameter since non-yaml resources are not provided to the function via stdin._  
_Also a CI pipeline should ensure that the generate is always consistent with the source._  

If needed kpt setters can be used to change kustomization values to e.g. deploy to a preview namespace:
```
kpt cfg set ./examples/hello-world namespace mypreviewns
kpt fn run ./examples/hello-world
```

Enable live deployments for the output directory _(let kpt generate the `inventory-template.yaml` once with component UUID and namespace)_:
```
kpt live init ./examples/hello-world/deploy
```

Apply the rendered output that was written to the `deploy` sub directory previously _(could be run in a CD pipeline)_:
```
kpt live apply ./examples/hello-world/deploy
```

## Development

Build the docker image:
```
make
```

Run a basic e2e test:
```
make test
```