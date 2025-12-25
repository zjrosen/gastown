package cmd

import (
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/steveyegge/gastown/internal/beads"
	"github.com/steveyegge/gastown/internal/workspace"
)

// Molecule command flags
var (
	moleculeJSON        bool
	moleculeInstParent  string
	moleculeInstContext []string
	moleculeCatalogOnly bool // List only catalog templates
	moleculeDBOnly      bool // List only database molecules
	moleculeBondParent  string
	moleculeBondRef     string
	moleculeBondVars    []string
)

var moleculeCmd = &cobra.Command{
	Use:     "mol",
	Aliases: []string{"molecule"},
	GroupID: GroupWork,
	Short:   "Molecule workflow commands",
	Long: `Manage molecule workflow templates.

Molecules are composable workflow patterns stored as beads issues.
When instantiated on a parent issue, they create child beads forming a DAG.

LIFECYCLE:
  Proto (template)
       │
       ▼ instantiate/bond
  ┌─────────────────┐
  │ Mol (durable)   │ ← tracked in .beads/
  │ Wisp (ephemeral)│ ← tracked in .beads/ with Wisp=true
  └────────┬────────┘
           │
    ┌──────┴──────┐
    ▼             ▼
  burn         squash
  (no record)  (→ digest)

PHASE TRANSITIONS (for pluggable molecules):
  ┌─────────────┬─────────────┬─────────────┬─────────────────────┐
  │ Phase       │ Parallelism │ Blocks      │ Purpose             │
  ├─────────────┼─────────────┼─────────────┼─────────────────────┤
  │ discovery   │ full        │ (nothing)   │ Inventory, gather   │
  │ structural  │ sequential  │ discovery   │ Big-picture review  │
  │ tactical    │ parallel    │ structural  │ Detailed work       │
  │ synthesis   │ single      │ tactical    │ Aggregate results   │
  └─────────────┴─────────────┴─────────────┴─────────────────────┘

COMMANDS:
  catalog      List available molecule protos
  instantiate  Create steps from a molecule template
  progress     Show execution progress of an instantiated molecule
  status       Show what's on an agent's hook
  burn         Discard molecule without creating a digest
  squash       Complete molecule and create a digest`,
}

var moleculeListCmd = &cobra.Command{
	Use:   "list",
	Short: "List molecules",
	Long: `List all molecule definitions.

By default, lists molecules from all sources:
- Built-in molecules (shipped with gt)
- Town-level: <town>/.beads/molecules.jsonl
- Rig-level: <rig>/.beads/molecules.jsonl
- Project-level: .beads/molecules.jsonl
- Database: molecules stored as issues

Use --catalog to show only template molecules (not instantiated).
Use --db to show only database molecules.`,
	RunE: runMoleculeList,
}

var moleculeExportCmd = &cobra.Command{
	Use:   "export <path>",
	Short: "Export built-in molecules to JSONL",
	Long: `Export built-in molecule templates to a JSONL file.

This creates a molecules.jsonl file containing all built-in molecules.
You can place this in:
- <town>/.beads/molecules.jsonl (town-level)
- <rig>/.beads/molecules.jsonl (rig-level)
- .beads/molecules.jsonl (project-level)

The file can be edited to customize or add new molecules.`,
	Args: cobra.ExactArgs(1),
	RunE: runMoleculeExport,
}

var moleculeShowCmd = &cobra.Command{
	Use:   "show <id>",
	Short: "Show molecule with parsed steps",
	Long: `Show a molecule definition with its parsed steps.

Displays the molecule's title, description structure, and all defined steps
with their dependencies.`,
	Args: cobra.ExactArgs(1),
	RunE: runMoleculeShow,
}

var moleculeParseCmd = &cobra.Command{
	Use:   "parse <id>",
	Short: "Validate and show parsed structure",
	Long: `Parse and validate a molecule definition.

This command parses the molecule's step definitions and reports any errors.
Useful for debugging molecule definitions before instantiation.`,
	Args: cobra.ExactArgs(1),
	RunE: runMoleculeParse,
}

