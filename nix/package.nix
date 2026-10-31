{
  lib,
  buildGoModule,
  makeWrapper,
  yt-dlp,
  ffmpeg,
  xdg-utils,
  version,
  commit,
}:

buildGoModule {
  pname = "music-manager";
  inherit version;

  src = lib.fileset.toSource {
    root = ../.;
    fileset = lib.fileset.unions [
      ../go.mod
      ../go.sum
      ../cmd
      ../internal
    ];
  };

  vendorHash = "sha256-nA2Evpdnm6UFiXDygjzJQFU+7OK3+cgw6ItumlwyPTI=";

  env.CGO_ENABLED = "0";

  ldflags = [
    "-s"
    "-w"
    "-X music-manager/internal/version.Version=${version}"
    "-X music-manager/internal/version.Commit=${commit}"
  ];

  nativeBuildInputs = [ makeWrapper ];

  # yt-dlp, ffmpeg and xdg-open (which opens the web interface in a browser)
  # are only fallbacks, appended to PATH: one the user already has, say a newer
  # yt-dlp, is found first.
  postInstall = ''
    wrapProgram $out/bin/mm --suffix PATH : ${lib.makeBinPath [ yt-dlp ffmpeg xdg-utils ]}
  '';

  meta = {
    description = "Manage a music library for an MP3 player: download from YouTube, tag from MusicBrainz, render a clean folder";
    mainProgram = "mm";
    platforms = [ "x86_64-linux" "aarch64-linux" ];
  };
}
