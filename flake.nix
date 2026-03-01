{
  description = "Sidecar - companion TUI for CLI coding agents";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
  };

  outputs =
    {
      self,
      nixpkgs,
    }:
    let
      supportedSystems = [
        "x86_64-linux"
        "aarch64-linux"
        "x86_64-darwin"
        "aarch64-darwin"
      ];
      forAllSystems = nixpkgs.lib.genAttrs supportedSystems;
    in
    {
      packages = forAllSystems (
        system:
        let
          pkgs = nixpkgs.legacyPackages.${system};
          baseVersion = builtins.replaceStrings [ "\n" " " ] [ "" "" ] (builtins.readFile ./.version);
          version = "v${baseVersion}-dev+${self.shortRev or "dirty"}";
        in
        {
          sidecar = pkgs.buildGoModule {
            pname = "sidecar";
            inherit version;

            src = ./.;

            vendorHash = "sha256-UpccauexSoMQy76Xdyf7AcVlomMVd0M9PtISjWa5bio=";

            subPackages = [ "cmd/sidecar" ];

            ldflags = [
              "-s"
              "-w"
              "-X main.Version=${version}"
            ];

            meta = {
              description = "Sidecar for CLI agents - diffs, file trees, conversation history, and task management";
              homepage = "https://github.com/boozedog/sidecar";
              mainProgram = "sidecar";
            };
          };

          default = self.packages.${system}.sidecar;
        }
      );

      apps = forAllSystems (system: {
        default = {
          type = "app";
          program = "${self.packages.${system}.sidecar}/bin/sidecar";
        };
      });
    };
}
