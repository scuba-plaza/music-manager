self:
{ config, lib, pkgs, ... }:

let
  cfg = config.programs.music-manager;
in
{
  options.programs.music-manager = {
    enable = lib.mkEnableOption "mm, the music library manager";

    package = lib.mkOption {
      type = lib.types.package;
      default = self.packages.${pkgs.stdenv.hostPlatform.system}.default;
      defaultText = lib.literalExpression "music-manager.packages.\${system}.default";
      description = "The mm package to install.";
    };

    library = lib.mkOption {
      type = lib.types.nullOr lib.types.str;
      default = null;
      example = "/home/me/Music";
      description = "Library directory, exported as MM_LIBRARY.";
    };
  };

  config = lib.mkIf cfg.enable {
    home.packages = [ cfg.package ];
    home.sessionVariables = lib.mkIf (cfg.library != null) { MM_LIBRARY = cfg.library; };
  };
}
