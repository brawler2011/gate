{
  description = "go-judge sandbox image with the language runtime";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-26.05";
    nix2container = {
      url = "github:nlewo/nix2container";
      inputs.nixpkgs.follows = "nixpkgs";
    };
  };

  outputs =
    { nixpkgs, nix2container, ... }:
    let
      system = "x86_64-linux";
      pkgs = nixpkgs.legacyPackages.${system};
      n2c = nix2container.packages.${system}.nix2container;

      # langs.toml is shared with the backend: each language names its nixpkgs package.
      langs = builtins.fromTOML (builtins.readFile ./langs.toml);
      langPackages = map (lang: pkgs.${lang.package}) (builtins.attrValues langs);

      go = pkgs.${langs.go.package};

      # A file of this directory by content only: copying the path itself would keep
      # the checkout's file mode, and a different mode gives a different image.
      source = name: pkgs.writeText name (builtins.readFile (./. + "/${name}"));

      # pdflatex with the packages needed for contest booklets (olymp.sty, Cyrillic).
      # metafont generates the Cyrillic (LH) font metrics on demand.
      tex = pkgs.texliveBasic.withPackages (
        ps: with ps; [
          collection-latexrecommended
          collection-fontsrecommended
          collection-langcyrillic
          metafont
        ]
      );

      # Everything visible inside the sandbox: its bin/ is mounted as /usr/bin and /bin.
      runtime = pkgs.buildEnv {
        name = "judge-runtime";
        paths = [
          pkgs.bash
          pkgs.coreutils
          tex
        ]
        ++ langPackages;
      };

      # Go 1.20+ ships no precompiled standard library, so a cold build exceeds
      # the compile limit. The sandbox mounts this cache read-only as GOCACHE.
      goCache = pkgs.runCommand "go-std-cache" { nativeBuildInputs = [ go ]; } ''
        export HOME=$TMPDIR GOCACHE=$out CGO_ENABLED=0
        go build std
        # Go stamps each action entry and trim.txt with the current time. Fix both,
        # so the same toolchain gives the same cache and image layer. Entries keep
        # their fixed width (the time is the last 20 characters).
        find $out -name '*-a' -exec sed -i -E 's/.{20}$/                   0/' {} +
        echo 0 > $out/trim.txt
      '';

      # Toolchain identity pinned into every problem build. The store paths change
      # whenever any compiler, library or flake input changes, and only then.
      identity = {
        digest =
          "sha256:"
          + builtins.hashString "sha256" (
            builtins.unsafeDiscardStringContext "${runtime}\n${goCache}"
          );
        architecture =
          {
            x86_64-linux = "amd64";
            aarch64-linux = "arm64";
          }
          .${system};
        abi = "linux-gnu";
      };

      # What the backend reads from the sandbox at startup (mounted at /etc/judge).
      manifest = pkgs.runCommand "judge-manifest" {
        identity = builtins.toJSON identity;
        passAsFile = [ "identity" ];
      } ''
        mkdir $out
        cp "$identityPath" $out/identity.json
        cp ${source "langs.toml"} $out/langs.toml
      '';

      # The image root: /opt as mount.yaml expects it, and a writable /tmp.
      root = pkgs.runCommand "judge-root" { } ''
        mkdir -p $out/opt $out/tmp
        ln -s ${runtime} $out/opt/runtime
        ln -s ${goCache} $out/opt/go-cache
        ln -s ${manifest} $out/opt/manifest
        ln -s ${pkgs.go-judge}/bin/go-judge $out/opt/go-judge
        ln -s ${source "mount.yaml"} $out/opt/mount.yaml
      '';

      # One layer for the shared base, one per language, then TeX, the Go cache
      # and go-judge. Each layer skips the store paths of the layers before it.
      # Shared paths belong to the first layer that needs them, in language-name
      # order (cpp, go, java, python). Updating an early language can move shared
      # paths into or out of later language, TeX, Go cache and go-judge layers,
      # changing those layers too. Updating Go also rebuilds its standard-library
      # cache. The last layer (runtime env, manifest, /opt) changes with any language.
      # This layout reduces transfers but does not guarantee language isolation.
      layers = builtins.foldl' (built: deps: built ++ [ (n2c.buildLayer { inherit deps; layers = built; }) ]) [ ] (
        [
          [
            pkgs.bash
            pkgs.coreutils
          ]
        ]
        ++ map (pkg: [ pkg ]) langPackages
        ++ [
          [ tex ]
          [ goCache ]
          [ pkgs.go-judge ]
        ]
      );

      image = n2c.buildImage {
        name = "gate-go-judge";
        copyToRoot = root;
        inherit layers;
        perms = [
          {
            path = root;
            regex = "/tmp$";
            mode = "1777";
          }
        ];
        config = {
          entrypoint = [
            "/opt/go-judge"
            "-mount-conf=/opt/mount.yaml"
          ];
          # Lets `docker exec` reach the runtime tools for debugging; sandboxed commands get their own PATH.
          env = [ "PATH=/opt/runtime/bin" ];
          workingDir = "/opt";
          exposedPorts = {
            "5050/tcp" = { };
            "5051/tcp" = { };
          };
        };
      };
    in
    {
      packages.${system}.default = image;
    };
}
