{ pkgs ? import <nixpkgs> { } }:

pkgs.mkShell {
  name = "niimbot-go";

  buildInputs = [
    pkgs.go
    pkgs.git
  ];

  shellHook = ''
    # Keep caches inside the project so it also works under nix-build.
    export GOCACHE="$PWD/.gocache"
    export GOPATH="$PWD/.gopath"
    export PATH="$PWD/bin:$PATH"
  '';
}
