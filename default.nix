{ pkgs ? import <nixpkgs> {} }:

  pkgs.buildGoModule rec {
    pname = "ts-restic-proxy";
    version = "0.0.1";
    src = ./.;
    vendorSha256 = null;
    subPackages = [ "cmd/ts-restic-proxy" ];
  }