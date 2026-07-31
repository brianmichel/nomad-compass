// Package cli implements Compass's command-line interface.
package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/hashicorp/nomad/jobspec2"
	"github.com/spf13/cobra"

	"github.com/brianmichel/nomad-compass/internal/manifest"
	bundleplan "github.com/brianmichel/nomad-compass/internal/plan"
)

// Run executes a Compass CLI command. It returns an error suitable for
// presentation by the process entrypoint; it does not call os.Exit.
func Run(ctx context.Context, args []string, in io.Reader, out, errOut io.Writer) error {
	command := NewCommand(ctx, in, out, errOut)
	command.SetArgs(args)
	return command.ExecuteContext(ctx)
}

// NewCommand constructs the CLI root command. Keeping construction separate
// from process execution makes command behavior easy to test and reuse.
func NewCommand(ctx context.Context, in io.Reader, out, errOut io.Writer) *cobra.Command {
	if ctx == nil {
		ctx = context.Background()
	}
	if in == nil {
		in = strings.NewReader("")
	}
	if out == nil {
		out = io.Discard
	}
	if errOut == nil {
		errOut = io.Discard
	}

	state := &commandState{
		in:     in,
		out:    out,
		errOut: errOut,
		format: "text",
		server: defaultServerURL(),
	}
	root := &cobra.Command{
		Use:           "nomad-compass",
		Short:         "GitOps operations for Nomad Compass",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}
	root.SetOut(out)
	root.SetErr(errOut)
	root.PersistentFlags().StringVar(&state.format, "format", state.format, "output format: text or json")
	root.PersistentFlags().StringVar(&state.server, "server", state.server, "Compass HTTP API URL")
	root.PersistentPreRunE = func(*cobra.Command, []string) error {
		if state.format != "text" && state.format != "json" {
			return fmt.Errorf("unsupported output format %q", state.format)
		}
		return nil
	}

	root.AddCommand(newBundleCommand(state))
	root.AddCommand(newStatusCommand(state))
	root.AddCommand(newRepoCommand(state))
	root.AddCommand(newCredentialCommand(state))
	return root
}

type commandState struct {
	in     io.Reader
	out    io.Writer
	errOut io.Writer
	format string
	server string
}

func newBundleCommand(state *commandState) *cobra.Command {
	bundle := &cobra.Command{
		Use:   "bundle",
		Short: "Validate and inspect Compass bundles",
	}
	validate := &cobra.Command{
		Use:   "validate",
		Short: "Validate a bundle without contacting Nomad",
		Args:  cobra.NoArgs,
	}
	var filePath string
	validate.Flags().StringVar(&filePath, "file", "", "bundle manifest path, or - to read stdin")
	validate.RunE = func(cmd *cobra.Command, _ []string) error {
		if filePath == "" {
			return errors.New("bundle manifest path is required (--file path)")
		}
		return runBundleValidate(cmd.Context(), state, filePath)
	}

	var planFile, againstFile, revision string
	plan := &cobra.Command{
		Use:   "plan",
		Short: "Show an offline change plan for a bundle",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if planFile == "" {
				return errors.New("bundle manifest path is required (--file path)")
			}
			if againstFile == "-" {
				return errors.New("--against cannot read stdin; use a file path")
			}
			return runBundlePlan(cmd.Context(), state, planFile, againstFile, revision)
		},
	}
	plan.Flags().StringVar(&planFile, "file", "", "desired bundle manifest path, or - to read stdin")
	plan.Flags().StringVar(&againstFile, "against", "", "optional previous bundle manifest to compare against")
	plan.Flags().StringVar(&revision, "revision", "working-tree", "revision label to display")

	bundle.AddCommand(validate, plan)
	return bundle
}

