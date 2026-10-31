// Package cli is the agent-facing Pursuit command line.
package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/pursuit/pursuit/internal/apiclient"
	"github.com/pursuit/pursuit/internal/config"
	"github.com/pursuit/pursuit/internal/version"
)

type Flag struct {
	Name       string `json:"name"`
	Type       string `json:"type"`
	Help       string `json:"help"`
	Repeatable bool   `json:"repeatable,omitempty"`
}

type Arg struct {
	Name     string `json:"name"`
	Required bool   `json:"required"`
	Help     string `json:"help,omitempty"`
	Many     bool   `json:"many,omitempty"`
}

type Command struct {
	Name     string     `json:"name"`
	Summary  string     `json:"summary"`
	Usage    string     `json:"usage,omitempty"`
	Example  string     `json:"example,omitempty"`
	Mutating bool       `json:"mutating,omitempty"`
	Args     []Arg      `json:"args,omitempty"`
	Flags    []Flag     `json:"flags,omitempty"`
	Children []*Command `json:"-"`
	Run      func(*exec) error `json:"-"`
}

type globals struct {
	JSON    bool
	Text    bool
	Help    bool
	DryRun  bool
	AllPages bool
	Server  string
	Token   string
	Config  string
	Idem    string
	Fields  string
}

type exec struct {
	cmd    *Command
	args   []string
	flags  map[string][]string
	g      globals
	json   bool
	stdout io.Writer
	stderr io.Writer
	client *apiclient.Client
	cfg    config.Config
	// tokenSource is flag, env, config, a credential storage name
	// (file, macos-keychain, secret-service), or none.
	tokenSource string
}

type exitError struct {
	code    int
	raw     []byte
	message string
	hint    string
}

func (e *exitError) Error() string {
	if e.message != "" {
		return e.message
	}
	return "exit " + strconv.Itoa(e.code)
}

// Run executes one CLI invocation. stdout and stderr are separate so tests
// can assert the JSON error channel.
func Run(args []string, stdout, stderr io.Writer) int {
	if stdout == nil {
		stdout = os.Stdout
	}
	if stderr == nil {
		stderr = os.Stderr
	}
	rest, g, err := splitGlobals(args)
	if err != nil {
		writeErr(stderr, 2, "usage", err.Error(), "run pursuit --help")
		return 2
	}
	root := rootCommand()
	if len(rest) == 0 {
		if g.Help && g.JSON {
			writeSchemaIndex(stdout)
			return 0
		}
		if g.Help {
			fmt.Fprint(stdout, helpText(root, "pursuit"))
			return 0
		}
		writeErr(stderr, 2, "usage", "missing command", "run pursuit --help")
		return 2
	}
	if rest[0] == "schema" {
		return runSchema(rest[1:], g, stdout, stderr)
	}
	cmd, left, err := walk(root, rest)
	if err != nil {
		writeErr(stderr, 2, "usage", err.Error(), "run pursuit --help")
		return 2
	}
	path := commandPath(root, rest)
	if g.Help {
		if g.JSON {
			enc := json.NewEncoder(stdout)
			enc.SetEscapeHTML(false)
			_ = enc.Encode(schemaFor(cmd, path))
			return 0
		}
		fmt.Fprint(stdout, helpText(cmd, path))
		return 0
	}
	if cmd.Run == nil {
		writeErr(stderr, 2, "usage", path+" needs a subcommand", "run "+path+" --help")
		return 2
	}
	parsed, err := parseFlags(left, cmd.Flags)
	if err != nil {
		writeErr(stderr, 2, "usage", err.Error(), "run "+path+" --help")
		return 2
	}
	if err := checkArgs(cmd, parsed.args); err != nil {
		writeErr(stderr, 2, "usage", err.Error(), "run "+path+" --help")
		return 2
	}
	if g.DryRun && !cmd.Mutating {
		writeErr(stderr, 2, "usage", "--dry-run is only valid on mutations", "remove --dry-run")
		return 2
	}
	cfg, err := config.Load(g.Config)
	if err != nil {
		writeErr(stderr, 1, "config", err.Error(), "check --config or PURSUIT_CONFIG")
		return 1
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
		case err != nil && cmd.Name != "logout":
			// A locked or denied keychain is not "logged out": say so.
			writeErr(stderr, 5, "unauthorized", "stored credential unavailable: "+err.Error(), "unlock the keychain, pass --token or PURSUIT_TOKEN, or run pursuit auth logout")
			return 5
		case ok:
			cfg.Token, source = c.Token, c.Storage
		}
	}
	jsonOut := g.JSON || !isTTY(stdout)
	if g.Text {
		jsonOut = false
	}
	if g.JSON {
		jsonOut = true
	}
	e := &exec{
		cmd: cmd, args: parsed.args, flags: parsed.flags, g: g, json: jsonOut,
		stdout: stdout, stderr: stderr,
		client:      &apiclient.Client{BaseURL: cfg.ServerURL, Token: cfg.Token},
		tokenSource: source,
	}
	e.cfg = cfg
	if err := cmd.Run(e); err != nil {
		var ex *exitError
		if errors.As(err, &ex) {
			if ex.raw != nil {
				writeRawErr(stderr, ex.raw)
			} else if ex.message != "" {
				writeErr(stderr, ex.code, codeName(ex.code), ex.message, ex.hint)
			}
			return ex.code
		}
		writeErr(stderr, 1, "internal", err.Error(), "")
		return 1
	}
	return 0
}

