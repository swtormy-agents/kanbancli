package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/spf13/cobra"
	"github.com/swtormy-agents/kanbancli/internal/kanban"
)

func (a *app) healthCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "health",
		Short: "Verify KANBANCLI_URL and token (GET /api/users/me)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			c, err := a.client(ctx)
			if err != nil {
				return err
			}
			path := "/api/users/me"
			if a.dryRun {
				return a.emitValue(map[string]any{
					"request": kanban.DryRun(http.MethodGet, c.BaseURL, path, nil, nil),
					"mode":    "read-only",
				})
			}
			raw, _, err := c.Get(ctx, path, nil)
			if err != nil {
				return a.wrapAPI(err)
			}
			var server json.RawMessage = raw
			return a.emitValue(map[string]any{"ok": true, "url": c.BaseURL, "server": server})
		},
	}
}

func (a *app) versionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Server configuration snapshot (GET /api/config)",
		Args:  cobra.NoArgs,
		RunE:  a.simpleGet("/api/config", nil),
	}
}

func (a *app) meCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "me",
		Short: "Current user (GET /api/users/me)",
		Args:  cobra.NoArgs,
		RunE:  a.simpleGet("/api/users/me", nil),
	}
}

func (a *app) simpleGet(path string, query map[string]string) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		c, err := a.client(ctx)
		if err != nil {
			return err
		}
		if a.dryRun {
			return a.emitValue(kanban.DryRun(http.MethodGet, c.BaseURL, path, urlValues(query), nil))
		}
		raw, _, err := c.Get(ctx, path, urlValues(query))
		if err != nil {
			return a.wrapAPI(err)
		}
		return a.emit(raw)
	}
}

func (a *app) authCmd() *cobra.Command {
	auth := &cobra.Command{Use: "auth", Short: "Authentication helpers"}
	var showToken bool
	login := &cobra.Command{
		Use:   "login",
		Short: "Obtain an access token (POST /api/access-tokens)",
		Long:  "Reads KANBANCLI_EMAIL and KANBANCLI_PASSWORD from the environment only. With --show-token, prints the token alone to stdout; otherwise prints a JSON ok object without the token.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			email := strings.TrimSpace(a.getenv("KANBANCLI_EMAIL"))
			password := strings.TrimSpace(a.getenv("KANBANCLI_PASSWORD"))
			if email == "" {
				return configErr(fmt.Errorf("KANBANCLI_EMAIL is not set"))
			}
			if password == "" {
				return configErr(fmt.Errorf("KANBANCLI_PASSWORD is not set"))
			}
			ctx := cmd.Context()
			c, err := a.clientWithoutAuth(ctx)
			if err != nil {
				return err
			}
			c = c.WithSecrets(password)
			body := map[string]any{
				"emailOrUsername": email,
				"password":        password,
			}
			if a.dryRun {
				return a.emitValue(kanban.DryRunUnauthenticated(http.MethodPost, c.BaseURL, "/api/access-tokens", body))
			}
			raw, _, err := c.JSON(ctx, http.MethodPost, "/api/access-tokens", body, false)
			if err != nil {
				return a.wrapAPI(redactClientErr(c, err))
			}
			tok, err := kanban.ExtractAccessToken(raw)
			if err != nil {
				return runtimeErr(fmt.Errorf("%s", c.Redact(err.Error())))
			}
			if showToken {
				_, err = fmt.Fprintln(a.out, tok)
				return err
			}
			return a.emitValue(map[string]any{"ok": true})
		},
	}
	login.Flags().BoolVar(&showToken, "show-token", false, "print the access token only to stdout (never logged)")
	auth.AddCommand(login)
	return auth
}

func redactClientErr(c *kanban.Client, err error) error {
	var api *kanban.APIError
	if errors.As(err, &api) {
		return &kanban.APIError{Status: api.Status, Message: c.Redact(api.Message), Body: api.Body}
	}
	return fmt.Errorf("%s", c.Redact(err.Error()))
}

func (a *app) projectsCmd() *cobra.Command {
	projects := &cobra.Command{Use: "projects", Short: "Projects"}
	list := &cobra.Command{
		Use:   "list",
		Short: "List projects (GET /api/projects)",
		Args:  cobra.NoArgs,
		RunE:  a.simpleGet("/api/projects", nil),
	}
	projects.AddCommand(list)
	return projects
}

func (a *app) projectCmd() *cobra.Command {
	var projectID string
	get := &cobra.Command{
		Use:   "get",
		Short: "Get a project (GET /api/projects/{id})",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parsePositiveID("--id", projectID)
			if err != nil {
				return err
			}
			path, err := projectPathFromID(id, "")
			if err != nil {
				return err
			}
			return a.simpleGet(path, nil)(cmd, args)
		},
	}
	get.Flags().StringVar(&projectID, "id", "", "project id (required)")
	_ = get.MarkFlagRequired("id")
	project := &cobra.Command{Use: "project", Short: "Project"}
	project.AddCommand(get)
	return project
}

