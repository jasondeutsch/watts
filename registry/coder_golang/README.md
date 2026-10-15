# Golang coder workload image

An example team image extending the Pi/Ralph workload with Go 1.27.0,
gopls v0.23.0, and golangci-lint v2.14.0. Checks and skills remain package
assets configured in the example's ACL and variable file.

```sh
make images
```

See [local build requirements and diagnostics](../README.md#build-locally).
Bake supplies the unpublished Agent base directly from its build target.

Build the image on each local Docker worker before submitting a task. Watts pins
its resolved image ID at submission; changing a tag does not change an existing
execution. Adapt these pinned tool versions and rebuild for your team's needs.

Image builds compile and lint a small Go module to verify compiler/analyzer compatibility, rather than checking version commands alone.