func codeName(code int) string {
	switch code {
	case 2:
		return "usage"
	case 3:
		return "not_found"
	case 4:
		return "conflict"
	case 5:
		return "unauthorized"
	case 6:
		return "validation"
	default:
		return "error"
	}
}

func writeErr(w io.Writer, _ int, code, message, hint string) {
	payload := map[string]any{"code": code, "message": message}
	if hint != "" {
		payload["hint"] = hint
	}
	b, _ := json.Marshal(map[string]any{"error": payload})
	fmt.Fprintf(w, "%s\n", b)
}

func writeRawErr(w io.Writer, b []byte) {
	b = bytes.TrimSpace(b)
	if len(b) == 0 || b[0] != '{' {
		writeErr(w, 1, "error", string(b), "")
		return
	}
	if b[len(b)-1] != '\n' {
		b = append(b, '\n')
	}
	_, _ = w.Write(b)
}

func isTTY(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	st, err := f.Stat()
	if err != nil {
		return false
	}
	return st.Mode()&os.ModeCharDevice != 0
}

func splitGlobals(args []string) ([]string, globals, error) {
	var g globals
	var rest []string
	for i := 0; i < len(args); i++ {
		name, val, has := splitEq(args[i])
		switch name {
		case "--json":
			on, n, err := takeBool(val, has, args, i)
			if err != nil {
				return nil, g, err
			}
			i += n
			g.JSON = on
		case "--text":
			on, n, err := takeBool(val, has, args, i)
			if err != nil {
				return nil, g, err
			}
			i += n
			g.Text = on
		case "--help", "-h":
			on, n, err := takeBool(val, has, args, i)
			if err != nil {
				return nil, g, err
			}
			i += n
			g.Help = on
		case "--dry-run":
			on, n, err := takeBool(val, has, args, i)
			if err != nil {
				return nil, g, err
			}
			i += n
			g.DryRun = on
		case "--all-pages":
			on, n, err := takeBool(val, has, args, i)
			if err != nil {
				return nil, g, err
			}
			i += n
			g.AllPages = on
		case "--server", "--token", "--config", "--idempotency-key", "--fields":
			v, n, err := takeVal(name, val, has, args, i)
			if err != nil {
				return nil, g, err
			}
			i += n
			switch name {
			case "--server":
				g.Server = v
			case "--token":
				g.Token = v
			case "--config":
				g.Config = v
			case "--idempotency-key":
				g.Idem = v
			case "--fields":
				g.Fields = v
			}
		default:
			rest = append(rest, args[i])
		}
	}
	return rest, g, nil
}

func splitEq(a string) (name, val string, has bool) {
	if !strings.HasPrefix(a, "-") {
		return a, "", false
	}
	if i := strings.IndexByte(a, '='); i >= 0 {
		return a[:i], a[i+1:], true
	}
	return a, "", false
}

func takeBool(val string, has bool, args []string, i int) (bool, int, error) {
	if has {
		return parseBool(val)
	}
	if i+1 < len(args) {
		if b, _, err := parseBool(args[i+1]); err == nil && (args[i+1] == "true" || args[i+1] == "false") {
			return b, 1, nil
		}
	}
	return true, 0, nil
}

func parseBool(s string) (bool, int, error) {
	switch strings.ToLower(s) {
	case "true", "1":
		return true, 0, nil
	case "false", "0":
		return false, 0, nil
	default:
		return false, 0, fmt.Errorf("invalid boolean %q", s)
	}
}

