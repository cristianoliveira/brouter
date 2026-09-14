# Development shell plus the NixOS/Sway desktop handler package
# (`nix build .#brouter-handler`). The package carries the desktop
# entry and wrapper; routing logic stays in the Go binary.
{
  description = "brouter development shell and desktop handler package";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";

  outputs = { self, nixpkgs }:
    let
      systems = [
        "x86_64-linux"
        "aarch64-linux"
        "x86_64-darwin"
        "aarch64-darwin"
      ];
      forAllSystems = f:
        nixpkgs.lib.genAttrs systems
          (system: f nixpkgs.legacyPackages.${system});
    in
    {
      devShells = forAllSystems (pkgs: {
        default = pkgs.mkShell {
          packages = with pkgs; [
            go_1_27
            golangci-lint
            gnumake
            desktop-file-utils
          ];
        };
      });

      packages = forAllSystems (pkgs: {
        brouter-handler = pkgs.callPackage ./nix/brouter-handler.nix { src = self; };
        default = self.packages.${pkgs.system}.brouter-handler;
      });
    };
}
