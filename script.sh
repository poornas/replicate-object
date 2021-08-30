#!/bin/sh
# copy src.json to /tmp/data directory
jq 'select(.key | contains("/") | not)' /tmp/data/src.json | jq -c 'del(.url)' > /tmp/data/srcdiff.json

# source cluster env
export MINIO_SOURCE_ENDPOINT=https://minio-src:9000
export MINIO_SOURCE_ACCESS_KEY=minio
export MINIO_SOURCE_SECRET_KEY=minio123
export MINIO_SOURCE_BUCKET=sourcebucket

# dest cluster env
export MINIO_ENDPOINT=https://minio-dst:9000
export MINIO_ACCESS_KEY=minio
export MINIO_SECRET_KEY=minio123
export MINIO_BUCKET=destbucket
# change data dir to where srcdiff.json is present
./replicate copy --data-dir /tmp/data
