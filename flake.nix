{
  description = "Manage a music library for an MP3 player";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";

  outputs =
    { self, nixpkgs }:
    let
      systems = [ "x86_64-linux" "aarch64-linux" ];
      forAll = f: nixpkgs.lib.genAttrs systems (system: f nixpkgs.legacyPackages.${system});
      version = if self ? shortRev then "git-${self.shortRev}" else "dev";
      commit = self.rev or self.dirtyRev or "";
    in
    {
      overlays.default = final: _prev: {
        music-manager = final.callPackage ./nix/package.nix {
          inherit version commit;
          frontend = final.callPackage ./nix/frontend.nix { inherit version; };
        };
      };

      packages = forAll (pkgs: rec {
        frontend = pkgs.callPackage ./nix/frontend.nix { inherit version; };
        music-manager = pkgs.callPackage ./nix/package.nix { inherit version commit frontend; };
        default = music-manager;
      });

      homeManagerModules.default = import ./nix/hm-module.nix self;

      apps = forAll (pkgs: {
        default = {
          type = "app";
          program = "${self.packages.${pkgs.stdenv.hostPlatform.system}.default}/bin/mm";
          meta.description = self.packages.${pkgs.stdenv.hostPlatform.system}.default.meta.description;
        };
      });

      devShells = forAll (pkgs: {
        default = pkgs.mkShell {
          packages = with pkgs; [
            go
            nodejs
            pnpm
            yt-dlp
            ffmpeg
            playwright-driver.browsers
          ];
          PLAYWRIGHT_BROWSERS_PATH = "${pkgs.playwright-driver.browsers}";
          PLAYWRIGHT_SKIP_VALIDATE_HOST_REQUIREMENTS = "true";
        };
      });

      checks = forAll (
        pkgs:
        let
          inherit (pkgs) lib;
          mm = self.packages.${pkgs.stdenv.hostPlatform.system}.default;
          evaluated =
            (lib.evalModules {
              modules = [
                self.homeManagerModules.default
                {
                  options = {
                    home.packages = lib.mkOption { type = lib.types.listOf lib.types.package; default = [ ]; };
                    home.sessionVariables = lib.mkOption { type = lib.types.attrsOf lib.types.str; default = { }; };
                  };
                  config = {
                    _module.args.pkgs = pkgs;
                    programs.music-manager = {
                      enable = true;
                      package = mm;
                      library = "/home/me/Music";
                    };
                  };
                }
              ];
            }).config;
        in
        {
          frontend = self.packages.${pkgs.stdenv.hostPlatform.system}.frontend;
          build = mm;
          hm-module = pkgs.runCommand "hm-module" { } ''
            test "${toString evaluated.home.packages}" = "${mm}"
            test "${evaluated.home.sessionVariables.MM_LIBRARY}" = "/home/me/Music"
            touch $out
          '';
        }
      );
    };
}
