{
  description = "Language runtime for the go-judge sandbox";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-26.05";

  outputs =
    { nixpkgs, ... }:
    let
      system = "x86_64-linux";
      pkgs = nixpkgs.legacyPackages.${system};

      # langs.toml is shared with the backend: each language names its nixpkgs package.
      langs = builtins.fromTOML (builtins.readFile ./langs.toml);
      langPackages = map (lang: pkgs.${lang.package}) (builtins.attrValues langs);

      go = pkgs.${langs.go.package};

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
    in
    {
      packages.${system} = {
        default = runtime;
        go-cache = goCache;

        # What the backend reads from the sandbox at startup (mounted at /etc/judge).
        manifest = pkgs.runCommand "judge-manifest" {
          identity = builtins.toJSON identity;
          passAsFile = [ "identity" ];
        } ''
          mkdir $out
          cp "$identityPath" $out/identity.json
          cp ${./langs.toml} $out/langs.toml
        '';
      };
    };
}
