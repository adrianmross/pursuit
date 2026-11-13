// Package cli is the agent-facing Pursuit command line.
package cli

import (
	"fmt"
	"io"
	"net/url"
	"strconv"

	pcli "github.com/adrianmross/appkit/cli"
	"github.com/pursuit/pursuit/internal/apiclient"
	"github.com/pursuit/pursuit/internal/config"
	"github.com/pursuit/pursuit/internal/version"
)

type (
	Flag      = pcli.Flag
	Arg       = pcli.Arg
	Command   = pcli.Command[exec]
	globals   = pcli.Globals
	exitError = pcli.ExitError
)

var exitStatus = pcli.ExitStatus

// exec is one invocation: the parsed command line plus Pursuit's config,
// client and where the token came from.
type exec struct {
	*pcli.Exec
	client *apiclient.Client
	cfg    config.Config
	// tokenSource is flag, env, config, a credential storage name
	// (file, macos-keychain, secret-service), or none.
	tokenSource string
}

// Run executes one CLI invocation. stdout and stderr are separate so tests
// can assert the JSON error channel.
func Run(args []string, stdout, stderr io.Writer) int {
	return pcli.Run(pcli.AppInfo{Name: "pursuit", EnvPrefix: "PURSUIT_"}, rootCommand(), connect, args, stdout, stderr)
}

// connect loads the config and the credential for one invocation.
func connect(x *pcli.Exec, cmd *Command) (*exec, error) {
	g := x.G
	cfg, err := config.Load(g.Config)
	if err != nil {
		return nil, &exitError{Code: 1, Name: "config", Message: err.Error(), Hint: "check --config or PURSUIT_CONFIG"}
	}
	if g.Server != "" {
		cfg.ServerURL = g.Server
	}
	if g.Token != "" {
		cfg.Token = g.Token
	}
	// --token, PURSUIT_TOKEN, and a config-file token always beat a stored
	// `pursuit auth login` credential, so agent workflows are unchanged.
	source := tokenSource(g, cfg)
	if cfg.Token == "" && cmd.Name != "serve" {
		c, ok, err := config.LoadCredential(cfg.ServerURL)
		switch {
		// login and logout are how you recover from a broken credential.
		case err != nil && cmd.Name != "logout" && cmd.Name != "login":
			// A locked or denied keychain is not "logged out": say so.
			return nil, &exitError{Code: 5, Message: "stored credential unavailable: " + err.Error(), Hint: "unlock the keychain, pass --token or PURSUIT_TOKEN, or run pursuit auth logout"}
		case ok:
			cfg.Token, source = c.Token, c.Storage
		}
	}
	e := &exec{
		Exec:        x,
		client:      &apiclient.Client{BaseURL: cfg.ServerURL, Token: cfg.Token, Bypass: cfg.VercelBypass},
		cfg:         cfg,
		tokenSource: source,
	}
	x.Do = func(method, path string, q url.Values, body []byte, contentType string, opt pcli.Options) (pcli.Response, error) {
		r, err := e.client.Do(nil, method, path, q, body, contentType, apiclient.Options(opt))
		return pcli.Response{Status: r.Status, Body: r.Body, ContentType: r.ContentType}, err
	}
	return e, nil
}

func atoiFlag(v string) (int, error) {
	n, err := strconv.Atoi(v)
	if err != nil || n < 1 {
		return 0, &exitError{Code: 2, Message: "invalid --version", Hint: "version is a positive integer"}
	}
	return n, nil
}

func localVersion(stdout io.Writer, jsonOut bool) {
	if jsonOut {
		fmt.Fprintf(stdout, "{\"version\":%q}\n", version.Version)
		return
	}
	fmt.Fprintln(stdout, version.Version)
}
