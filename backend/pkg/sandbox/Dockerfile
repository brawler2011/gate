FROM criyle/go-judge:v1.11.3 AS gojudge

FROM debian:bookworm-slim

# Install the exact same compilers as HydroJudge
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    gcc \
    g++ \
    python3 \
    fp-compiler \
    openjdk-17-jdk-headless \
    php-cli \
    rustc \
    ghc \
    libjavascriptcoregtk-4.0-bin \
    golang \
    ruby \
    mono-runtime \
    mono-mcs \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /opt
COPY --from=gojudge /opt/go-judge /opt/mount.yaml /opt/

# Create Go compiler symlink for languages.yaml path compatibility
RUN mkdir -p /usr/local/go/bin && ln -sf /usr/bin/go /usr/local/go/bin/go

# Modify default go-judge mounts to support java-17 instead of java-21
RUN sed -i 's/java-21-openjdk/java-17-openjdk/g' /opt/mount.yaml

EXPOSE 5051 5052

ENTRYPOINT ["/opt/go-judge"]
