{ pkgs, perSystem }:
perSystem.devshell.mkShell {
  packages = with pkgs; [
    delve
    go
    go-outline
    gotools
    just
    mockgen
    pgweb
    postgresql_17
    process-compose
    revive
  ];

  env = [
    {
      name = "NIX_PATH";
      value = "nixpkgs=${toString pkgs.path}";
    }
    {
      name = "NIX_DIR";
      eval = "$PRJ_ROOT/nix";
    }
  ];

  commands = [ ];
}
