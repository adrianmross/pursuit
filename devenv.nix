{ pkgs, lib, config, ... }:

{
  packages = [ pkgs.git ];

  languages.go.enable = true;

  scripts.pursuit-build.exec = "go build -o bin/pursuit ./cmd/pursuit";
  scripts.pursuit-serve.exec = "go run ./cmd/pursuit serve";
  scripts.pursuit-test.exec = "go test ./...";
  scripts.pursuit-lint.exec = "go vet ./...";
  scripts.pursuit-run.exec = "pursuit-build && ./bin/pursuit serve";
  scripts.pursuit-dev.exec = "pursuit-build && portless pursuit ./bin/pursuit serve";

  enterShell = ''
    echo "pursuit devenv: pursuit-build, pursuit-run, pursuit-dev (portless), pursuit-test, pursuit-lint, or pursuit-serve directly."
  '';

  # Local Postgres instead of SQLite: devenv up --profile postgres
  profiles.postgres.module = {
    services.postgres = {
      enable = true;
      initialDatabases = [{ name = "pursuit"; }];
      listen_addresses = "127.0.0.1";
      initialScript = ''
        alter user postgres with password 'pursuit';
      '';
    };
    env.PURSUIT_DATABASE_URL = "postgres://postgres:pursuit@127.0.0.1:5432/pursuit?sslmode=disable";
  };

  # Existing Neon database: devenv shell --profile neon, with
  # PURSUIT_DATABASE_URL already exported (e.g. from a local .env.neon you source).
  profiles.neon.module = {
    dotenv.enable = true;
    dotenv.filename = ".env.neon";
  };

  # Browser extension (extension/): devenv shell --profile extension
  profiles.extension.module = {
    languages.javascript.enable = true;
    languages.javascript.package = pkgs.nodejs_22;
    scripts.pursuit-extension-build.exec = "cd extension && npm install && npm run build";
    scripts.pursuit-extension-test.exec = "cd extension && npm install && npm test";
  };
}
