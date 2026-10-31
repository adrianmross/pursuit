{ pkgs, lib, config, ... }:

let
  # Container builds carry only the prebuilt binary, not the dev toolchain.
  dev = !config.container.isBuilding;

  # One description of the local stack, run two ways: devenv processes
  # (profiles.postgres) and docker-compose.yml (generated, see dev:compose).
  db = { name = "pursuit"; user = "pursuit"; pass = "pursuit"; };
  dbUrl = host: "postgres://${db.user}:${db.pass}@${host}:5432/${db.name}?sslmode=disable";
  replicas = { pursuit-a = 8787; pursuit-b = 8788; };

  compose = (pkgs.formats.yaml { }).generate "docker-compose.yml" {
    services = {
      postgres = {
        image = "postgres:${lib.versions.major config.services.postgres.package.version}-alpine";
        environment = { POSTGRES_USER = db.user; POSTGRES_PASSWORD = db.pass; POSTGRES_DB = db.name; };
        healthcheck = {
          test = [ "CMD-SHELL" "pg_isready -U ${db.user}" ];
          interval = "5s";
          timeout = "3s";
          retries = 10;
        };
      };
    } // lib.mapAttrs (name: port: {
      build = ".";
      environment = { PURSUIT_DATABASE_URL = dbUrl "postgres"; PURSUIT_ADDR = ":8787"; };
      ports = [ "${toString port}:8787" ];
      depends_on.postgres.condition = "service_healthy";
      healthcheck = {
        test = [ "CMD" "/pursuit" "ready" "--server" "http://127.0.0.1:8787" ];
        interval = "5s";
        timeout = "3s";
        retries = 10;
      };
    }) replicas;
  };
in
{
  # go.mod needs go 1.26; track pkgs.go rather than a pinned go_1_xx.
  languages.go = {
    enable = dev;
    package = pkgs.go;
  };

  # The binary go:embeds web-app and docs-site builds, so Node is always on.
  languages.javascript = {
    enable = dev;
    package = pkgs.nodejs_22;
  };

  packages = lib.optionals dev (with pkgs; [
    git
    gnumake
    entr
    jq
    curl
    ripgrep
    goreleaser
    actionlint
    nil
    nixd
    alejandra
    statix
    deadnix
  ]);

  enterShell = ''
    export XDG_CACHE_HOME="''${XDG_CACHE_HOME:-$HOME/.cache}"
    export XDG_DATA_HOME="''${XDG_DATA_HOME:-$HOME/.local/share}"
    export XDG_STATE_HOME="''${XDG_STATE_HOME:-$HOME/.local/state}"

    if [ "''${LOCAL_CACHE:-0}" = "1" ]; then
      export XDG_CACHE_HOME="$PWD/.cache"
      export XDG_DATA_HOME="$PWD/.local/share"
      export XDG_STATE_HOME="$PWD/.local/state"
    fi

    mkdir -p "$XDG_CACHE_HOME" "$XDG_DATA_HOME" "$XDG_STATE_HOME"

    echo "pursuit dev env"
    echo "  pursuit-run | pursuit-dev (portless) | pursuit-watch"
    echo "  changelogger            # add a release-note fragment"
    echo "  devenv tasks run dev:validate"
  '';

  # Pinned to the version CI's adrianmross/changelogger action installs.
  scripts.changelogger.exec = ''go run github.com/adrianmross/changelogger/cmd/changelogger@v0.10.0 "$@"'';

  scripts.versions.exec = ''
    go version
    node --version
    goreleaser --version | grep GitVersion
  '';

  scripts.pursuit-web.exec = "make web";
  scripts.pursuit-build.exec = "make build";
  scripts.pursuit-serve.exec = "go run ./cmd/pursuit serve";
  scripts.pursuit-test.exec = "go test ./...";
  scripts.pursuit-lint.exec = "go vet ./...";
  scripts.pursuit-run.exec = "pursuit-build && ./bin/pursuit serve";
  # Worktrunk names a worktree dir "<repo>.<branch>"; derive a distinct
  # portless name from it so a worktree's dev server never collides with
  # the primary checkout's (or another worktree's) "pursuit.localhost".
  scripts.pursuit-dev.exec = ''
    name="pursuit-$(basename "$PWD" | sed -n 's/^pursuit\.//p')"
    [ "$name" = "pursuit-" ] && name="pursuit"
    pursuit-build && portless "$name" ./bin/pursuit serve
  '';
  # Rebuilds and restarts on every .go change, kept alive at a stable
  # https://<name>.localhost URL by portless.
  scripts.pursuit-watch.exec = ''
    export PURSUIT_DATA_DIR="$PWD/.devdata"
    mkdir -p "$PURSUIT_DATA_DIR"
    find . -name '*.go' -not -path './bin/*' | entr -rn sh -c 'pursuit-dev'
  '';
  scripts.pursuit-extension-build.exec = "cd extension && npm install && npm run build";
  scripts.pursuit-extension-test.exec = "cd extension && npm install && npm test";

  tasks = {
    "dev:versions".exec = "versions";
    "dev:web".exec = "make web";
    "dev:vet".exec = "go vet ./...";
    "dev:test".exec = "make test";
    "dev:extension".exec = "pursuit-extension-test";
    "dev:release-check".exec = "goreleaser check && actionlint && changelogger check --base origin/main";
    "dev:validate".exec = "make test && go vet ./... && pursuit-extension-test && goreleaser check && actionlint";
    "dev:compose".exec = "{ echo '# Generated from devenv.nix by: devenv tasks run dev:compose'; cat ${compose}; } > docker-compose.yml";
  };

  enterTest = ''
    go version
    node --version
    goreleaser check
    actionlint
  '';

  # ponytail: copies a prebuilt binary (`make linux`), not a Nix build of the
  # app; a buildGoModule + buildNpmPackage derivation is the upgrade path.
  # Building on macOS needs a Linux builder; CI builds it on ubuntu.
  containers.serve = {
    name = "pursuit";
    copyToRoot = ./bin/linux;
    startupCommand = "/pursuit serve";
  };

  # Local Postgres and two replicas on :8787 and :8788, sharing it:
  # devenv up --profile postgres. Postgres alone: add `postgres` at the end.
  profiles.postgres.module = {
    tasks."pursuit:build".exec = "make build";
    processes = lib.mapAttrs (name: port: {
      exec = "./bin/pursuit serve";
      env.PURSUIT_ADDR = ":${toString port}";
      after = [ "pursuit:build" "devenv:processes:postgres" ];
    }) replicas;
    services.postgres = {
      enable = true;
      # devenv's superuser is the OS user, so give pursuit its own role.
      initialDatabases = [ db ];
      listen_addresses = "127.0.0.1";
    };
    env.PURSUIT_DATABASE_URL = dbUrl "127.0.0.1";
  };

  # Existing Neon database: devenv shell --profile neon, with
  # PURSUIT_DATABASE_URL already exported (e.g. from a local .env.neon you source).
  profiles.neon.module = {
    dotenv.enable = true;
    dotenv.filename = ".env.neon";
  };
}
