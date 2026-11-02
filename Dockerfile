# ChatWithRepo API image for AWS (arm64), used by two things (template.yaml):
#
# - ApiContainerFunction (Lambda): the unchanged FastAPI app runs under
#   uvicorn, and the Lambda Web Adapter extension turns invocations into
#   HTTP requests to it - with streamed responses for /chat/{id}/ask/stream.
# - IndexTaskDefinition (Fargate): repository indexing jobs, started with
#   the command `python -m backend.jobs.index_chat '<job>'`. Fargate ignores
#   the Lambda extension.
#
# Built and pushed by deploy-aws.sh. Render does not use this file.

FROM public.ecr.aws/docker/library/python:3.12-slim

COPY --from=public.ecr.aws/awsguru/aws-lambda-adapter:0.9.1 /lambda-adapter /opt/extensions/lambda-adapter

WORKDIR /var/task

COPY requirements-aws.txt .
RUN pip install --no-cache-dir -r requirements-aws.txt boto3

COPY backend ./backend
RUN python -m compileall -q backend

# Lambda's filesystem is read-only except /tmp.
ENV PYTHONUNBUFFERED=1 \
    PYTHONDONTWRITEBYTECODE=1 \
    HOME=/tmp \
    XDG_CACHE_HOME=/tmp/.cache

CMD ["uvicorn", "backend.app:app", "--host", "127.0.0.1", "--port", "8000"]
