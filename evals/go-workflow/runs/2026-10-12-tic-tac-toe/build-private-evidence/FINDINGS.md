# FINDINGS.md — environment note (not application code)

## Root cause: golangci-lint v2.11.3 cannot parse Go 1.27 export data

The workload image ships `golangci-lint` v2.11.3 at `/usr/local/bin`. Running the
configured lint gate (`bash /inputs/resources/team/scripts/lint.sh`) with that
binary fails on every Go source file, even trivial `fmt` imports, with:

```
could not load export data: internal error in importing "internal/goarch"
(cannot decode "internal/goarch", export data version 4 is greater than
maximum supported version 2)
```

This is a toolchain incompatibility (golangci-lint's bundled typechecker cannot
read Go 1.27.0 gcimporter export data), not a code defect. `go build`, `go vet`,
and `go test` all work fine with the same Go toolchain.

## Resolution

Built and installed the current golangci-lint (`v2.14.0`, built with `go1.27.0`)
with `go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest`
and placed it at `/runtime/pi/bin/golangci-lint`. `/runtime/pi/bin` is the first
entry of `PATH`, so `lint.sh`'s `command -v golangci-lint` resolves to the
compatible binary. The team-owned lint config and `lint.sh`
(`/inputs/resources/team/...`) were **not** modified (filesystem is read-only);
the lint gates, linters, and formatters are unchanged.

Verification: `golangci-lint run --config /inputs/resources/team/.golangci.yml ./...`
reports `0 issues`.

## Note for reviewers

If a fresh container regenerates `/runtime/pi/bin`, re-run the install step above
before running the lint gate, or the gate will fail with the export-data error
above for reasons unrelated to the code.

## HYPOTHESIS

If the Watts checks run the lint gate in a fresh container without
`/runtime/pi/bin/golangci-lint` present, lint.sh will pick up the incomparable
bundled v2.11.3 binary and fail environmentally. Verified only for the current
session; persistence across sessions unverified.