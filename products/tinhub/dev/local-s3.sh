#!/bin/sh
# local-s3.sh [DATA]: an S3-compatible store on this machine for developing tinhub against object storage, as it runs in
# production (hosted R2 or S3). It builds MinIO from the same pinned source version CI uses (dl.min.io and quay.io no
# longer hand out the server anonymously), serves DATA (default ~/.cache/tinhub-s3) on 127.0.0.1:9000, makes the bucket
# with tinhub packs copy when tinhub is on the PATH, and prints the TINHUB_* variables to export. Needs Go.
set -eu
data=${1:-${HOME}/.cache/tinhub-s3}
bin=${TINHUB_MINIO_BIN:-${HOME}/.cache/tinhub-minio-bin}
addr=${TINHUB_S3_LOCAL_ADDR:-127.0.0.1:9000}
user=${TINHUB_S3_LOCAL_KEY:-tinhub}
pass=${TINHUB_S3_LOCAL_SECRET:-tinhub-local-secret}
if [ ! -x "$bin/minio" ]; then
	echo "local-s3: building MinIO into $bin" >&2
	GOBIN="$bin" go install github.com/minio/minio@v0.0.0-20260212201848-7aac2a2c5b7c
fi
mkdir -p "$data"
if ! curl -fs -o /dev/null "http://$addr/minio/health/live"; then
	MINIO_ROOT_USER=$user MINIO_ROOT_PASSWORD=$pass nohup "$bin/minio" server "$data" --address "$addr" --quiet > "$data.log" 2>&1 &
	for i in $(seq 1 60); do curl -fs -o /dev/null "http://$addr/minio/health/live" && break; sleep 0.5; done
	curl -fs -o /dev/null "http://$addr/minio/health/live" || { cat "$data.log" >&2; echo "local-s3: MinIO did not start" >&2; exit 1; }
fi
export TINHUB_PACKS_STORE=s3 TINHUB_S3_ENDPOINT="http://$addr" TINHUB_S3_REGION=us-east-1 TINHUB_S3_BUCKET=tinhub
export TINHUB_S3_ACCESS_KEY=$user TINHUB_S3_SECRET_KEY=$pass TINHUB_S3_PATH_STYLE=true
if command -v tinhub > /dev/null 2>&1; then
	# packs copy of an empty directory makes the bucket and copies nothing
	empty=$(mktemp -d)
	TINHUB_PACKS_STORE=dir TINHUB_PACKS_DIR="$empty" tinhub packs copy > /dev/null
	rm -rf "$empty"
fi
cat <<VARS
export TINHUB_PACKS_STORE=s3 TINHUB_S3_ENDPOINT=http://$addr TINHUB_S3_REGION=us-east-1 TINHUB_S3_BUCKET=tinhub
export TINHUB_S3_ACCESS_KEY=$user TINHUB_S3_SECRET_KEY=$pass TINHUB_S3_PATH_STYLE=true
VARS
