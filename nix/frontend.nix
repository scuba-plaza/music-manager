{
  stdenvNoCC,
  nodejs,
  pnpm,
  fetchPnpmDeps,
  pnpmConfigHook,
  version,
}:

stdenvNoCC.mkDerivation (finalAttrs: {
  pname = "music-manager-web";
  inherit version;

  src = ../web;

  pnpmDeps = fetchPnpmDeps {
    inherit (finalAttrs) pname version src;
    pnpm = pnpm;
    fetcherVersion = 4;
    hash = "sha256-KwsVljP8pE/20xK6V7PH5zZ0Z+9LQ7IRRi4aVU+BdJM=";
  };

  nativeBuildInputs = [
    nodejs
    pnpmConfigHook
    pnpm
  ];

  env = {
    PLAYWRIGHT_SKIP_BROWSER_DOWNLOAD = "1";
    npm_config_manage_package_manager_versions = "false";
  };

  buildPhase = ''
    runHook preBuild
    pnpm run build:nix
    runHook postBuild
  '';

  doCheck = true;

  checkPhase = ''
    runHook preCheck
    pnpm run check
    runHook postCheck
  '';

  installPhase = ''
    runHook preInstall
    cp -r dist $out
    runHook postInstall
  '';
})