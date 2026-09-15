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
  pkgs,
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
  # Isolated route-script helper (TASK-0029): the vendored, pinned
  # C Lua 5.4.7 sources compiled as a standalone process, so the Go
  # router stays CGO-free. Capability libraries (os, io, package,
  # debug) are excluded from the link; the helper opens only
  # base/table/string.
  luaExclude = [
    "lua.c" "luac.c" "onelua.c" "linit.c" "loadlib.c" "liolib.c"
    "loslib.c" "ldblib.c" "lmathlib.c" "lutf8lib.c"
  ];
  # Pinned, hash-verified Lua 5.4.7 source (same pin as
  # scripts/fetch-lua.sh). The helper links only the safe subset.
  luaSrc = pkgs.fetchzip {
    url = "https://www.lua.org/ftp/lua-5.4.7.tar.gz";
    sha256 = "sha256-o1jnQCcnlk2OXU4Cc8X7bJiOU2QN92vd91TtBhNM/hU=";
  };
  luaSrcDir = "${luaSrc}/src";
  luaSources = map (name: luaSrcDir + ("/" + name)) (
    builtins.filter (
      name: lib.hasSuffix ".c" name && !(builtins.elem name luaExclude)
    ) (builtins.attrNames (builtins.readDir luaSrcDir))
  );
  luaHelper = stdenv.mkDerivation {
    pname = "brouter-lua-helper";
    inherit version;
    src = ../native/lua-helper;
    dontConfigure = true;
    buildPhase = ''
      cc -O2 -I ${luaSrcDir} lua_helper.c ${
        lib.concatStringsSep " " luaSources
      } -lm -o brouter-lua-helper
    '';
    installPhase = "install -Dm755 brouter-lua-helper $out/bin/brouter-lua-helper";
    meta = with lib; {
      description = "Isolated C Lua route-script evaluator for brouter";
      platforms = [ "aarch64-darwin" "x86_64-linux" "aarch64-linux" ];
    };
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
      # User-selected menu icon (TASK-0026 candidate C): 1x/2x template
      # monochrome PNGs loaded by the shim from bundle Resources.
      mkdir -p $app/Contents/Resources
      install -m644 menu-icon.png $app/Contents/Resources/menu-icon.png
      install -m644 menu-icon@2x.png $app/Contents/Resources/menu-icon@2x.png
      cc -arch arm64 -fobjc-arc \
        -framework Foundation -framework CoreServices -framework AppKit \
        -o $app/Contents/MacOS/BrouterHandler main.m
      install -m755 ${brouter}/bin/brouter $app/Contents/MacOS/brouter
      install -m755 ${luaHelper}/bin/brouter-lua-helper $app/Contents/MacOS/brouter-lua-helper
      install -m755 ${brouter}/bin/brouter $out/bin/brouter
      install -m755 ${luaHelper}/bin/brouter-lua-helper $out/bin/brouter-lua-helper
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
    paths = [ brouter luaHelper ];

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
