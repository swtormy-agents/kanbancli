package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime/debug"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/swtormy-agents/kanbancli/internal/kanban"
)

const (
	exitRuntime = 1
	exitUsage   = 2
	exitAuth    = 3
	exitBlocked = 4
)

type exitError struct {
	code int
	kind string
	err  error
	api  *kanban.APIError
}

func (e *exitError) Error() string { return e.err.Error() }

func usageErr(format string, a ...any) error {
	return &exitError{code: exitUsage, kind: "usage", err: fmt.Errorf(format, a...)}
}

func runtimeErr(err error) error {
	return &exitError{code: exitRuntime, kind: "runtime", err: err}
}

func authErr(err error) error {
	return &exitError{code: exitAuth, kind: "auth", err: err}
}

func configErr(err error) error {
	return &exitError{code: exitUsage, kind: "config", err: err}
}

func blockedErr(err error) error {
	return &exitError{code: exitBlocked, kind: "blocked", err: err}
}

var Version = "dev"

func versionString() string {
	v := Version
	if v != "dev" {
		return v
	}
	if bi, ok := debug.ReadBuildInfo(); ok {
		rev, dirty := "", ""
		for _, s := range bi.Settings {
			switch s.Key {
			case "vcs.revision":
				rev = s.Value
			case "vcs.modified":
				if s.Value == "true" {
					dirty = "+dirty"
				}
			}
		}
		if len(rev) > 7 {
			rev = rev[:7]
		}
		if rev != "" {
			return "dev-" + rev + dirty
		}
	}
	return v
}

func staleHint(err error) string {
	m := err.Error()
	if strings.Contains(m, "unknown flag") || strings.Contains(m, "unknown command") || strings.Contains(m, "unknown shorthand") {
		return fmt.Sprintf(" (this is kanbancli %s; rebuild or `go install github.com/swtormy-agents/kanbancli/cmd/kanbancli@feat/v0.1`)", versionString())
	}
	return ""
}

type app struct {
	in      io.Reader
	out     io.Writer
	errOut  io.Writer
	getenv  func(string) string
	dryRun  bool
	jq      string
	pretty  bool
	output  string
	baseURL string
	timeout time.Duration
	token   string
}

func Execute(args []string, stdin io.Reader, stdout, stderr io.Writer, getenv func(string) string) int {
	a := &app{in: stdin, out: stdout, errOut: stderr, getenv: getenv, output: "json"}
	root := a.rootCmd()
	root.SetArgs(args)
	root.SetOut(stdout)
	root.SetErr(stderr)
	if err := root.Execute(); err != nil {
		if _, ok := err.(*exitError); !ok {
			err = usageErr("%s%s", err, staleHint(err))
		}
		return a.reportError(err)
	}
	return 0
}

func (a *app) reportError(err error) int {
	ee, ok := err.(*exitError)
	if !ok {
		ee = &exitError{code: exitUsage, kind: "usage", err: err}
	}
	detail := map[string]any{"type": ee.kind, "message": a.redact(ee.err.Error())}
	if ee.api != nil {
		detail["status"] = ee.api.Status
		if ee.api.Message != "" {
			detail["message"] = a.redact(ee.api.Message)
		}
	}
	b, _ := marshal(map[string]any{"ok": false, "error": detail})
	fmt.Fprintln(a.errOut, string(b))
	return ee.code
}

func (a *app) redact(s string) string {
	return kanban.StripURLUserinfo((&kanban.Client{Token: a.token}).Redact(s))
}

func (a *app) validateToken() error {
	if len(a.token) < kanban.MinTokenLength {
		return configErr(fmt.Errorf(
			"KANBANCLI_TOKEN must be at least %d characters (shorter values are rejected because they cannot be redacted safely from logs)",
			kanban.MinTokenLength,
		))
	}
	return nil
}

func (a *app) resolveToken() error {
	a.token = strings.TrimSpace(a.getenv("KANBANCLI_TOKEN"))
	if a.token == "" {
		return authErr(errors.New("KANBANCLI_TOKEN is not set; export it in the environment (never pass on the command line)"))
	}
	return a.validateToken()
}

