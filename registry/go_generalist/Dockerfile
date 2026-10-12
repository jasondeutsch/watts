# Build context: registry/go_generalist. Build watts/pi-ralph:0.1.0 first.
FROM golang:1.27.0-bookworm AS tools
ARG GOPLS_VERSION=v0.23.0
ARG GOLANGCI_LINT_VERSION=v2.14.0
RUN GOBIN=/tools go install "golang.org/x/tools/gopls@${GOPLS_VERSION}"
RUN GOBIN=/tools go install "github.com/golangci/golangci-lint/v2/cmd/golangci-lint@${GOLANGCI_LINT_VERSION}"

# A matching compiler version alone does not establish analyzer compatibility.
# Exercise standard-library export data before accepting this toolchain image.
RUN mkdir /tmp/toolchain-check \
    && cd /tmp/toolchain-check \
    && printf 'module toolchain-check\n\ngo 1.27.0\n' > go.mod \
    && printf 'package main\nimport "fmt"\nfunc main() { fmt.Println("toolchain check") }\n' > main.go \
    && go build ./... \
    && go test ./... \
    && /tools/golangci-lint run --no-config ./... \
    && rm -rf /tmp/toolchain-check

FROM watts/pi-ralph:0.1.0
USER root
COPY --from=tools /usr/local/go /usr/local/go
COPY --from=tools /tools/ /usr/local/bin/
ENV PATH=/usr/local/go/bin:/usr/local/bin:/usr/bin:/bin \
    GOTOOLCHAIN=local \
    GOCACHE=/runtime/go-cache \
    GOMODCACHE=/runtime/go-mod
USER node
WORKDIR /workspace
