# Security-review verification tests

These tests document and re-check findings from the security review
in `REVIEW.md`. They are ordinary Go tests against the main module.

Run from the repo root:

```
go test ./client/ -count=1 -run 'SecurityReview'
go test ./server/ -count=1 -run 'SecurityReview'
go test ./wg-server/ -count=1 -run 'SecurityReview'
```

Podman e2e for kill switch + TUN reuse (`//go:build e2e`):

```
go test -tags e2e ./test/clientks/ -count=1 -timeout 15m -v
```
