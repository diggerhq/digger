{ pkgs, perSystem }:
perSystem.devshell.mkShell {
  packages = with pkgs; [
    delve
    go
    go-outline
    gotools
    mockgen
    pgweb
    postgresql_17
    revive
    process-compose
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
