{
  stdenvNoCC,
  nodejs,
  pnpm_10,
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
    pnpm = pnpm_10;
    fetcherVersion = 4;
    hash = "sha256-UrV6mXwm3qjq2p4SOvJmmWzx7HDfpvCipNVnGcKmdHY=";
  };

  nativeBuildInputs = [
    nodejs
    pnpmConfigHook
    pnpm_10
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
