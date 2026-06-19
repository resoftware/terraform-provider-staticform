default: build

.PHONY: build install test testacc docs fmt vet lint snapshot

build:
	go build ./...

install:
	go install .

# Unit tests only (fast, no API calls).
test:
	go test ./... -count=1

# Acceptance tests. Requires STATICFORM_API_KEY (use an sf_test_ key) and,
# optionally, STATICFORM_BASE_URL pointing at a non-production environment.
testacc:
	TF_ACC=1 go test ./... -v -count=1 -timeout 30m

# Generate provider documentation from schema + examples into docs/.
docs:
	go generate ./...

fmt:
	gofmt -w .

vet:
	go vet ./...

# Build a release locally without publishing, to validate the pipeline.
snapshot:
	goreleaser release --snapshot --clean
