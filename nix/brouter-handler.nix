# The NixOS/Sway desktop handler package: the brouter binary plus a
# desktop entry declaring the http and https schemes and a thin wrapper
# that resolves the user config at runtime. All argument plumbing; no
# routing logic and no mutable installation steps.
{
  lib,
  buildGoModule,
  replaceVars,
  symlinkJoin,
  go_1_27,
  coreutils,
  dbus,
  src,
}:
let
  version = "0.1.0";

  # go.mod requires go >= 1.27; nixpkgs' default buildGoModule Go is older.
  buildBrouter = buildGoModule.override { go = go_1_27; };

  brouter = buildBrouter {
    pname = "brouter";
    inherit version;
    inherit src;
    # Pinned module hashes keep the build offline and reproducible.
    vendorHash = "sha256-auuwDfIhmAyLv8UjVz2eBFUXrRiNmvktifPxRKsEJxY=";
    # Testing is the pinned `make check` gate's job (it needs git, make,
    # and platform probes the sandbox does not carry); the package build
    # verifies compilation.
    doCheck = false;
  };
in
symlinkJoin {
  name = "brouter-handler-${version}";
  paths = [ brouter ];

  passthru.desktopEntryId = "brouter-handler.desktop";

  postBuild = ''
    install -Dm755 ${
      replaceVars ../scripts/nixos/brouter-handler-wrapper.sh.in {
        brouter = "${brouter}/bin/brouter";
        mkdir = "${coreutils}/bin/mkdir";
        date = "${coreutils}/bin/date";
        dbus_send = "${dbus}/bin/dbus-send";
      }
    } $out/bin/brouter-handler

    # The desktop Exec must reference THIS package output, so the
    # placeholder is substituted here where $out is the final path; a
    # substitution derivation would embed its own store path instead.
    install -Dm644 ${../scripts/nixos/brouter-handler.desktop.in} \
      $out/share/applications/brouter-handler.desktop
    substituteInPlace $out/share/applications/brouter-handler.desktop \
      --replace-fail "@packageBin@" "$out"
  '';

  meta = with lib; {
    description = "NixOS desktop URL handler routing http/https through brouter";
    platforms = platforms.linux ++ platforms.darwin;
    mainProgram = "brouter-handler";
  };
}