func (a *app) boardsCmd() *cobra.Command {
	var boardID string
	get := &cobra.Command{
		Use:   "get",
		Short: "Get a board with lists and cards (GET /api/boards/{id})",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parsePositiveID("--id", boardID)
			if err != nil {
				return err
			}
			path, err := boardPathFromID(id, "")
			if err != nil {
				return err
			}
			return a.simpleGet(path, nil)(cmd, args)
		},
	}
	get.Flags().StringVar(&boardID, "id", "", "board id (required)")
	_ = get.MarkFlagRequired("id")
	boards := &cobra.Command{Use: "boards", Short: "Boards"}
	boards.AddCommand(get)
	return boards
}

func (a *app) listsCmd() *cobra.Command {
	var boardID string
	list := &cobra.Command{
		Use:   "list",
		Short: "List columns on a board (from GET /api/boards/{id})",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parsePositiveID("--board", boardID)
			if err != nil {
				return err
			}
			path, err := boardPathFromID(id, "")
			if err != nil {
				return err
			}
			ctx := cmd.Context()
			c, err := a.client(ctx)
			if err != nil {
				return err
			}
			if a.dryRun {
				return a.emitValue(map[string]any{
					"request": kanban.DryRun(http.MethodGet, c.BaseURL, path, nil, nil),
					"extract": "lists",
				})
			}
			raw, _, err := c.Get(ctx, path, nil)
			if err != nil {
				return a.wrapAPI(err)
			}
			lists, err := kanban.ExtractBoardLists(raw)
			if err != nil {
				return runtimeErr(err)
			}
			return a.emit(lists)
		},
	}
	list.Flags().StringVar(&boardID, "board", "", "board id (required)")
	_ = list.MarkFlagRequired("board")
	lists := &cobra.Command{Use: "lists", Short: "Board lists (columns)"}
	lists.AddCommand(list)
	return lists
}

func (a *app) cardsCmd() *cobra.Command {
	cards := &cobra.Command{Use: "cards", Short: "Cards on a board"}
	var boardID string
	list := &cobra.Command{
		Use:   "list",
		Short: "List cards on a board (from GET /api/boards/{id})",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parsePositiveID("--board", boardID)
			if err != nil {
				return err
			}
			path, err := boardPathFromID(id, "")
			if err != nil {
				return err
			}
			ctx := cmd.Context()
			c, err := a.client(ctx)
			if err != nil {
				return err
			}
			if a.dryRun {
				return a.emitValue(map[string]any{
					"request": kanban.DryRun(http.MethodGet, c.BaseURL, path, nil, nil),
					"extract": "cards",
				})
			}
			raw, _, err := c.Get(ctx, path, nil)
			if err != nil {
				return a.wrapAPI(err)
			}
			out, err := kanban.ExtractBoardCards(raw)
			if err != nil {
				return runtimeErr(err)
			}
			return a.emit(out)
		},
	}
	list.Flags().StringVar(&boardID, "board", "", "board id (required)")
	_ = list.MarkFlagRequired("board")
	cards.AddCommand(list)
	return cards
}

func (a *app) cardCmd() *cobra.Command {
	card := &cobra.Command{Use: "card", Short: "Card"}
	var writeFlag bool

	getID := ""
	get := &cobra.Command{
		Use:   "get",
		Short: "Get a card (GET /api/cards/{id})",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parsePositiveID("--id", getID)
			if err != nil {
				return err
			}
			path, err := cardPathFromID(id, "")
			if err != nil {
				return err
			}
			return a.simpleGet(path, nil)(cmd, args)
		},
	}
	get.Flags().StringVar(&getID, "id", "", "card id (required)")
	_ = get.MarkFlagRequired("id")

	create := a.cardCreateCmd(&writeFlag)
	move := a.cardMoveCmd(&writeFlag)
	update := a.cardUpdateCmd(&writeFlag)

	card.AddCommand(get, create, move, update)
	return card
}

