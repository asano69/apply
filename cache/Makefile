.PHONY: init install format lint test build publish clean

PYTHON ?= uv run

DIST_DIR := dist

init:
	@command -v uv >/dev/null 2>&1 || { echo >&2 "Error: uv is not installed."; exit 1; }

install: init
	@uv sync

format: init
	@$(PYTHON) ruff check search_replace tests --fix
	@$(PYTHON) black search_replace tests

lint: init
	@$(PYTHON) ruff check search_replace tests
	@$(PYTHON) black --check search_replace tests
	@$(PYTHON) mypy search_replace

test: init
	@$(PYTHON) pytest

# Preferred build (uv)
build: init
	@rm -rf $(DIST_DIR)
	@uv build
	@ls -la $(DIST_DIR)

# Publish to PyPI (requires UV_PUBLISH_TOKEN)
publish: build
	@test -n "$$UV_PUBLISH_TOKEN" || { echo >&2 "Error: UV_PUBLISH_TOKEN is not set"; exit 1; }
	@uv publish

clean:
	@rm -rf $(DIST_DIR) .pytest_cache .ruff_cache .mypy_cache
