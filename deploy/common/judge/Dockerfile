FROM criyle/go-judge:v1.11.3 AS gojudge

FROM debian:bookworm-slim

# Install compilers and runtimes
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
    texlive-latex-base \
    texlive-latex-recommended \
    texlive-lang-cyrillic \
    texlive-fonts-recommended \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /opt
COPY --from=gojudge /opt/go-judge /opt/mount.yaml /opt/

# Create Go compiler symlink for languages.yaml path compatibility
RUN mkdir -p /usr/local/go/bin && ln -sf /usr/bin/go /usr/local/go/bin/go

# Modify default go-judge mounts to support java-17 instead of java-21, and add texlive mounts
RUN sed -i 's/java-21-openjdk/java-17-openjdk/g' /opt/mount.yaml && \
    python3 -c 'content = open("/opt/mount.yaml").read(); idx = content.find("proc: true"); tex_mounts = "  - type: bind\n    source: /var/lib/texmf\n    target: /var/lib/texmf\n    readonly: true\n  - type: bind\n    source: /etc/texmf\n    target: /etc/texmf\n    readonly: true\n"; open("/opt/mount.yaml", "w").write(content[:idx] + tex_mounts + content[idx:])'

EXPOSE 5051 5052

ENTRYPOINT ["/opt/go-judge"]
