{ pkgs ? import <nixpkgs> { } }:

pkgs.mkShell {
  name = "niimbot-go";

  # dejavu/dejavu_fonts and dmtxutils/dmtx-utils are named differently
  # across nixpkgs channels; accept either.
  buildInputs = [
    pkgs.go
    pkgs.git
    pkgs.fontconfig
    pkgs.dejavu_fonts
    pkgs.dmtx-utils
  ];

  shellHook = ''
    # Keep caches inside the project so it also works under nix-build.
    export GOCACHE="$PWD/.gocache"
    export GOPATH="$PWD/.gopath"
    export PATH="$PWD/bin:$PATH"
  '';
}
