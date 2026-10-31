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
        music-manager = final.callPackage ./nix/package.nix { inherit version commit; };
      };

      packages = forAll (pkgs: rec {
        music-manager = pkgs.callPackage ./nix/package.nix { inherit version commit; };
        default = music-manager;
      });

      apps = forAll (pkgs: {
        default = {
          type = "app";
          program = "${self.packages.${pkgs.system}.default}/bin/mm";
        };
      });

      devShells = forAll (pkgs: {
        default = pkgs.mkShell {
          packages = with pkgs; [
            go
            nodejs
            pnpm_10
            yt-dlp
            ffmpeg
            playwright-driver.browsers
          ];
          PLAYWRIGHT_BROWSERS_PATH = "${pkgs.playwright-driver.browsers}";
          PLAYWRIGHT_SKIP_VALIDATE_HOST_REQUIREMENTS = "true";
        };
      });

      checks = forAll (pkgs: {
        build = self.packages.${pkgs.system}.default;
      });
    };
}
