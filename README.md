Migrate objects from one MinIO to another or move objects one level up in MinIO

# Top level commands

```
NAME:
  replicate - Migration tool to move/copy objects to MinIO

USAGE:
  replicate COMMAND [COMMAND FLAGS | -h] [ARGUMENTS...]

COMMANDS:
  copy  copy objects from one MinIO to another
  help, h  Shows a list of commands or help for one command
  
FLAGS:
  --help, -h     show help
  --version, -v  print the version
```

## copy
```
NAME:
  replicate copy - copy objects from one MinIO to another

USAGE:
  replicate copy [--skip, --fake]

FLAGS:
   --insecure, -i          disable TLS certificate verification
   --log, -l               enable logging
   --debug                 enable debugging
   --data-dir value        data directory
   --skip value, -s value  number of entries to skip from input file (default: 0)
   --fake                  perform a fake migration
   --help, -h              show help
   

EXAMPLES:
1. Replicate objects in "srcdiff.json" in directory /tmp/data to MinIO.
   $ export MINIO_ENDPOINT=https://minio:9000
   $ export MINIO_ACCESS_KEY=minio
   $ export MINIO_SECRET_KEY=minio123
   $ export MINIO_BUCKET=miniobucket
   $ export MINIO_SOURCE_ENDPOINT=https://minio-src:9000
   $ export MINIO_SOURCE_ACCESS_KEY=minio
   $ export MINIO_SOURCE_SECRET_KEY=minio123
   $ export MINIO_SOURCE_BUCKET=srcbucket
   $ replicate copy --data-dir /tmp/data

2. Replicate objects in "srcdiff.json" from one MinIO to another after skipping 10000 entries in this file
   $ export MINIO_ENDPOINT=https://minio:9000
   $ export MINIO_ACCESS_KEY=minio
   $ export MINIO_SECRET_KEY=minio123
   $ export MINIO_SOURCE_ENDPOINT=https://minio-src:9000
   $ export MINIO_SOURCE_ACCESS_KEY=minio
   $ export MINIO_SOURCE_SECRET_KEY=minio123
   $ export MINIO_BUCKET=miniobucket
   $ export MINIO_SOURCE_BUCKET=srcbucket
   $ replicate copy --data-dir /tmp/data --skip 10000

3. Perform a dry run for replicating objects in "srcdiff.json" from one MinIO to another
   $ export MINIO_ENDPOINT=https://minio:9000
   $ export MINIO_ACCESS_KEY=minio
   $ export MINIO_SECRET_KEY=minio123
   $ export MINIO_SOURCE_ENDPOINT=https://minio-src:9000
   $ export MINIO_SOURCE_ACCESS_KEY=minio
   $ export MINIO_SOURCE_SECRET_KEY=minio123
   $ export MINIO_BUCKET=miniobucket
   $ export MINIO_SOURCE_BUCKET=srcbucket
   $ replicate copy --data-dir /tmp/data --fake --log
```