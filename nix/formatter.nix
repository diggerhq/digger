{
  flake,
  inputs,
  pkgs,
  ...
}:
let
  treefmt-settings = {
    package = pkgs.treefmt;
    projectRootFile = "flake.nix";
    programs = {
        deadnix.enable = true;
            nixfmt.enable = true;

            shellcheck.enable = true;
            shfmt.enable = true;

            yamlfmt.enable = true;
    };
    settings.formatter = {
        deadnix.pipeline = "nix";
            deadnix.priority = 1;
            nixfmt.pipeline = "nix";
            nixfmt.priority = 2;

            shellcheck.pipeline = "shell";
            shellcheck.includes = [
              "*.sh"
              "*.bash"
              "*.envrc"
              "*.envrc.*"
              "bin/*"
            ];
            shellcheck.priority = 1;
            shfmt.pipeline = "shell";
            shfmt.includes = [
              "*.sh"
              "*.bash"
              "*.envrc"
              "*.envrc.*"
              "bin/*"
            ];
            shfmt.priority = 2;

            yamlfmt.pipeline = "yaml";
            yamlfmt.priority = 1;
    };

      };

  formatter = inputs.treefmt-nix.lib.mkWrapper pkgs treefmt-settings;

  check =
    pkgs.runCommand "format-check"
      {
        nativeBuildInputs = [
          formatter
          pkgs.git
        ];

        # only check on Linux
        meta.platforms = pkgs.lib.platforms.linux;
      }
      ''
        export HOME=$NIX_BUILD_TOP/home

        # keep timestamps so that treefmt is able to detect mtime changes
        cp --no-preserve=mode --preserve=timestamps -r ${flake} source
        cd source
        git init --quiet
        git add .
        treefmt --no-cache
        if ! git diff --exit-code; then
          echo "-------------------------------"
          echo "aborting due to above changes ^"
          exit 1
        fi
        touch $out
      '';
in
formatter
// {
  meta = formatter.meta // {
    tests = {
      check = check;
    };
  };
}