var moleculeInstantiateCmd = &cobra.Command{
	Use:   "instantiate <mol-id>",
	Short: "Create steps from molecule template",
	Long: `Instantiate a molecule on a parent issue.

Creates child issues for each step defined in the molecule, wiring up
dependencies according to the Needs: declarations.

Template variables ({{variable}}) can be substituted using --context flags.

Examples:
  gt molecule instantiate mol-xyz --parent=gt-abc
  gt molecule instantiate mol-xyz --parent=gt-abc --context feature=auth --context file=login.go`,
	Args: cobra.ExactArgs(1),
	RunE: runMoleculeInstantiate,
}

var moleculeInstancesCmd = &cobra.Command{
	Use:   "instances <mol-id>",
	Short: "Show all instantiations of a molecule",
	Long: `Show all parent issues that have instantiated this molecule.

Lists each instantiation with its status and progress.`,
	Args: cobra.ExactArgs(1),
	RunE: runMoleculeInstances,
}

var moleculeProgressCmd = &cobra.Command{
	Use:   "progress <root-issue-id>",
	Short: "Show progress through a molecule's steps",
	Long: `Show the execution progress of an instantiated molecule.

Given a root issue (the parent of molecule steps), displays:
- Total steps and completion status
- Which steps are done, in-progress, ready, or blocked
- Overall progress percentage

This is useful for the Witness to monitor molecule execution.

Example:
  gt molecule progress gt-abc`,
	Args: cobra.ExactArgs(1),
	RunE: runMoleculeProgress,
}

var moleculeAttachCmd = &cobra.Command{
	Use:   "attach <pinned-bead-id> <molecule-id>",
	Short: "Attach a molecule to a pinned bead",
	Long: `Attach a molecule to a pinned/handoff bead.

This records which molecule an agent is currently working on. The attachment
is stored in the pinned bead's description and visible via 'bd show'.

Example:
  gt molecule attach gt-abc mol-xyz`,
	Args: cobra.ExactArgs(2),
	RunE: runMoleculeAttach,
}

var moleculeDetachCmd = &cobra.Command{
	Use:   "detach <pinned-bead-id>",
	Short: "Detach molecule from a pinned bead",
	Long: `Remove molecule attachment from a pinned/handoff bead.

This clears the attached_molecule and attached_at fields from the bead.

Example:
  gt molecule detach gt-abc`,
	Args: cobra.ExactArgs(1),
	RunE: runMoleculeDetach,
}

var moleculeAttachmentCmd = &cobra.Command{
	Use:   "attachment <pinned-bead-id>",
	Short: "Show attachment status of a pinned bead",
	Long: `Show which molecule is attached to a pinned bead.

Example:
  gt molecule attachment gt-abc`,
	Args: cobra.ExactArgs(1),
	RunE: runMoleculeAttachment,
}

var moleculeAttachFromMailCmd = &cobra.Command{
	Use:   "attach-from-mail <mail-id>",
	Short: "Attach a molecule from a mail message",
	Long: `Attach a molecule to the current agent's hook from a mail message.

This command reads a mail message, extracts the molecule ID from the body,
and attaches it to the agent's pinned bead (hook).

The mail body should contain an "attached_molecule:" field with the molecule ID.

Usage: gt mol attach-from-mail <mail-id>

Behavior:
1. Read mail body for attached_molecule field
2. Attach molecule to agent's hook
3. Mark mail as read
4. Return control for execution

Example:
  gt mol attach-from-mail msg-abc123`,
	Args: cobra.ExactArgs(1),
	RunE: runMoleculeAttachFromMail,
}

var moleculeStatusCmd = &cobra.Command{
	Use:   "status [target]",
	Short: "Show what's on an agent's hook",
	Long: `Show what's slung on an agent's hook.

If no target is specified, shows the current agent's status based on
the working directory (polecat, crew member, witness, etc.).

Output includes:
- What's slung (molecule name, associated issue)
- Current phase and progress
- Whether it's a wisp
- Next action hint

Examples:
  gt mol status                    # Show current agent's hook
  gt mol status gastown/nux        # Show specific polecat's hook
  gt mol status gastown/witness    # Show witness's hook`,
	Args: cobra.MaximumNArgs(1),
	RunE: runMoleculeStatus,
}

