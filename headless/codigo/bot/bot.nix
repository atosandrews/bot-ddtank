{ pkgs ? import <nixpkgs> {} }:
pkgs.mkShell {
  packages = [ pkgs.go pkgs.onnxruntime ];
  DDBOT_ONNXRUNTIME_LIBRARY = "${pkgs.onnxruntime}/lib/libonnxruntime.so";
}
