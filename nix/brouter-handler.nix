# The platform-dispatched desktop handler package.
#
# Darwin: the brouter CLI plus a macOS app bundle at
# $out/Applications/BrouterHandler.app (native shim + embedded CLI,
# deterministic arm64, ad-hoc signed).
# Linux: the brouter CLI plus the desktop wrapper and entry declaring
# the http and https schemes.
#
# Both are argument plumbing around the same Go binary; no routing
# logic and no mutable installation steps live in Nix.
{
  lib,
  stdenv,
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
if stdenv.hostPlatform.isDarwin then
  # One derivation holding regular files: a bundle whose Info.plist or
  # Mach-O contents are symlinks (as symlinkJoin produces) fails the
  # code seal, so the app tree is copied, then signed after assembly.
  stdenv.mkDerivation {
    pname = "brouter-handler";
    inherit version;
    src = ../native/macos;

    dontConfigure = true;

    buildPhase = ''
      app=$out/Applications/BrouterHandler.app
      mkdir -p $app/Contents/MacOS $out/bin

      install -Dm644 Info.plist $app/Contents/Info.plist
      cc -arch arm64 -fobjc-arc \
        -framework Foundation -framework CoreServices \
        -o $app/Contents/MacOS/BrouterHandler main.m
      install -m755 ${brouter}/bin/brouter $app/Contents/MacOS/brouter
      install -m755 ${brouter}/bin/brouter $out/bin/brouter
    '';
    # The stdenv fixup phase strips binaries AFTER buildPhase, which
    # would invalidate any earlier signature; signing in postFixup is
    # therefore the final mutation of the sealed tree.
    postFixup = ''
      /usr/bin/codesign --force --sign - $out/Applications/BrouterHandler.app
    '';
    installPhase = "runHook postInstall";

    meta = with lib; {
      description = "brouter CLI and macOS URL handler app";
      # arm64 only: the shim is compiled -arch arm64 and Intel is out of
      # scope; x86_64-darwin packages are not produced at all.
      platforms = [ "aarch64-darwin" ];
      mainProgram = "brouter";
    };
  }
else
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
      platforms = platforms.linux;
      mainProgram = "brouter-handler";
    };
  }