var moleculeCurrentCmd = &cobra.Command{
	Use:   "current [identity]",
	Short: "Show what agent should be working on",
	Long: `Query what an agent is supposed to be working on via breadcrumb trail.

Looks up the agent's handoff bead, checks for attached molecules, and
identifies the current/next step in the workflow.

If no identity is specified, uses the current agent based on working directory.

Output includes:
- Identity and handoff bead info
- Attached molecule (if any)
- Progress through steps
- Current step that should be worked on next

Examples:
  gt molecule current              # Current agent's work
  gt molecule current gastown/furiosa
  gt molecule current deacon
  gt mol current gastown/witness`,
	Args: cobra.MaximumNArgs(1),
	RunE: runMoleculeCurrent,
}

var moleculeCatalogCmd = &cobra.Command{
	Use:   "catalog",
	Short: "List available molecule protos",
	Long: `List molecule protos available for slinging.

This is a convenience alias for 'gt mol list --catalog' that shows only
reusable templates, not instantiated molecules.

Protos come from:
- Built-in molecules (shipped with gt)
- Town-level: <town>/.beads/molecules.jsonl
- Rig-level: <rig>/.beads/molecules.jsonl
- Project-level: .beads/molecules.jsonl`,
	RunE: runMoleculeCatalog,
}

var moleculeBurnCmd = &cobra.Command{
	Use:   "burn [target]",
	Short: "Burn current molecule without creating a digest",
	Long: `Burn (destroy) the current molecule attachment.

This discards the molecule without creating a permanent record. Use this
when abandoning work or when a molecule doesn't need an audit trail.

If no target is specified, burns the current agent's attached molecule.

For wisps, burning is the default completion action. For regular molecules,
consider using 'squash' instead to preserve an audit trail.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runMoleculeBurn,
}

var moleculeSquashCmd = &cobra.Command{
	Use:   "squash [target]",
	Short: "Compress molecule into a digest",
	Long: `Squash the current molecule into a permanent digest.

This condenses a completed molecule's execution into a compact record.
The digest preserves:
- What molecule was executed
- When it ran
- Summary of results

Use this for patrol cycles and other operational work that should have
a permanent (but compact) record.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runMoleculeSquash,
}

var moleculeBondCmd = &cobra.Command{
	Use:   "bond <proto-id>",
	Short: "Dynamically bond a child molecule to a running parent",
	Long: `Bond a child molecule to a running parent molecule/wisp.

This creates a new child molecule instance under the specified parent,
enabling the Christmas Ornament pattern where a step can dynamically
spawn children for parallel execution.

Examples:
  # Bond a polecat inspection arm to current patrol wisp
  gt mol bond mol-polecat-arm --parent=patrol-x7k --ref=arm-toast \
    --var polecat_name=toast --var rig=gastown

  # The child will have ID: patrol-x7k.arm-toast
  # And template variables {{polecat_name}} and {{rig}} expanded

Usage in mol-witness-patrol's survey-workers step:
  for polecat in $(gt polecat list <rig> --names); do
    gt mol bond mol-polecat-arm --parent=$PATROL_WISP_ID \
      --ref=arm-$polecat \
      --var polecat_name=$polecat \
      --var rig=<rig>
  done`,
	Args: cobra.ExactArgs(1),
	RunE: runMoleculeBond,
}