func takeVal(name, val string, has bool, args []string, i int) (string, int, error) {
	if has {
		if val == "" {
			return "", 0, fmt.Errorf("%s needs a value", name)
		}
		return val, 0, nil
	}
	if i+1 >= len(args) || strings.HasPrefix(args[i+1], "-") {
		return "", 0, fmt.Errorf("%s needs a value", name)
	}
	return args[i+1], 1, nil
}

func walk(root *Command, args []string) (*Command, []string, error) {
	cur := root
	i := 0
	for i < len(args) {
		if strings.HasPrefix(args[i], "-") {
			break
		}
		var next *Command
		for _, ch := range cur.Children {
			if ch.Name == args[i] {
				next = ch
				break
			}
		}
		if next == nil {
			break
		}
		cur = next
		i++
	}
	if cur == root {
		return nil, nil, fmt.Errorf("unknown command %q", args[0])
	}
	if i < len(args) && !strings.HasPrefix(args[i], "-") && len(cur.Children) > 0 {
		return nil, nil, fmt.Errorf("unknown command %q", strings.Join(args[:i+1], " "))
	}
	return cur, args[i:], nil
}

func commandPath(root *Command, args []string) string {
	parts := []string{"pursuit"}
	cur := root
	for _, a := range args {
		if strings.HasPrefix(a, "-") {
			break
		}
		var next *Command
		for _, ch := range cur.Children {
			if ch.Name == a {
				next = ch
				break
			}
		}
		if next == nil {
			break
		}
		parts = append(parts, next.Name)
		cur = next
	}
	return strings.Join(parts, " ")
}

func parseFlags(args []string, flags []Flag) (struct {
	args  []string
	flags map[string][]string
}, error) {
	known := map[string]Flag{}
	for _, f := range flags {
		known["--"+f.Name] = f
	}
	out := struct {
		args  []string
		flags map[string][]string
	}{flags: map[string][]string{}}
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			out.args = append(out.args, args[i+1:]...)
			break
		}
		if !strings.HasPrefix(a, "-") {
			out.args = append(out.args, a)
			continue
		}
		name, val, has := splitEq(a)
		f, ok := known[name]
		if !ok {
			return out, fmt.Errorf("unknown flag %s", name)
		}
		key := f.Name
		if f.Type == "bool" {
			on, n, err := takeBool(val, has, args, i)
			if err != nil {
				return out, err
			}
			i += n
			if on {
				out.flags[key] = []string{"true"}
			}
			continue
		}
		v, n, err := takeVal(name, val, has, args, i)
		if err != nil {
			return out, err
		}
		i += n
		if !f.Repeatable && len(out.flags[key]) > 0 {
			return out, fmt.Errorf("flag --%s was given twice", key)
		}
		out.flags[key] = append(out.flags[key], v)
	}
	return out, nil
}

func checkArgs(cmd *Command, args []string) error {
	need := 0
	many := false
	for _, a := range cmd.Args {
		if a.Required {
			need++
		}
		if a.Many {
			many = true
		}
	}
	if len(args) < need {
		return fmt.Errorf("%s requires %s", cmd.Name, cmd.Args[0].Name)
	}
	if !many && len(cmd.Args) > 0 && len(args) > len(cmd.Args) {
		return fmt.Errorf("unexpected argument %q", args[len(cmd.Args)])
	}
	if len(cmd.Args) == 0 && len(args) > 0 {
		return fmt.Errorf("unexpected argument %q", args[0])
	}
	return nil
}

func helpText(cmd *Command, path string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s — %s\n\n", path, cmd.Summary)
	if cmd.Usage != "" {
		fmt.Fprintf(&b, "Usage:\n  %s\n", cmd.Usage)
	} else {
		fmt.Fprintf(&b, "Usage:\n  %s\n", path)
	}
	if len(cmd.Children) > 0 {
		b.WriteString("\nCommands:\n")
		for _, ch := range cmd.Children {
			fmt.Fprintf(&b, "  %-16s %s\n", ch.Name, ch.Summary)
		}
	}
	if len(cmd.Flags) > 0 {
		b.WriteString("\nFlags:\n")
		for _, f := range cmd.Flags {
			extra := ""
			if f.Repeatable {
				extra = ", repeatable"
			}
			fmt.Fprintf(&b, "  --%-14s %s (%s%s)\n", f.Name, f.Help, f.Type, extra)
		}
	}
	b.WriteString("\nGlobal: --json --text --fields --all-pages --dry-run --idempotency-key --server --token\n")
	if cmd.Example != "" {
		fmt.Fprintf(&b, "\nExample:\n  %s\n", cmd.Example)
	}
	return b.String()
}

