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

      catalog = builtins.fromTOML (builtins.readFile ./langs.toml);
      jre = jlinkRuntime pkgs.${catalog.java.package};
      # Packages that the image takes in a different form than nixpkgs builds them.
      trimmed = {
        ${catalog.java.package} = _: jre;
        ${catalog.kotlin.package} = kotlin: kotlin.override { inherit jre; };
        ${catalog.scala.package} =
          scala:
          let
            compiler = scala.override { inherit jre; };
          in
          pkgs.symlinkJoin {
            name = "scala-jar";
            paths = [
              compiler
              (pkgs.writeShellScriptBin "scala-jar" ''
                export SCALA_LIB=${compiler.bare}/lib
                ${builtins.readFile ./scala-jar.sh}
              '')
            ];
          };
        ${catalog.csharp.package} = dotnetTools;
      };

      # csc compiles against the reference pack, and the program runs with a fixed runtime config, so a build yields one file.
      dotnetTools =
        sdk:
        let
          root = "${sdk.unwrapped or sdk}/share/dotnet";
          runtimeConfig = pkgs.writeText "solution.runtimeconfig.json" (
            builtins.toJSON {
              runtimeOptions = {
                tfm = "net10.0";
                framework = {
                  name = "Microsoft.NETCore.App";
                  version = "10.0.0";
                };
                configProperties."System.Globalization.Invariant" = true;
              };
            }
          );
        in
        pkgs.symlinkJoin {
          name = "dotnet-tools";
          paths = [
            (pkgs.writeShellScriptBin "csc-dll" ''
              set -eu
              out=$1
              shift
              export DOTNET_SYSTEM_GLOBALIZATION_INVARIANT=1 DOTNET_CLI_TELEMETRY_OPTOUT=1
              csc=(${root}/sdk/*/Roslyn/bincore/csc.dll)
              refs=(${root}/packs/Microsoft.NETCore.App.Ref/*/ref/net10.0/*.dll)
              exec ${root}/dotnet "''${csc[0]}" -nologo -optimize+ -langversion:latest -out:"$out" "''${refs[@]/#/-r:}" "$@"
            '')
            (pkgs.writeShellScriptBin "dotnet-dll" ''
              exec ${root}/dotnet exec --runtimeconfig ${runtimeConfig} "$@"
            '')
          ];
        };

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
              --add-modules java.se,jdk.compiler,jdk.jartool,jdk.localedata,jdk.unsupported \
              --strip-debug --no-man-pages --no-header-files --generate-cds-archive \
              --output $out
            # Drop the full JDK reference from RPATH; $ORIGIN already finds the copied libraries.
            find $out -type f -exec remove-references-to -t ${jdk} {} +
          '';

      go = pkgs.${catalog.go.package};

      javacJar = pkgs.writeShellScriptBin "javac-jar" (builtins.readFile ./javac-jar.sh);

      # Copy content only: checkout file modes must not affect image identity.
      source = name: pkgs.writeText name (builtins.readFile (./. + "/${name}"));


      # Each image installs the configurations of its variant; the manifest keeps the whole catalog.
      mkImage =
        variant:
        let
          langs = pkgs.lib.filterAttrs (_: lang: (lang.variant or variant) == variant) catalog;
          # Configurations may share a compiler package.
          langPackages = builtins.listToAttrs (
            map (name: {
              inherit name;
              value = (trimmed.${name} or (pkg: pkg)) pkgs.${name};
            }) (pkgs.lib.unique (map (lang: lang.package) (builtins.attrValues langs)))
          );

          # Expose the Nix runtime as /usr/bin and /bin in mount.yaml.
          runtime = pkgs.buildEnv {
            name = "judge-runtime";
            paths = [
              pkgs.bash
              pkgs.coreutils
              javacJar
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
            echo ${variant} > $out/variant
          '';

          root = pkgs.runCommand "judge-root" { } ''
            mkdir -p $out/opt $out/tmp
            ln -s ${runtime} $out/opt/runtime
            ln -s ${goCache} $out/opt/go-cache
            ln -s ${manifest} $out/opt/manifest
            ln -s ${pkgs.go-judge}/bin/go-judge $out/opt/go-judge
            ln -s ${source "mount.yaml"} $out/opt/mount.yaml
          '';

          image = n2c.buildImage {
            name = if variant == "full" then "gate-go-judge-full" else "gate-go-judge";
            copyToRoot = root;
            # Each store path ships once; large toolchains still get layers of their own.
            maxLayers = 100;
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
        image;
    in
    {
      packages.${system} = rec {
        minimal = mkImage "minimal";
        full = mkImage "full";
        default = minimal;
      };
    };
}
