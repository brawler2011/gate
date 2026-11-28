{
  description = "go-judge sandbox image that typesets problem statements";

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

      tex = pkgs.texliveBasic.withPackages (
        ps: with ps; [
          amsfonts
          amsmath
          babel
          cm-unicode
          dvisvgm
          fontspec
          graphics
          hyphen-afrikaans
          hyphen-estonian
          hyphen-finnish
          hyphen-german
          hyphen-icelandic
          hyphen-latvian
          hyphen-lithuanian
          hyphen-norwegian
          hyphen-polish
          hyphen-russian
          hyphen-swedish
          hyphen-ukrainian
          luaotfload
          luatex
          metafont
          metapost
          wrapfig
        ]
      );

      # rsvg-convert renders SVG text with these fonts only.
      fonts = pkgs.makeFontsConf { fontDirectories = [ pkgs.dejavu_fonts ]; };

      # Regional subsets: LuaTeX holds a whole font per size in memory, and Pan-CJK has 65k glyphs.
      noto = "https://github.com/notofonts/noto-cjk/raw/f8d157532fbfaeda587e826d4cd5b21a49186f7c/Serif/SubsetOTF";
      cjk = pkgs.runCommand "noto-serif-cjk-subsets" { } ''
          mkdir -p $out
          cp ${pkgs.fetchurl { url = "${noto}/JP/NotoSerifJP-Regular.otf"; hash = "sha256-LJoS29TyQIxGEMfuhKEIti1yNsN3W67WGMZNnLRLLwQ="; }} $out/NotoSerifJP-Regular.otf
          cp ${pkgs.fetchurl { url = "${noto}/JP/NotoSerifJP-Bold.otf"; hash = "sha256-HgNIig1egZ8H/NdPVHA6eWG6Rm066QD4oqcwVB5tRUM="; }} $out/NotoSerifJP-Bold.otf
          cp ${pkgs.fetchurl { url = "${noto}/SC/NotoSerifSC-Regular.otf"; hash = "sha256-6POW3swfCWOgFqmJw9iFLoY9E1CZb1c4YKgHZ8g6HNM="; }} $out/NotoSerifSC-Regular.otf
          cp ${pkgs.fetchurl { url = "${noto}/SC/NotoSerifSC-Bold.otf"; hash = "sha256-JGk9SL25FS8KBrAq9iVjihCXq9beQBDrugJ/boJxBSc="; }} $out/NotoSerifSC-Bold.otf
      '';

      # luaotfload's font database and parsed fonts; without them every sandboxed run rebuilds them for seconds.
      fontCache = pkgs.runCommand "typeset-font-cache" { nativeBuildInputs = [ tex ]; } ''
        export HOME=$TMPDIR OSFONTDIR=${cjk} TEXMFVAR=$out TEXMFCACHE=$out
        mkdir -p $out
        cat > warm.tex <<'EOF'
        \documentclass{article}
        \usepackage{fontspec}
        \begin{document}
        \fontspec{cmunrm.otf}[BoldFont=cmunbx.otf, ItalicFont=cmunti.otf, BoldItalicFont=cmunbi.otf]a\textbf{a}\textit{a}\textbf{\textit{a}}
        \fontspec{cmunss.otf}[BoldFont=cmunsx.otf, ItalicFont=cmunsi.otf, BoldItalicFont=cmunso.otf]a\textbf{a}\textit{a}\textbf{\textit{a}}
        \fontspec{cmuntt.otf}[BoldFont=cmuntb.otf, ItalicFont=cmunit.otf, BoldItalicFont=cmuntx.otf]a\textbf{a}\textit{a}\textbf{\textit{a}}
        \fontspec{NotoSerifJP-Regular.otf}[Renderer=HarfBuzz, BoldFont=NotoSerifJP-Bold.otf]字\textbf{字}
        \fontspec{NotoSerifSC-Regular.otf}[Renderer=HarfBuzz, BoldFont=NotoSerifSC-Bold.otf]字\textbf{字}
        \end{document}
        EOF
        lualatex -interaction=nonstopmode -halt-on-error warm.tex
      '';

      # The typeset scripts source this before LuaLaTeX. luaotfload reads its font database only from the writable cache.
      typesetEnv = pkgs.writeTextDir "bin/typeset-env" ''
        export OSFONTDIR=${cjk} TEXMFCACHE=/tmp/texmf-var:${fontCache}
        mkdir -p /tmp/texmf-var/luatex-cache/generic
        cp -r ${fontCache}/luatex-cache/generic/names /tmp/texmf-var/luatex-cache/generic/
        chmod -R u+w /tmp/texmf-var/luatex-cache
      '';

      # Expose the Nix runtime as /usr/bin and /bin in mount.yaml.
      runtime = pkgs.buildEnv {
        name = "typeset-runtime";
        paths = [
          pkgs.bash
          pkgs.coreutils
          pkgs.ghostscript
          pkgs.librsvg
          pkgs.pandoc
          tex
          typesetEnv
        ];
      };

      root = pkgs.runCommand "typeset-root" { } ''
        mkdir -p $out/opt $out/tmp
        ln -s ${runtime} $out/opt/runtime
        ln -s ${fonts} $out/opt/fonts.conf
        ln -s ${pkgs.go-judge}/bin/go-judge $out/opt/go-judge
        ln -s ${pkgs.writeText "mount.yaml" (builtins.readFile ./mount.yaml)} $out/opt/mount.yaml
      '';

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

      image = n2c.buildImage {
        name = "gate-typeset";
        copyToRoot = root;
        layers = [
          base
          (layer [ tex ])
          (layer [ pkgs.pandoc ])
          (layer [
            pkgs.librsvg
            pkgs.dejavu_fonts
          ])
          (layer [ pkgs.ghostscript ])
          (layer [ cjk ])
          (layer [ fontCache ])
          (layer [ pkgs.go-judge ])
        ];
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