func (a *app) cardCreateCmd(writeFlag *bool) *cobra.Command {
	var listID, name, description string
	var position string
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a card (POST /api/lists/{list}/cards)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := a.requireHumanWrite(cmd, "card create"); err != nil {
				return err
			}
			lid, err := parsePositiveID("--list", listID)
			if err != nil {
				return err
			}
			if strings.TrimSpace(name) == "" {
				return usageErr("--name is required")
			}
			path, err := listCardsPath(lid)
			if err != nil {
				return err
			}
			body := map[string]any{"name": name}
			if description != "" {
				body["description"] = description
			}
			if position != "" {
				pos, err := parsePositiveID("--position", position)
				if err != nil {
					return err
				}
				body["position"] = pos
			}
			return a.jsonWrite(cmd, http.MethodPost, path, body)
		},
	}
	cmd.Flags().StringVar(&listID, "list", "", "list id (required)")
	cmd.Flags().StringVar(&name, "name", "", "card title (required)")
	cmd.Flags().StringVar(&description, "description", "", "card description")
	cmd.Flags().StringVar(&position, "position", "", "position within the list")
	bindWriteFlag(cmd, writeFlag)
	_ = cmd.MarkFlagRequired("list")
	_ = cmd.MarkFlagRequired("name")
	return cmd
}

func (a *app) cardMoveCmd(writeFlag *bool) *cobra.Command {
	var cardID, listID, position string
	cmd := &cobra.Command{
		Use:   "move",
		Short: "Move a card to another list (PATCH /api/cards/{id})",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := a.requireHumanWrite(cmd, "card move"); err != nil {
				return err
			}
			cid, err := parsePositiveID("--id", cardID)
			if err != nil {
				return err
			}
			lid, err := parsePositiveID("--list", listID)
			if err != nil {
				return err
			}
			path, err := cardPathFromID(cid, "")
			if err != nil {
				return err
			}
			body := map[string]any{"listId": strconvFormat(lid)}
			if position != "" {
				pos, err := parsePositiveID("--position", position)
				if err != nil {
					return err
				}
				body["position"] = pos
			}
			return a.jsonWrite(cmd, http.MethodPatch, path, body)
		},
	}
	cmd.Flags().StringVar(&cardID, "id", "", "card id (required)")
	cmd.Flags().StringVar(&listID, "list", "", "destination list id (required)")
	cmd.Flags().StringVar(&position, "position", "", "position within the list")
	bindWriteFlag(cmd, writeFlag)
	_ = cmd.MarkFlagRequired("id")
	_ = cmd.MarkFlagRequired("list")
	return cmd
}

func (a *app) cardUpdateCmd(writeFlag *bool) *cobra.Command {
	var cardID, name, description, listID, position string
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update a card (PATCH /api/cards/{id})",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := a.requireHumanWrite(cmd, "card update"); err != nil {
				return err
			}
			cid, err := parsePositiveID("--id", cardID)
			if err != nil {
				return err
			}
			path, err := cardPathFromID(cid, "")
			if err != nil {
				return err
			}
			body := map[string]any{}
			if name != "" {
				body["name"] = name
			}
			if description != "" {
				body["description"] = description
			}
			if listID != "" {
				lid, err := parsePositiveID("--list", listID)
				if err != nil {
					return err
				}
				body["listId"] = strconvFormat(lid)
			}
			if position != "" {
				pos, err := parsePositiveID("--position", position)
				if err != nil {
					return err
				}
				body["position"] = pos
			}
			if len(body) == 0 {
				return usageErr("at least one of --name, --description, --list, --position is required")
			}
			return a.jsonWrite(cmd, http.MethodPatch, path, body)
		},
	}
	cmd.Flags().StringVar(&cardID, "id", "", "card id (required)")
	cmd.Flags().StringVar(&name, "name", "", "card title")
	cmd.Flags().StringVar(&description, "description", "", "card description")
	cmd.Flags().StringVar(&listID, "list", "", "list id")
	cmd.Flags().StringVar(&position, "position", "", "position within the list")
	bindWriteFlag(cmd, writeFlag)
	_ = cmd.MarkFlagRequired("id")
	return cmd
}

func strconvFormat(id uint64) string {
	return fmt.Sprintf("%d", id)
}

func (a *app) jsonWrite(cmd *cobra.Command, method, path string, body map[string]any) error {
	ctx := cmd.Context()
	c, err := a.client(ctx)
	if err != nil {
		return err
	}
	if a.dryRun {
		return a.emitValue(kanban.DryRun(method, c.BaseURL, path, nil, body))
	}
	raw, _, err := c.JSON(ctx, method, path, body, true)
	if err != nil {
		return a.wrapAPI(err)
	}
	return a.emit(raw)
}

func (a *app) commentsCmd() *cobra.Command {
	comments := &cobra.Command{Use: "comments", Short: "Card comments"}
	var cardID string
	list := &cobra.Command{
		Use:   "list",
		Short: "List comments on a card (GET /api/cards/{card}/comments)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cid, err := parsePositiveID("--card", cardID)
			if err != nil {
				return err
			}
			path, err := cardPathFromID(cid, "/comments")
			if err != nil {
				return err
			}
			return a.simpleGet(path, nil)(cmd, args)
		},
	}
	list.Flags().StringVar(&cardID, "card", "", "card id (required)")
	_ = list.MarkFlagRequired("card")
	comments.AddCommand(list)
	return comments
}

