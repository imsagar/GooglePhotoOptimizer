.PHONY: runner dev all

# Build runner for current platform
runner:
	go build -o gpoptimizer-runner ./cmd/runner
	@echo "Built: ./gpoptimizer-runner"
	@echo "Pair:  ./gpoptimizer-runner --pair <CODE> --server http://localhost:8080"
	@echo "Run:   GOOGLE_CLIENT_ID=... GOOGLE_CLIENT_SECRET=... ./gpoptimizer-runner"

# Build runner for all platforms
runner-all:
	GOOS=darwin GOARCH=arm64 go build -o dist/gpoptimizer-runner-darwin-arm64 ./cmd/runner
	GOOS=darwin GOARCH=amd64 go build -o dist/gpoptimizer-runner-darwin-amd64 ./cmd/runner
	GOOS=linux GOARCH=amd64 go build -o dist/gpoptimizer-runner-linux-amd64 ./cmd/runner
	GOOS=windows GOARCH=amd64 go build -o dist/gpoptimizer-runner-windows-amd64.exe ./cmd/runner
	@echo "Built all runners in dist/"

# Start everything for dev
dev:
	@echo "Building runner..."
	go build -o gpoptimizer-runner ./cmd/runner
	@echo "Starting Docker services..."
	docker compose up --build -d
	@echo ""
	@echo "=== GPOptimizer Dev ==="
	@echo "Web UI:  http://localhost:5173"
	@echo "Runner:  ./gpoptimizer-runner --pair <CODE> --server http://localhost:8080"
	@echo ""