type schemaNode struct {
	Name     string   `json:"name"`
	Summary  string   `json:"summary"`
	Usage    string   `json:"usage,omitempty"`
	Example  string   `json:"example,omitempty"`
	Mutating bool     `json:"mutating,omitempty"`
	Args     []Arg    `json:"args,omitempty"`
	Flags    []Flag   `json:"flags,omitempty"`
	Children []schemaChild `json:"children,omitempty"`
}

type schemaChild struct {
	Name    string `json:"name"`
	Summary string `json:"summary"`
}

func schemaFor(cmd *Command, path string) schemaNode {
	n := schemaNode{
		Name: path, Summary: cmd.Summary, Usage: cmd.Usage, Example: cmd.Example,
		Mutating: cmd.Mutating, Args: cmd.Args, Flags: cmd.Flags,
	}
	for _, ch := range cmd.Children {
		n.Children = append(n.Children, schemaChild{Name: ch.Name, Summary: ch.Summary})
	}
	return n
}

func runSchema(args []string, g globals, stdout, stderr io.Writer) int {
	root := rootCommand()
	if len(args) == 0 {
		writeSchemaIndex(stdout)
		return 0
	}
	cmd, left, err := walk(root, args)
	if err != nil || len(left) > 0 {
		msg := "unknown command"
		if err != nil {
			msg = err.Error()
		}
		writeErr(stderr, 2, "usage", msg, "run pursuit schema")
		return 2
	}
	_ = g
	enc := json.NewEncoder(stdout)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(schemaFor(cmd, "pursuit "+strings.Join(args, " ")))
	return 0
}

func writeSchemaIndex(w io.Writer) {
	root := rootCommand()
	var items []schemaChild
	var walkFn func(prefix string, c *Command)
	walkFn = func(prefix string, c *Command) {
		for _, ch := range c.Children {
			name := strings.TrimSpace(prefix + " " + ch.Name)
			items = append(items, schemaChild{Name: name, Summary: ch.Summary})
			walkFn(name, ch)
		}
	}
	walkFn("pursuit", root)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(map[string]any{"commands": items})
}

func (e *exec) flag(name string) string {
	if v := e.flags[name]; len(v) > 0 {
		return v[0]
	}
	return ""
}

func (e *exec) send(method, path string, q url.Values, body []byte, contentType string, mut, fields bool) error {
	if fields && e.g.Fields != "" {
		if q == nil {
			q = url.Values{}
		}
		q.Set("fields", e.g.Fields)
	}
	opt := apiclient.Options{}
	if mut {
		opt.DryRun = e.g.DryRun
		opt.IdempotencyKey = e.g.Idem
	}
	if method == "GET" && e.g.AllPages {
		return e.sendAllPages(path, q)
	}
	resp, err := e.client.Do(nil, method, path, q, body, contentType, opt)
	if err != nil {
		return &exitError{code: 1, message: strings.TrimPrefix(err.Error(), "unavailable: "), hint: "check that pursuit serve is running and PURSUIT_SERVER is correct"}
	}
	if resp.Status >= 400 {
		return &exitError{code: exitStatus(resp.Status), raw: resp.Body}
	}
	return e.print(resp.Body, resp.ContentType)
}

// sendAllPages follows next_cursor and prints one page holding every item.
func (e *exec) sendAllPages(path string, q url.Values) error {
	if q == nil {
		q = url.Values{}
	}
	if q.Get("limit") == "" {
		q.Set("limit", "200")
	}
	var all []json.RawMessage
	for {
		resp, err := e.client.Do(nil, "GET", path, q, nil, "", apiclient.Options{})
		if err != nil {
			return &exitError{code: 1, message: strings.TrimPrefix(err.Error(), "unavailable: "), hint: "check that pursuit serve is running and PURSUIT_SERVER is correct"}
		}
		if resp.Status >= 400 {
			return &exitError{code: exitStatus(resp.Status), raw: resp.Body}
		}
		var pg struct {
			Data []json.RawMessage `json:"data"`
			Page *struct {
				HasMore    bool   `json:"has_more"`
				NextCursor string `json:"next_cursor"`
			} `json:"page"`
		}
		if err := json.Unmarshal(resp.Body, &pg); err != nil || pg.Page == nil {
			return e.print(resp.Body, resp.ContentType)
		}
		all = append(all, pg.Data...)
		if !pg.Page.HasMore || pg.Page.NextCursor == "" {
			break
		}
		q.Set("cursor", pg.Page.NextCursor)
	}
	if all == nil {
		all = []json.RawMessage{}
	}
	b, _ := json.Marshal(map[string]any{"data": all, "page": map[string]any{"has_more": false, "next_cursor": "", "total": len(all)}})
	return e.print(b, "application/json")
}