func (a *app) client(ctx context.Context) (*kanban.Client, error) {
	if err := a.resolveToken(); err != nil {
		return nil, err
	}
	base := a.baseURL
	if base == "" {
		base = strings.TrimSpace(a.getenv("KANBANCLI_URL"))
	}
	if base == "" {
		return nil, authErr(errors.New("KANBANCLI_URL is not set (example: https://kanban.example.com)"))
	}
	normalized, err := kanban.ValidateBaseURL(base)
	if err != nil {
		return nil, configErr(err)
	}
	return &kanban.Client{
		BaseURL: normalized,
		Token:   a.token,
		HTTP:    &http.Client{Timeout: a.timeout},
	}, nil
}

func (a *app) clientWithoutAuth(ctx context.Context) (*kanban.Client, error) {
	base := a.baseURL
	if base == "" {
		base = strings.TrimSpace(a.getenv("KANBANCLI_URL"))
	}
	if base == "" {
		return nil, authErr(errors.New("KANBANCLI_URL is not set (example: https://kanban.example.com)"))
	}
	normalized, err := kanban.ValidateBaseURL(base)
	if err != nil {
		return nil, configErr(err)
	}
	return &kanban.Client{
		BaseURL: normalized,
		HTTP:    &http.Client{Timeout: a.timeout},
	}, nil
}

func (a *app) wrapAPI(err error) error {
	var api *kanban.APIError
	if errors.As(err, &api) {
		code, kind := exitRuntime, "api"
		if api.IsAuth() {
			code, kind = exitAuth, "auth"
		}
		return &exitError{code: code, kind: kind, err: api, api: api}
	}
	return runtimeErr(err)
}

func (a *app) rootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "kanbancli",
		Short:         "PLANKA REST API from the shell (unofficial; not affiliated with PLANKA Software GmbH)",
		Long:          rootLong,
		Version:       versionString(),
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.CompletionOptions.DisableDefaultCmd = true
	root.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return usageErr("%s%s", err, staleHint(err))
	})
	pf := root.PersistentFlags()
	pf.BoolVar(&a.dryRun, "dry-run", false, "print the HTTP request instead of sending it")
	pf.StringVar(&a.jq, "jq", "", "filter JSON output with a jq expression")
	pf.BoolVar(&a.pretty, "pretty", false, "indent JSON output")
	pf.StringVar(&a.output, "output", "json", "output format: json or table")
	pf.StringVar(&a.baseURL, "url", "", "Server base URL (default $KANBANCLI_URL)")
	pf.DurationVar(&a.timeout, "http-timeout", 60*time.Second, "HTTP timeout per request")

	root.AddCommand(
		a.healthCmd(),
		a.versionCmd(),
		a.meCmd(),
		a.authCmd(),
		a.projectsCmd(),
		a.projectCmd(),
		a.boardsCmd(),
		a.listsCmd(),
		a.cardsCmd(),
		a.cardCmd(),
		a.commentsCmd(),
		a.commentCmd(),
		a.usersCmd(),
		a.apiCmd(),
		a.skillCmd(),
	)
	return root
}

const rootLong = `kanbancli calls the PLANKA REST API from the shell. JSON on stdout by default; errors on stderr.

Unofficial — not affiliated with or endorsed by PLANKA Software GmbH.

Setup
  export KANBANCLI_URL=https://kanban.example.com
  export KANBANCLI_TOKEN=...              access token or API key (never pass on the command line)

Mutating commands and non-GET api calls need --write, an interactive terminal, and typing "write" when prompted.

Exit codes: 0 ok, 1 runtime/API, 2 usage or config, 3 auth, 4 blocked.`

var isTerminal = func(r ioReader) bool {
	f, ok := r.(*os.File)
	return ok && isTerminalFile(f)
}

type ioReader interface {
	Read(p []byte) (n int, err error)
}
