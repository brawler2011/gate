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

      # metafont generates Cyrillic font metrics on demand.
      tex = pkgs.texliveBasic.withPackages (
        ps: with ps; [
          amsfonts
          amsmath
          babel-russian
          cm-super
          cyrillic
          graphics
          hyphen-russian
          lh
          metafont
        ]
      );

      # rsvg-convert renders SVG text with these fonts only.
      fonts = pkgs.makeFontsConf { fontDirectories = [ pkgs.dejavu_fonts ]; };

      # Expose the Nix runtime as /usr/bin and /bin in mount.yaml.
      runtime = pkgs.buildEnv {
        name = "typeset-runtime";
        paths = [
          pkgs.bash
          pkgs.coreutils
          pkgs.librsvg
          pkgs.pandoc
          tex
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
