{
  pkgs,
  lib,
  config,
  inputs,
  ...
}: {
  languages = {
    go = {
      enable = true;
      enableHardeningWorkaround = true;
      package = pkgs.go;
    };
  };

  # https://devenv.sh/packages/
  packages = with pkgs; [
    go-task
    gotestsum
    goreleaser
    gomarkdoc
    lefthook
    commitlint-rs
    zensical
    shellcheck
  ];

  enterShell = ''
    lefthook install
    go mod tidy
  '';

  # https://devenv.sh/tests/
  enterTest = ''
    go test ./... -v
  '';
}