func init() {
	// List flags
	moleculeListCmd.Flags().BoolVar(&moleculeJSON, "json", false, "Output as JSON")
	moleculeListCmd.Flags().BoolVar(&moleculeCatalogOnly, "catalog", false, "Show only catalog templates")
	moleculeListCmd.Flags().BoolVar(&moleculeDBOnly, "db", false, "Show only database molecules")

	// Show flags
	moleculeShowCmd.Flags().BoolVar(&moleculeJSON, "json", false, "Output as JSON")

	// Parse flags
	moleculeParseCmd.Flags().BoolVar(&moleculeJSON, "json", false, "Output as JSON")

	// Instantiate flags
	moleculeInstantiateCmd.Flags().StringVar(&moleculeInstParent, "parent", "", "Parent issue ID (required)")
	moleculeInstantiateCmd.Flags().StringArrayVar(&moleculeInstContext, "context", nil, "Context variable (key=value)")
	moleculeInstantiateCmd.MarkFlagRequired("parent")

	// Instances flags
	moleculeInstancesCmd.Flags().BoolVar(&moleculeJSON, "json", false, "Output as JSON")

	// Progress flags
	moleculeProgressCmd.Flags().BoolVar(&moleculeJSON, "json", false, "Output as JSON")

	// Attachment flags
	moleculeAttachmentCmd.Flags().BoolVar(&moleculeJSON, "json", false, "Output as JSON")

	// Status flags
	moleculeStatusCmd.Flags().BoolVar(&moleculeJSON, "json", false, "Output as JSON")

	// Current flags
	moleculeCurrentCmd.Flags().BoolVar(&moleculeJSON, "json", false, "Output as JSON")

	// Catalog flags
	moleculeCatalogCmd.Flags().BoolVar(&moleculeJSON, "json", false, "Output as JSON")

	// Burn flags
	moleculeBurnCmd.Flags().BoolVar(&moleculeJSON, "json", false, "Output as JSON")

	// Squash flags
	moleculeSquashCmd.Flags().BoolVar(&moleculeJSON, "json", false, "Output as JSON")

	// Bond flags
	moleculeBondCmd.Flags().StringVar(&moleculeBondParent, "parent", "", "Parent molecule/wisp ID (required)")
	moleculeBondCmd.Flags().StringVar(&moleculeBondRef, "ref", "", "Child reference suffix (e.g., arm-toast)")
	moleculeBondCmd.Flags().StringArrayVar(&moleculeBondVars, "var", nil, "Template variable (key=value)")
	moleculeBondCmd.Flags().BoolVar(&moleculeJSON, "json", false, "Output as JSON")
	moleculeBondCmd.MarkFlagRequired("parent")

	// Add subcommands
	moleculeCmd.AddCommand(moleculeStatusCmd)
	moleculeCmd.AddCommand(moleculeCurrentCmd)
	moleculeCmd.AddCommand(moleculeCatalogCmd)
	moleculeCmd.AddCommand(moleculeBurnCmd)
	moleculeCmd.AddCommand(moleculeSquashCmd)
	moleculeCmd.AddCommand(moleculeListCmd)
	moleculeCmd.AddCommand(moleculeShowCmd)
	moleculeCmd.AddCommand(moleculeParseCmd)
	moleculeCmd.AddCommand(moleculeInstantiateCmd)
	moleculeCmd.AddCommand(moleculeInstancesCmd)
	moleculeCmd.AddCommand(moleculeExportCmd)
	moleculeCmd.AddCommand(moleculeProgressCmd)
	moleculeCmd.AddCommand(moleculeAttachCmd)
	moleculeCmd.AddCommand(moleculeDetachCmd)
	moleculeCmd.AddCommand(moleculeAttachmentCmd)
	moleculeCmd.AddCommand(moleculeAttachFromMailCmd)
	moleculeCmd.AddCommand(moleculeBondCmd)

	rootCmd.AddCommand(moleculeCmd)
}

// loadMoleculeCatalog loads the molecule catalog with hierarchical sources.
func loadMoleculeCatalog(workDir string) (*beads.MoleculeCatalog, error) {
	var townRoot, rigPath, projectPath string

	// Try to find town root
	townRoot, _ = workspace.FindFromCwd()

	// Try to find rig path
	if townRoot != "" {
		rigName, _, err := findCurrentRig(townRoot)
		if err == nil && rigName != "" {
			rigPath = filepath.Join(townRoot, rigName)
		}
	}

	// Project path is the work directory
	projectPath = workDir

	return beads.LoadCatalog(townRoot, rigPath, projectPath)
}
