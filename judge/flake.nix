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
      # A language may install a trimmed build of its package instead of the package itself.
      trimmed = {
        java = jlinkRuntime;
      };
      langPackages = builtins.mapAttrs (
        name: lang: (trimmed.${name} or (pkg: pkg)) pkgs.${lang.package}
      ) langs;

      # A Java runtime with the standard API (java.se), javac (jdk.compiler), the
      # non-US locale data standard APIs use (jdk.localedata) and sun.misc.Unsafe
      # (jdk.unsupported). It leaves out jmods, src.zip and the
      # tooling modules of the full JDK. The CDS archive keeps JVM startup as fast as
      # the JDK's; modules stay uncompressed for the same reason.
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
            # The copied libraries keep the JDK's lib directory in their RPATH next
            # to $ORIGIN, which already finds them; drop it so the image does not
            # carry the full JDK.
            find $out -type f -exec remove-references-to -t ${jdk} {} +
          '';

      go = pkgs.${langs.go.package};

      # A file of this directory by content only: copying the path itself would keep
      # the checkout's file mode, and a different mode gives a different image.
      source = name: pkgs.writeText name (builtins.readFile (./. + "/${name}"));

      # pdflatex with the packages needed for contest booklets (olymp.sty, Cyrillic).
      # metafont generates the Cyrillic (LH) font metrics on demand.
      tex = pkgs.texliveBasic.withPackages (
        ps: with ps; [
          babel-russian
          cm-super
          cyrillic
          hyphen-russian
          lh
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
        ++ builtins.attrValues langPackages;
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

      # Image identity: judges key the problem programs they compile by its digest. The store paths change
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

      # The base layer (bash, coreutils and glibc), then one layer per language, TeX
      # and go-judge, each holding its closure minus the base. These layers do not
      # depend on each other: a store path two of them need is in both (identical
      # files, so the overlay is unaffected), and changing one language changes only
      # its layer and the last one (runtime env, manifest, /opt). The Go
      # standard-library cache refers to Go and changes with it, so it shares Go's layer.
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
      ++ pkgs.lib.mapAttrsToList (name: pkg: layer ([ pkg ] ++ pkgs.lib.optional (name == "go") goCache)) langPackages
      ++ [
        (layer [ tex ])
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
