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
    echo "  pursuit-run | pursuit-dev (portless) | pursuit-watch | pursuit-preview (hot reload)"
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
  # Hot-reloading preview of this worktree: next dev + Go API, via portless.
  # `pursuit-preview up|down|status|logs` runs it detached; any number of
  # worktrees can have one up at once.
  scripts.pursuit-preview.exec = ''bash scripts/preview.sh "$@"'';
  scripts.pursuit-watch.exec = ''
    export PURSUIT_DATA_DIR="$PWD/.devdata"
    mkdir -p "$PURSUIT_DATA_DIR"
    find . -name '*.go' -not -path './bin/*' | entr -rn sh -c 'pursuit-dev'
  '';
  scripts.pursuit-extension-build.exec = "cd extension && npm install && npm run build";
  scripts.pursuit-extension-test.exec = "cd extension && npm install && npm test";
  # The spec compiles ./cmd/pursuit, which embeds the dashboard: run make web first.
  scripts.pursuit-extension-e2e.exec = "cd extension && npm install && npx playwright install chromium && npm run e2e";

  tasks = {
    "dev:versions".exec = "versions";
    "dev:web".exec = "make web";
    "dev:vet".exec = "go vet ./...";
    "dev:test".exec = "make test";
    "dev:extension".exec = "pursuit-extension-test";
    "dev:extension-e2e".exec = "make web && pursuit-extension-e2e";
    "dev:release-check".exec = "goreleaser check && actionlint && changelogger check --base origin/main";
    "dev:validate".exec = "make test && go vet ./... && pursuit-extension-test && pursuit-extension-e2e && goreleaser check && actionlint";
    "dev:compose".exec = "{ echo '# Generated from devenv.nix by: devenv tasks run dev:compose'; cat ${compose}; } > docker-compose.yml";
    # Pin the Dockerfile's images and go.mod's toolchain to devenv's Go and Node.
    "dev:pins".exec = ''
      sed -i.bak -E \
        -e 's/^ARG GO_VERSION=.*/ARG GO_VERSION=${config.languages.go.package.version}/' \
        -e 's/^ARG NODE_VERSION=.*/ARG NODE_VERSION=${config.languages.javascript.package.version}/' \
        Dockerfile && rm Dockerfile.bak
      go mod edit -toolchain=go${config.languages.go.package.version}
    '';
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

  # Local S3 for document storage: devenv up --profile s3, alone or with
  # --profile postgres. It makes the bucket (the server never does) and
  # points a locally run pursuit at it.
  profiles.s3.module = {
    services.minio = {
      enable = true;
      listenAddress = "127.0.0.1:9000";
      accessKey = "pursuit";
      secretKey = "pursuit-dev-secret";
      buckets = [ "pursuit" ];
    };
    env = {
      PURSUIT_S3_BUCKET = "pursuit";
      PURSUIT_S3_ENDPOINT = "http://127.0.0.1:9000";
      PURSUIT_S3_PATH_STYLE = "true";
      PURSUIT_S3_ACCESS_KEY_ID = "pursuit";
      PURSUIT_S3_SECRET_ACCESS_KEY = "pursuit-dev-secret";
      PURSUIT_S3_REGION = "us-east-1";
    };
  };

  # Existing Neon database: devenv shell --profile neon, with
  # PURSUIT_DATABASE_URL already exported (e.g. from a local .env.neon you source).
  profiles.neon.module = {
    dotenv.enable = true;
    dotenv.filename = ".env.neon";
  };
}