func newStatusCommand(state *commandState) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show Compass and Nomad connectivity",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			var status statusResponse
			if err := state.client().get(cmd.Context(), "/api/status", &status); err != nil {
				return err
			}
			return writeValue(state.out, state.format, status, func() error {
				connected := "no"
				if status.NomadConnected {
					connected = "yes"
				}
				_, err := fmt.Fprintf(state.out, "Nomad connected: %s\n", connected)
				if status.NomadMessage != "" {
					_, _ = fmt.Fprintf(state.out, "Nomad message: %s\n", status.NomadMessage)
				}
				return err
			})
		},
	}
}

func newRepoCommand(state *commandState) *cobra.Command {
	repoCommand := &cobra.Command{
		Use:   "repo",
		Short: "Manage Git repositories tracked by Compass",
	}

	list := &cobra.Command{
		Use:   "list",
		Short: "List tracked repositories",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			var repos []repositoryResponse
			if err := state.client().get(cmd.Context(), "/api/repos", &repos); err != nil {
				return err
			}
			return writeValue(state.out, state.format, repos, func() error {
				w := tabwriter.NewWriter(state.out, 0, 4, 2, ' ', 0)
				if _, err := fmt.Fprintln(w, "ID\tNAME\tBRANCH\tURL\tJOBS"); err != nil {
					return err
				}
				for _, repo := range repos {
					if _, err := fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%d\n", repo.ID, repo.Name, repo.Branch, repo.RepoURL, len(repo.Jobs)); err != nil {
						return err
					}
				}
				return w.Flush()
			})
		},
	}

	var name, repoURL, branch, jobPath string
	var credentialID int64
	add := &cobra.Command{
		Use:   "add",
		Short: "Add and initially reconcile a repository",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if strings.TrimSpace(name) == "" || strings.TrimSpace(repoURL) == "" {
				return errors.New("--name and --url are required")
			}
			payload := createRepoRequest{Name: name, RepoURL: repoURL, Branch: branch, JobPath: jobPath, CredentialID: credentialID}
			var repo repositoryResponse
			if err := state.client().post(cmd.Context(), "/api/repos", payload, &repo); err != nil {
				return err
			}
			return writeValue(state.out, state.format, repo, func() error {
				_, err := fmt.Fprintf(state.out, "repository %d created: %s\n", repo.ID, repo.Name)
				return err
			})
		},
	}
	add.Flags().StringVar(&name, "name", "", "repository display name")
	add.Flags().StringVar(&repoURL, "url", "", "Git repository URL")
	add.Flags().StringVar(&branch, "branch", "main", "Git branch")
	add.Flags().StringVar(&jobPath, "job-path", ".nomad", "path in the repository containing jobs or a bundle")
	add.Flags().Int64Var(&credentialID, "credential-id", 0, "credential ID to use for Git authentication")

	var reconcileID int64
	reconcileCommand := &cobra.Command{
		Use:   "reconcile",
		Short: "Trigger reconciliation for one repository",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if reconcileID <= 0 {
				return errors.New("--id must be greater than zero")
			}
			path := "/api/repos/" + strconv.FormatInt(reconcileID, 10) + "/reconcile"
			var response map[string]string
			if err := state.client().post(cmd.Context(), path, nil, &response); err != nil {
				return err
			}
			return writeValue(state.out, state.format, response, func() error {
				_, err := fmt.Fprintf(state.out, "reconciliation requested for repository %d\n", reconcileID)
				return err
			})
		},
	}
	reconcileCommand.Flags().Int64Var(&reconcileID, "id", 0, "repository ID")

	var planID int64
	planCommand := &cobra.Command{
		Use:   "plan",
		Short: "Show a read-only live plan for one repository",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if planID <= 0 {
				return errors.New("--id must be greater than zero")
			}
			var report PlanReport
			path := "/api/repos/" + strconv.FormatInt(planID, 10) + "/plan"
			if err := state.client().get(cmd.Context(), path, &report); err != nil {
				return err
			}
			return writeValue(state.out, state.format, report, func() error {
				return writeTextPlan(state.out, report)
			})
		},
	}
	planCommand.Flags().Int64Var(&planID, "id", 0, "repository ID")

	var deleteID int64
	var unschedule, confirm bool
	deleteCommand := &cobra.Command{
		Use:   "delete",
		Short: "Delete repository metadata",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if deleteID <= 0 {
				return errors.New("--id must be greater than zero")
			}
			if !confirm {
				return errors.New("repository deletion requires --yes")
			}
			path := "/api/repos/" + strconv.FormatInt(deleteID, 10)
			var response map[string]string
			if err := state.client().delete(cmd.Context(), path, deleteRepoRequest{Unschedule: unschedule}, &response); err != nil {
				return err
			}
			return writeValue(state.out, state.format, response, func() error {
				_, err := fmt.Fprintf(state.out, "repository %d deleted\n", deleteID)
				return err
			})
		},
	}
	deleteCommand.Flags().Int64Var(&deleteID, "id", 0, "repository ID")
	deleteCommand.Flags().BoolVar(&unschedule, "unschedule", false, "remove managed Nomad resources")
	deleteCommand.Flags().BoolVar(&confirm, "yes", false, "confirm deletion")

	repoCommand.AddCommand(list, add, reconcileCommand, planCommand, deleteCommand)
	return repoCommand
}