func (a *app) commentCmd() *cobra.Command {
	var cardID, text string
	var writeFlag bool
	add := &cobra.Command{
		Use:   "add",
		Short: "Add a comment (POST /api/cards/{card}/comments)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := a.requireHumanWrite(cmd, "comment add"); err != nil {
				return err
			}
			cid, err := parsePositiveID("--card", cardID)
			if err != nil {
				return err
			}
			if strings.TrimSpace(text) == "" {
				return usageErr("--text is required")
			}
			path, err := cardPathFromID(cid, "/comments")
			if err != nil {
				return err
			}
			body := map[string]any{"text": text}
			return a.jsonWrite(cmd, http.MethodPost, path, body)
		},
	}
	add.Flags().StringVar(&cardID, "card", "", "card id (required)")
	add.Flags().StringVar(&text, "text", "", "comment text (required)")
	bindWriteFlag(add, &writeFlag)
	_ = add.MarkFlagRequired("card")
	_ = add.MarkFlagRequired("text")
	comment := &cobra.Command{Use: "comment", Short: "Card comment"}
	comment.AddCommand(add)
	return comment
}

func (a *app) usersCmd() *cobra.Command {
	users := &cobra.Command{Use: "users", Short: "Users (metadata only)"}
	list := &cobra.Command{
		Use:   "list",
		Short: "List users (GET /api/users; id, username, name only)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			c, err := a.client(ctx)
			if err != nil {
				return err
			}
			path := "/api/users"
			if a.dryRun {
				return a.emitValue(kanban.DryRun(http.MethodGet, c.BaseURL, path, nil, nil))
			}
			raw, _, err := c.Get(ctx, path, nil)
			if err != nil {
				return a.wrapAPI(err)
			}
			safe, err := kanban.SanitizeUsers(raw)
			if err != nil {
				return runtimeErr(err)
			}
			return a.emit(safe)
		},
	}
	users.AddCommand(list)
	return users
}

func (a *app) apiCmd() *cobra.Command {
	var bodyJSON string
	var writeFlag bool
	var queryPairs []string
	cmd := &cobra.Command{
		Use:   "api <METHOD> <path>",
		Short: "Call a REST path under /api/ (escape hatch)",
		Long:  "Paths must start with /api/. Query parameters use --query key=value. Mutating methods need --write and interactive confirmation.",
		Example: `  kanbancli api GET /api/projects
  kanbancli api GET /api/boards/1 --query include=cards`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			method := strings.ToUpper(args[0])
			pathOnly, err := normalizeAPIPath(args[1])
			if err != nil {
				return err
			}
			q, err := parseQueryFlags(queryPairs)
			if err != nil {
				return err
			}
			if err := checkAPIAllowed(method, pathOnly); err != nil {
				return err
			}
			if apiNeedsWrite(method) {
				if err := a.requireHumanWrite(cmd, "api "+method+" "+pathOnly); err != nil {
					return err
				}
			}
			ctx := cmd.Context()
			c, err := a.client(ctx)
			if err != nil {
				return err
			}
			if a.dryRun {
				var body any
				if bodyJSON != "" {
					_ = json.Unmarshal([]byte(bodyJSON), &body)
				}
				return a.emitValue(kanban.DryRun(method, c.BaseURL, pathOnly, q, body))
			}
			var raw json.RawMessage
			if bodyJSON != "" {
				var body any
				if err := json.Unmarshal([]byte(bodyJSON), &body); err != nil {
					return usageErr("--body: %v", err)
				}
				raw, _, err = c.JSON(ctx, method, pathOnly, body, true)
			} else {
				raw, _, err = c.Do(ctx, method, pathOnly, q, nil, true)
			}
			if err != nil {
				return a.wrapAPI(err)
			}
			return a.emit(raw)
		},
	}
	cmd.Flags().StringVar(&bodyJSON, "body", "", "JSON body for POST/PUT/PATCH")
	cmd.Flags().StringArrayVar(&queryPairs, "query", nil, "query parameter (repeatable, key=value)")
	bindWriteFlag(cmd, &writeFlag)
	return cmd
}

func checkAPIAllowed(method, pathOnly string) error {
	if strings.Contains(strings.ToLower(pathOnly), ".pe.") {
		return blockedErr(fmt.Errorf("api %s %s is blocked; enterprise-only paths are out of scope", method, pathOnly))
	}
	return nil
}

func apiNeedsWrite(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return false
	default:
		return true
	}
}
