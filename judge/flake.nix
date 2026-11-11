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
    in
    {
      packages.${system} = {
        # Everything visible inside the sandbox: its bin/ is mounted as /usr/bin and /bin.
        default = pkgs.buildEnv {
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
        go-cache = pkgs.runCommand "go-std-cache" { nativeBuildInputs = [ go ]; } ''
          export HOME=$TMPDIR GOCACHE=$out CGO_ENABLED=0
          go build std
        '';
      };
    };
}