func newCredentialCommand(state *commandState) *cobra.Command {
	credentialCommand := &cobra.Command{
		Use:   "credential",
		Short: "Manage encrypted Git credentials",
	}
	list := &cobra.Command{
		Use:   "list",
		Short: "List credentials without revealing secret data",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			var credentials []credentialResponse
			if err := state.client().get(cmd.Context(), "/api/credentials", &credentials); err != nil {
				return err
			}
			return writeValue(state.out, state.format, credentials, func() error {
				w := tabwriter.NewWriter(state.out, 0, 4, 2, ' ', 0)
				if _, err := fmt.Fprintln(w, "ID\tNAME\tTYPE"); err != nil {
					return err
				}
				for _, credential := range credentials {
					if _, err := fmt.Fprintf(w, "%d\t%s\t%s\n", credential.ID, credential.Name, credential.Type); err != nil {
						return err
					}
				}
				return w.Flush()
			})
		},
	}

	var name, credentialType, token, username, privateKey, passphrase string
	add := &cobra.Command{
		Use:   "add",
		Short: "Create an encrypted Git credential",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if strings.TrimSpace(name) == "" || strings.TrimSpace(credentialType) == "" {
				return errors.New("--name and --type are required")
			}
			if err := validateCredentialInput(credentialType, token, privateKey); err != nil {
				return err
			}
			payload := createCredentialRequest{Name: name, Type: credentialType, Token: token, Username: username, PrivateKey: privateKey, Passphrase: passphrase}
			var credential credentialResponse
			if err := state.client().post(cmd.Context(), "/api/credentials", payload, &credential); err != nil {
				return err
			}
			return writeValue(state.out, state.format, credential, func() error {
				_, err := fmt.Fprintf(state.out, "credential %d created: %s\n", credential.ID, credential.Name)
				return err
			})
		},
	}
	add.Flags().StringVar(&name, "name", "", "credential display name")
	add.Flags().StringVar(&credentialType, "type", "", "credential type: https-token or ssh-key")
	add.Flags().StringVar(&token, "token", "", "HTTPS token")
	add.Flags().StringVar(&username, "username", "", "HTTPS username")
	add.Flags().StringVar(&privateKey, "private-key", "", "SSH private key")
	add.Flags().StringVar(&passphrase, "passphrase", "", "SSH private-key passphrase")

	var deleteID int64
	var deleteRepos, unschedule, confirm bool
	deleteCommand := &cobra.Command{
		Use:   "delete",
		Short: "Delete a credential",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if deleteID <= 0 {
				return errors.New("--id must be greater than zero")
			}
			if !confirm {
				return errors.New("credential deletion requires --yes")
			}
			if unschedule && !deleteRepos {
				return errors.New("--unschedule requires --delete-repos")
			}
			path := "/api/credentials/" + strconv.FormatInt(deleteID, 10)
			payload := deleteCredentialRequest{DeleteRepos: deleteRepos, Unschedule: unschedule}
			var response map[string]string
			if err := state.client().delete(cmd.Context(), path, payload, &response); err != nil {
				return err
			}
			return writeValue(state.out, state.format, response, func() error {
				_, err := fmt.Fprintf(state.out, "credential %d deleted\n", deleteID)
				return err
			})
		},
	}
	deleteCommand.Flags().Int64Var(&deleteID, "id", 0, "credential ID")
	deleteCommand.Flags().BoolVar(&deleteRepos, "delete-repos", false, "delete repositories using this credential")
	deleteCommand.Flags().BoolVar(&unschedule, "unschedule", false, "remove managed Nomad resources")
	deleteCommand.Flags().BoolVar(&confirm, "yes", false, "confirm deletion")

	credentialCommand.AddCommand(list, add, deleteCommand)
	return credentialCommand
}

