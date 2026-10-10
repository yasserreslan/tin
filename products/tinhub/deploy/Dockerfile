# tinhub in a container: every role in one process. Postgres runs beside it (deploy/compose.yaml), packs live on a
# volume at /var/lib/tinhub. Build from the repository root:
#   docker build -f products/tinhub/deploy/Dockerfile -t tinhub:dev .
ARG TIN_BUILDER=ghcr.io/yasserreslan/tin:0.4.0
FROM ${TIN_BUILDER} AS build
WORKDIR /src
COPY . .
RUN mkdir -p /out && tin build products/tinhub/main.tin -o /out/tinhub

FROM debian:bookworm-slim
# postgresql-client is for the backup and restore scripts; the binary itself is static
RUN apt-get update && apt-get install -y --no-install-recommends postgresql-client ca-certificates && rm -rf /var/lib/apt/lists/* \
	&& useradd --system --uid 10001 --home-dir /var/lib/tinhub --shell /usr/sbin/nologin tinhub \
	&& mkdir -p /var/lib/tinhub && chown tinhub /var/lib/tinhub
COPY --from=build /out/tinhub /usr/local/bin/tinhub
COPY products/tinhub/deploy/lib.sh products/tinhub/deploy/backup.sh products/tinhub/deploy/restore.sh /usr/local/lib/tinhub/
USER 10001
VOLUME /var/lib/tinhub
ENV TINHUB_LISTEN=:8080 TINHUB_PACKS_DIR=/var/lib/tinhub
EXPOSE 8080
# migrate, then serve: a migrated schema makes migrate a no-op
ENTRYPOINT ["/bin/sh", "-c", "/usr/local/bin/tinhub migrate && exec /usr/local/bin/tinhub run"]
