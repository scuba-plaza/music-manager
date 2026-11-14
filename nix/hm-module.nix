self:
{ config, lib, pkgs, ... }:

let
  cfg = config.programs.music-manager;
  toml = pkgs.formats.toml { };

  field = type: description: lib.mkOption {
    type = lib.types.nullOr type;
    default = null;
    inherit description;
  };

  fields = {
    label = { key = "label"; type = lib.types.str; description = "Name shown in the format menu."; };
    latinNames = { key = "latin_names"; type = lib.types.bool; description = "Write file and folder names in Latin letters only."; };
    componentMax = { key = "component_max"; type = lib.types.ints.positive; description = "Longest folder or file name, in bytes."; };
    stemMax = { key = "stem_max"; type = lib.types.ints.positive; description = "Longest file name without its extension, in bytes."; };
    tagWidth = { key = "tag_width"; type = lib.types.ints.positive; description = "Longest title, artist and album tag, in characters."; };
    latinGenre = { key = "latin_genre"; type = lib.types.bool; description = "Write the genre tag in Latin letters only."; };
    coverFormat = { key = "cover_format"; type = lib.types.str; description = "Cover image encoding."; };
    coverMaxEdge = { key = "cover_max_edge"; type = lib.types.ints.positive; description = "Longest cover edge, in pixels."; };
    coverQuality = { key = "cover_quality"; type = lib.types.ints.positive; description = "JPEG quality of the cover, 1 to 100."; };
    singleCovers = { key = "single_covers"; type = lib.types.bool; description = "Give singles a cover of their own."; };
    replaygain = { key = "replaygain"; type = lib.types.bool; description = "Write ReplayGain tags."; };
    playlistExt = { key = "playlist_ext"; type = lib.types.str; description = "Playlist file extension."; };
    playlistStyle = { key = "playlist_style"; type = lib.types.str; description = "How playlist entries are written."; };
  };

  profile = lib.types.submodule {
    options = lib.mapAttrs (_: f: field f.type f.description) fields;
  };

  entry =
    name: device:
    { inherit name; }
    // lib.concatMapAttrs (
      option: f: lib.optionalAttrs (device.${option} != null) { ${f.key} = device.${option}; }
    ) fields;
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

    devices = lib.mkOption {
      type = lib.types.attrsOf profile;
      default = { };
      description = "Device profiles written to ~/.config/mm/devices.toml. A field left unset takes the default of the program.";
    };
  };

  config = lib.mkIf cfg.enable {
    home.packages = [ cfg.package ];
    home.sessionVariables = lib.mkIf (cfg.library != null) { MM_LIBRARY = cfg.library; };
    xdg.configFile = lib.mkIf (cfg.devices != { }) {
      "mm/devices.toml".source = toml.generate "devices.toml" {
        profile = lib.mapAttrsToList entry cfg.devices;
      };
    };
  };
}
