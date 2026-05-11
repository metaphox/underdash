.PHONY: lint test install-hooks

# Run golangci-lint on all Go source.
lint:
	cd src && golangci-lint run ./...

# Run all tests.
test:
	cd src && go test ./...

# Install git hooks (symlinks scripts/pre-commit into .git/hooks/).
install-hooks:
	@mkdir -p .git/hooks
	@ln -sf ../../scripts/pre-commit .git/hooks/pre-commit
	@echo "Installed pre-commit hook."
