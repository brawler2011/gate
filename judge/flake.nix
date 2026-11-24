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

      # Configurations may share a compiler package.
      langs = builtins.fromTOML (builtins.readFile ./langs.toml);
      trimmed = {
        ${langs.java.package} = jlinkRuntime;
      };
      langPackages = builtins.listToAttrs (
        map (name: {
          inherit name;
          value = (trimmed.${name} or (pkg: pkg)) pkgs.${name};
        }) (pkgs.lib.unique (map (lang: lang.package) (builtins.attrValues langs)))
      );

      # Keep CDS and uncompressed modules for JVM startup speed.
      jlinkRuntime =
        jdk:
        pkgs.runCommand "${jdk.pname}-jlink-${jdk.version}"
          {
            nativeBuildInputs = [
              jdk
              pkgs.binutils
              pkgs.removeReferencesTo
            ];
            disallowedReferences = [ jdk ];
          }
          ''
            jlink --module-path ${jdk}/lib/openjdk/jmods \
              --add-modules java.se,jdk.compiler,jdk.localedata,jdk.unsupported \
              --strip-debug --no-man-pages --no-header-files --generate-cds-archive \
              --output $out
            # Drop the full JDK reference from RPATH; $ORIGIN already finds the copied libraries.
            find $out -type f -exec remove-references-to -t ${jdk} {} +
          '';

      go = pkgs.${langs.go.package};

      # Copy content only: checkout file modes must not affect image identity.
      source = name: pkgs.writeText name (builtins.readFile (./. + "/${name}"));


      # Expose the Nix runtime as /usr/bin and /bin in mount.yaml.
      runtime = pkgs.buildEnv {
        name = "judge-runtime";
        paths = [
          pkgs.bash
          pkgs.coreutils
        ]
        ++ builtins.attrValues langPackages;
      };

      # Prebuild the standard library so cold Go builds fit the compile limit.
      goCache = pkgs.runCommand "go-std-cache" { nativeBuildInputs = [ go ]; } ''
        export HOME=$TMPDIR GOCACHE=$out CGO_ENABLED=0
        go build std
        # Normalize timestamps for reproducible layers while preserving fixed-width cache entries.
        find $out -name '*-a' -exec sed -i -E 's/.{20}$/                   0/' {} +
        echo 0 > $out/trim.txt
      '';

      # Compiled program cache keys must change with the toolchain.
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

      manifest = pkgs.runCommand "judge-manifest" {
        identity = builtins.toJSON identity;
        passAsFile = [ "identity" ];
      } ''
        mkdir $out
        cp "$identityPath" $out/identity.json
        cp ${source "langs.toml"} $out/langs.toml
      '';

      root = pkgs.runCommand "judge-root" { } ''
        mkdir -p $out/opt $out/tmp
        ln -s ${runtime} $out/opt/runtime
        ln -s ${goCache} $out/opt/go-cache
        ln -s ${manifest} $out/opt/manifest
        ln -s ${pkgs.go-judge}/bin/go-judge $out/opt/go-judge
        ln -s ${source "mount.yaml"} $out/opt/mount.yaml
      '';

      # Independent language layers avoid rebuilding other toolchains; Go's cache shares its compiler layer.
      base = n2c.buildLayer {
        deps = [
          pkgs.bash
          pkgs.coreutils
        ];
      };
      layer =
        deps:
        n2c.buildLayer {
          inherit deps;
          layers = [ base ];
        };
      layers = [
        base
      ]
      ++ pkgs.lib.mapAttrsToList (name: pkg: layer ([ pkg ] ++ pkgs.lib.optional (name == langs.go.package) goCache)) langPackages
      ++ [
        (layer [ pkgs.go-judge ])
      ];

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
          # Allow docker exec to reach runtime tools; sandboxed commands receive their own PATH.
          env = [ "PATH=/opt/runtime/bin" ];
          workingDir = "/opt";
          exposedPorts = {
            "5050/tcp" = { };
          };
        };
      };
    in
    {
      packages.${system}.default = image;
    };
}