func runBundleValidate(ctx context.Context, state *commandState, filePath string) error {
	bundle, resources, err := loadValidatedBundle(ctx, state.in, filePath)
	if err != nil {
		return err
	}
	report := ValidationReport{Valid: true, Bundle: bundle.Name, Path: displayPath(filePath), Resources: resources}
	return writeValue(state.out, state.format, report, func() error {
		return writeTextReport(state.out, report)
	})
}

func loadValidatedBundle(ctx context.Context, in io.Reader, filePath string) (*manifest.Bundle, []ResourceReport, error) {
	source, err := readBundle(filePath, in)
	if err != nil {
		return nil, nil, err
	}
	bundle, err := manifest.Parse(source, displayPath(filePath))
	if err != nil {
		return nil, nil, err
	}
	ordered, err := bundle.OrderedResources()
	if err != nil {
		return nil, nil, err
	}
	resources, err := validateResources(ctx, ordered)
	if err != nil {
		return nil, nil, err
	}
	return bundle, resources, nil
}

// Plan types are aliases so the CLI and reconciliation service share one
// planning contract and cannot drift in their action semantics.
type PlanReport = bundleplan.Report
type PlanResource = bundleplan.ResourcePlan
type PlanSummary = bundleplan.Summary

// ValidationReport is the stable machine-readable result of bundle validate.
type ValidationReport struct {
	Valid     bool             `json:"valid"`
	Bundle    string           `json:"bundle"`
	Path      string           `json:"path"`
	Resources []ResourceReport `json:"resources"`
}

// ResourceReport describes a validated resource in dependency order.
type ResourceReport struct {
	Address      string   `json:"address"`
	Kind         string   `json:"kind"`
	DeleteMode   string   `json:"delete_mode"`
	DependsOn    []string `json:"depends_on,omitempty"`
	SpecHash     string   `json:"spec_hash"`
	ManifestHash string   `json:"manifest_hash"`
}

func validateResources(ctx context.Context, resources []manifest.Resource) ([]ResourceReport, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	reports := make([]ResourceReport, 0, len(resources))
	for _, resource := range resources {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		switch resource.Kind {
		case "job":
			source, err := manifest.NativeJobSource(resource)
			if err != nil {
				return nil, err
			}
			if _, err := jobspec2.ParseWithConfig(&jobspec2.ParseConfig{Body: source, Path: resource.SourcePath, Strict: true}); err != nil {
				return nil, fmt.Errorf("validate bundle job %q: %w", resource.Address, err)
			}
		case "volume":
			if _, err := manifest.CompileVolume(resource); err != nil {
				return nil, fmt.Errorf("validate bundle volume %q: %w", resource.Address, err)
			}
		case "acl_policy":
			if _, err := manifest.CompileACLPolicy(resource); err != nil {
				return nil, fmt.Errorf("validate bundle ACL policy %q: %w", resource.Address, err)
			}
		default:
			return nil, fmt.Errorf("bundle resource %q uses kind %q, which the CLI cannot validate yet", resource.Address, resource.Kind)
		}
		reports = append(reports, ResourceReport{Address: resource.Address, Kind: resource.Kind, DeleteMode: string(resource.DeleteMode), DependsOn: append([]string(nil), resource.DependsOn...), SpecHash: manifest.SpecHash(resource), ManifestHash: manifest.ManifestHash(resource)})
	}
	return reports, nil
}

func writeTextReport(out io.Writer, report ValidationReport) error {
	if _, err := fmt.Fprintf(out, "valid bundle %q (%s)\nresources:\n", report.Bundle, report.Path); err != nil {
		return err
	}
	for index, resource := range report.Resources {
		if _, err := fmt.Fprintf(out, "  %d. %s [%s] spec=%s manifest=%s\n", index+1, resource.Address, resource.DeleteMode, resource.SpecHash, resource.ManifestHash); err != nil {
			return err
		}
		if len(resource.DependsOn) > 0 {
			if _, err := fmt.Fprintf(out, "     depends_on: %s\n", strings.Join(resource.DependsOn, ", ")); err != nil {
				return err
			}
		}
	}
	return nil
}

func runBundlePlan(ctx context.Context, state *commandState, desiredPath, againstPath, revision string) error {
	desired, _, err := loadValidatedBundle(ctx, state.in, desiredPath)
	if err != nil {
		return err
	}
	var previous *manifest.Bundle
	if againstPath != "" {
		previous, _, err = loadValidatedBundle(ctx, nil, againstPath)
		if err != nil {
			return fmt.Errorf("load comparison bundle: %w", err)
		}
	}
	result := bundleplan.CompareBundles(desired, previous)
	result.Revision = revision
	result.ComparedTo = againstPath
	return writeValue(state.out, state.format, result, func() error {
		return writeTextPlan(state.out, result)
	})
}

func writeTextPlan(out io.Writer, plan PlanReport) error {
	if _, err := fmt.Fprintf(out, "Bundle: %s\nRevision: %s\n\n", plan.Bundle, plan.Revision); err != nil {
		return err
	}
	for _, resource := range plan.Resources {
		symbol := map[string]string{"create": "+", "update": "~", "delete": "-", "protected": "!", "unchanged": "=", "conflict": "?"}[resource.Action]
		line := fmt.Sprintf("%s %s", symbol, resource.Address)
		if resource.Action == "protected" {
			line += " deletion protected (" + resource.Reason + ")"
		} else if resource.Action == "conflict" {
			line += " ownership conflict: " + resource.Reason
		}
		if _, err := fmt.Fprintln(out, line); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintln(out, "\nPlan:"); err != nil {
		return err
	}
	items := []struct {
		count int
		name  string
	}{
		{plan.Summary.Create, "create"},
		{plan.Summary.Update, "update"},
		{plan.Summary.Delete, "delete"},
		{plan.Summary.Protected, "protected"},
		{plan.Summary.Unchanged, "unchanged"},
		{plan.Summary.Conflict, "conflict"},
	}
	for _, item := range items {
		if item.name == "conflict" && item.count == 0 {
			continue
		}
		if _, err := fmt.Fprintf(out, "  %d %s\n", item.count, item.name); err != nil {
			return err
		}
	}
	return nil
}

func writeValue(out io.Writer, format string, value any, text func() error) error {
	if format == "text" {
		return text()
	}
	encoder := json.NewEncoder(out)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}

func readBundle(path string, in io.Reader) ([]byte, error) {
	if path == "-" {
		contents, err := io.ReadAll(in)
		if err != nil {
			return nil, fmt.Errorf("read bundle from stdin: %w", err)
		}
		return contents, nil
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read bundle %q: %w", path, err)
	}
	return contents, nil
}

func displayPath(path string) string {
	if path == "-" {
		return "<stdin>"
	}
	return path
}

func defaultServerURL() string {
	address := os.Getenv("COMPASS_HTTP_ADDR")
	if address == "" {
		address = ":8080"
	}
	return normalizeServerURL(address)
}

func normalizeServerURL(address string) string {
	address = strings.TrimSpace(address)
	if strings.HasPrefix(address, ":") {
		return "http://127.0.0.1" + address
	}
	if !strings.Contains(address, "://") {
		return "http://" + address
	}
	return strings.TrimRight(address, "/")
}

type serverClient struct {
	baseURL string
	http    *http.Client
}

func (state *commandState) client() *serverClient {
	return &serverClient{baseURL: normalizeServerURL(state.server), http: &http.Client{Timeout: 30 * time.Second}}
}

func (c *serverClient) get(ctx context.Context, path string, result any) error {
	return c.request(ctx, http.MethodGet, path, nil, result)
}

func (c *serverClient) post(ctx context.Context, path string, payload, result any) error {
	return c.request(ctx, http.MethodPost, path, payload, result)
}

func (c *serverClient) delete(ctx context.Context, path string, payload, result any) error {
	return c.request(ctx, http.MethodDelete, path, payload, result)
}

func (c *serverClient) request(ctx context.Context, method, path string, payload, result any) error {
	var body io.Reader
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		body = strings.NewReader(string(encoded))
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return err
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("Compass API request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		var failure struct {
			Error string `json:"error"`
		}
		if decodeErr := json.NewDecoder(resp.Body).Decode(&failure); decodeErr == nil && failure.Error != "" {
			return fmt.Errorf("Compass API: %s", failure.Error)
		}
		return fmt.Errorf("Compass API returned HTTP %s", resp.Status)
	}
	if result == nil {
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(result); err != nil && !errors.Is(err, io.EOF) {
		return fmt.Errorf("decode Compass API response: %w", err)
	}
	return nil
}

type statusResponse struct {
	NomadConnected bool   `json:"nomad_connected"`
	NomadMessage   string `json:"nomad_message,omitempty"`
}

type repositoryResponse struct {
	ID         int64           `json:"id"`
	Name       string          `json:"name"`
	RepoURL    string          `json:"repo_url"`
	Branch     string          `json:"branch"`
	JobPath    string          `json:"job_path"`
	Jobs       []repositoryJob `json:"jobs"`
	LastCommit *string         `json:"last_commit,omitempty"`
}

type repositoryJob struct {
	Path   string `json:"path"`
	JobID  string `json:"job_id,omitempty"`
	Status string `json:"status,omitempty"`
}

type credentialResponse struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	Type      string    `json:"type"`
	CreatedAt time.Time `json:"created_at,omitempty"`
	UpdatedAt time.Time `json:"updated_at,omitempty"`
}

type createRepoRequest struct {
	Name         string `json:"name"`
	RepoURL      string `json:"repo_url"`
	Branch       string `json:"branch"`
	JobPath      string `json:"job_path"`
	CredentialID int64  `json:"credential_id"`
}

type deleteRepoRequest struct {
	Unschedule bool `json:"unschedule"`
}

type createCredentialRequest struct {
	Name       string `json:"name"`
	Type       string `json:"type"`
	Token      string `json:"token"`
	Username   string `json:"username"`
	PrivateKey string `json:"private_key"`
	Passphrase string `json:"passphrase"`
}

type deleteCredentialRequest struct {
	Unschedule  bool `json:"unschedule"`
	DeleteRepos bool `json:"delete_repos"`
}

func validateCredentialInput(credentialType, token, privateKey string) error {
	switch credentialType {
	case "https-token":
		if strings.TrimSpace(token) == "" {
			return errors.New("--token is required for https-token credentials")
		}
	case "ssh-key":
		if strings.TrimSpace(privateKey) == "" {
			return errors.New("--private-key is required for ssh-key credentials")
		}
	default:
		return fmt.Errorf("unsupported credential type %q", credentialType)
	}
	return nil
}
