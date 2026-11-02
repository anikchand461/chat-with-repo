# Lambda package for template.yaml (BuildMethod: makefile): only the backend
# code and its dependencies (requirements-aws.txt), so frontend/, mobile/
# and .env files stay out.
# Dependencies are Linux arm64 wheels, matching the function architecture.
build-FastAPIApp:
	rsync -a --exclude .env --exclude .venv --exclude __pycache__ --exclude tests backend/ $(ARTIFACTS_DIR)/backend/
	uv pip install --quiet --no-cache -r requirements-aws.txt --target $(ARTIFACTS_DIR) --python-platform aarch64-manylinux_2_28 --python-version 3.12 --only-binary :all:
