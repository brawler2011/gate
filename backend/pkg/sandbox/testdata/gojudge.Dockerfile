FROM criyle/go-judge:v1.11.3 AS gojudge

FROM debian:bookworm-slim

RUN apt-get update && \
    apt-get install -y --no-install-recommends ca-certificates gcc g++ libc6-dev && \
    rm -rf /var/lib/apt/lists/*

WORKDIR /opt
COPY --from=gojudge /opt/go-judge /opt/mount.yaml /opt/

EXPOSE 5051 5052

ENTRYPOINT ["/opt/go-judge"]
