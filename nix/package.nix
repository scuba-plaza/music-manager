{
  lib,
  buildGoModule,
  makeWrapper,
  yt-dlp,
  ffmpeg,
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

  vendorHash = "sha256-svoKbAfcQrOchgor8pAmA5bbcZ3EYLontTeTUXKrKSw=";

  env.CGO_ENABLED = "0";

  ldflags = [
    "-s"
    "-w"
    "-X music-manager/internal/version.Version=${version}"
    "-X music-manager/internal/version.Commit=${commit}"
  ];

  nativeBuildInputs = [ makeWrapper ];

  # yt-dlp and ffmpeg are only fallbacks, appended to PATH: one the user
  # already has, say a newer yt-dlp, is found first.
  postInstall = ''
    wrapProgram $out/bin/mm --suffix PATH : ${lib.makeBinPath [ yt-dlp ffmpeg ]}

    mkdir -p $out/share/bash-completion/completions $out/share/zsh/site-functions $out/share/fish/vendor_completions.d
    $out/bin/mm completion bash > $out/share/bash-completion/completions/mm
    $out/bin/mm completion zsh > $out/share/zsh/site-functions/_mm
    $out/bin/mm completion fish > $out/share/fish/vendor_completions.d/mm.fish
  '';

  meta = {
    description = "Manage a music library for an MP3 player: download from YouTube, tag from MusicBrainz, render a clean folder";
    mainProgram = "mm";
    platforms = [ "x86_64-linux" "aarch64-linux" ];
  };
}