func exitStatus(code int) int {
	switch code {
	case 400:
		return 6
	case 401, 403:
		return 5
	case 404:
		return 3
	case 409:
		return 4
	default:
		return 1
	}
}

func (e *exec) print(body []byte, contentType string) error {
	if e.json {
		if strings.Contains(contentType, "text/csv") {
			b, _ := json.Marshal(map[string]string{"format": "simplify-csv", "csv": string(body)})
			fmt.Fprintf(e.stdout, "%s\n", b)
			return nil
		}
		body = bytes.TrimSpace(body)
		if len(body) == 0 {
			body = []byte("null")
		}
		fmt.Fprintf(e.stdout, "%s\n", body)
		return nil
	}
	if strings.Contains(contentType, "text/csv") {
		_, err := e.stdout.Write(body)
		return err
	}
	return e.printText(body)
}

func (e *exec) printText(body []byte) error {
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		_, err := e.stdout.Write(body)
		return err
	}
	if m, ok := v.(map[string]any); ok {
		if data, ok := m["data"].([]any); ok {
			return printTable(e.stdout, data)
		}
	}
	return printObject(e.stdout, v, "")
}

func printTable(w io.Writer, rows []any) error {
	if len(rows) == 0 {
		fmt.Fprintln(w, "(empty)")
		return nil
	}
	first, ok := rows[0].(map[string]any)
	if !ok {
		enc := json.NewEncoder(w)
		enc.SetEscapeHTML(false)
		return enc.Encode(rows)
	}
	cols := make([]string, 0, len(first))
	for k := range first {
		cols = append(cols, k)
	}
	sort.Strings(cols)
	tw := tabwriter.NewWriter(w, 0, 2, 2, ' ', 0)
	fmt.Fprintln(tw, strings.Join(cols, "\t"))
	for _, row := range rows {
		m, _ := row.(map[string]any)
		line := make([]string, len(cols))
		for i, c := range cols {
			line[i] = fmt.Sprint(m[c])
		}
		fmt.Fprintln(tw, strings.Join(line, "\t"))
	}
	return tw.Flush()
}

func printObject(w io.Writer, v any, indent string) error {
	m, ok := v.(map[string]any)
	if !ok {
		enc := json.NewEncoder(w)
		enc.SetEscapeHTML(false)
		return enc.Encode(v)
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		val := m[k]
		if child, ok := val.(map[string]any); ok {
			fmt.Fprintf(w, "%s%s:\n", indent, k)
			if err := printObject(w, child, indent+"  "); err != nil {
				return err
			}
			continue
		}
		fmt.Fprintf(w, "%s%s: %s\n", indent, k, fmt.Sprint(val))
	}
	return nil
}

func (e *exec) jsonBody(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimRight(buf.Bytes(), "\n"), nil
}

func (e *exec) versionOf(path string) (int, error) {
	resp, err := e.client.Do(nil, "GET", path, nil, nil, "", apiclient.Options{})
	if err != nil {
		return 0, &exitError{code: 1, message: err.Error(), hint: "check that pursuit serve is running and PURSUIT_SERVER is correct"}
	}
	if resp.Status >= 400 {
		return 0, &exitError{code: exitStatus(resp.Status), raw: resp.Body}
	}
	var obj struct {
		Version int `json:"version"`
	}
	if err := json.Unmarshal(resp.Body, &obj); err != nil || obj.Version < 1 {
		return 0, &exitError{code: 1, message: "response did not include version", hint: "pass --version explicitly"}
	}
	return obj.Version, nil
}

func atoiFlag(v string) (int, error) {
	n, err := strconv.Atoi(v)
	if err != nil || n < 1 {
		return 0, &exitError{code: 2, message: "invalid --version", hint: "version is a positive integer"}
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
