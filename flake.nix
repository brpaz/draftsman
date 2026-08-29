{
  description = "draftsman - Conventional Commits release notes generator";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = {
    self,
    nixpkgs,
    flake-utils,
  }:
    flake-utils.lib.eachDefaultSystem (
      system: let
        pkgs = nixpkgs.legacyPackages.${system};
        version = self.shortRev or self.dirtyShortRev or "0.0.0-dev";
      in {
        packages.default = pkgs.buildGoModule {
          pname = "draftsman";
          inherit version;

          src = ./.;

          vendorHash = "sha256-Tn3ZGlDtl1Xox9KqQXfgKXx4dKeVDI4tif/OtpQuvmU=";

          subPackages = ["cmd/draftsman"];

          ldflags = [
            "-s"
            "-w"
            "-X main.Version=${version}"
            "-X main.Commit=${self.rev or "unknown"}"
          ];

          meta = with pkgs.lib; {
            description = "Generates release notes from Conventional Commits, publishing them as a live draft release across GitHub, Gitea, and Forgejo";
            homepage = "https://github.com/brpaz/draftsman";
            license = licenses.mit;
            mainProgram = "draftsman";
          };
        };

        apps.default = flake-utils.lib.mkApp {
          drv = self.packages.${system}.default;
          name = "draftsman";
        };
      }
    );
}
