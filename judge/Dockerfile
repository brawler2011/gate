FROM criyle/go-judge:v1.11.3 AS gojudge

# Build the language runtime from flake.nix/flake.lock and assemble the image root.
FROM nixos/nix:2.35.6 AS runtime
WORKDIR /src
COPY flake.nix flake.lock langs.toml ./
ENV NIX_CONFIG="experimental-features = nix-command flakes"
RUN nix build --out-link /runtime .#default \
    && nix build --out-link /go-cache .#go-cache \
    && nix build --out-link /manifest .#manifest \
    && mkdir -p /rootfs/nix/store /rootfs/opt \
    && cp -a $(nix-store --query --requisites /runtime /go-cache /manifest) /rootfs/nix/store/ \
    && ln -s "$(readlink /runtime)" /rootfs/opt/runtime \
    && ln -s "$(readlink /go-cache)" /rootfs/opt/go-cache \
    && ln -s "$(readlink /manifest)" /rootfs/opt/manifest \
    && mkdir -m 1777 /rootfs/tmp
COPY --from=gojudge /opt/go-judge /rootfs/opt/go-judge
COPY mount.yaml /rootfs/opt/mount.yaml

FROM scratch
COPY --from=runtime /rootfs /
# Lets `docker exec` reach the runtime tools for debugging; sandboxed commands get their own PATH.
ENV PATH=/opt/runtime/bin
WORKDIR /opt
EXPOSE 5050 5051
ENTRYPOINT ["/opt/go-judge", "-mount-conf=/opt/mount.yaml"]
