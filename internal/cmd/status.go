package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/steveyegge/gastown/internal/beads"
	"github.com/steveyegge/gastown/internal/config"
	"github.com/steveyegge/gastown/internal/constants"
	"github.com/steveyegge/gastown/internal/crew"
	"github.com/steveyegge/gastown/internal/git"
	"github.com/steveyegge/gastown/internal/rig"
	"github.com/steveyegge/gastown/internal/style"
	"github.com/steveyegge/gastown/internal/workspace"
)

var statusJSON bool

var statusCmd = &cobra.Command{
	Use:     "status",
	Aliases: []string{"stat"},
	GroupID: GroupDiag,
	Short:   "Show overall town status",
	Long: `Display the current status of the Gas Town workspace.

Shows town name, registered rigs, active polecats, and witness status.`,
	RunE: runStatus,
}

func init() {
	statusCmd.Flags().BoolVar(&statusJSON, "json", false, "Output as JSON")
	rootCmd.AddCommand(statusCmd)
}

// TownStatus represents the overall status of the workspace.
type TownStatus struct {
	Name     string      `json:"name"`
	Location string      `json:"location"`
	Rigs     []RigStatus `json:"rigs"`
	Summary  StatusSum   `json:"summary"`
}

// RigStatus represents status of a single rig.
type RigStatus struct {
	Name         string     `json:"name"`
	Polecats     []string   `json:"polecats"`
	PolecatCount int        `json:"polecat_count"`
	Crews        []string   `json:"crews"`
	CrewCount    int        `json:"crew_count"`
	HasWitness   bool       `json:"has_witness"`
	HasRefinery  bool       `json:"has_refinery"`
	Hooks        []AgentHookInfo `json:"hooks,omitempty"`
}

// AgentHookInfo represents an agent's hook (pinned work) status.
type AgentHookInfo struct {
	Agent    string `json:"agent"`              // Agent address (e.g., "gastown/toast", "gastown/witness")
	Role     string `json:"role"`               // Role type (polecat, crew, witness, refinery)
	HasWork  bool   `json:"has_work"`           // Whether agent has pinned work
	Molecule string `json:"molecule,omitempty"` // Attached molecule ID
	Title    string `json:"title,omitempty"`    // Pinned bead title
}

// StatusSum provides summary counts.
type StatusSum struct {
	RigCount      int `json:"rig_count"`
	PolecatCount  int `json:"polecat_count"`
	CrewCount     int `json:"crew_count"`
	WitnessCount  int `json:"witness_count"`
	RefineryCount int `json:"refinery_count"`
	ActiveHooks   int `json:"active_hooks"`
}

func runStatus(cmd *cobra.Command, args []string) error {
	// Find town root
	townRoot, err := workspace.FindFromCwdOrError()
	if err != nil {
		return fmt.Errorf("not in a Gas Town workspace: %w", err)
	}

	// Load town config
	townConfigPath := constants.MayorTownPath(townRoot)
	townConfig, err := config.LoadTownConfig(townConfigPath)
	if err != nil {
		// Try to continue without config
		townConfig = &config.TownConfig{Name: filepath.Base(townRoot)}
	}

	// Load rigs config
	rigsConfigPath := constants.MayorRigsPath(townRoot)
	rigsConfig, err := config.LoadRigsConfig(rigsConfigPath)
	if err != nil {
		// Empty config if file doesn't exist
		rigsConfig = &config.RigsConfig{Rigs: make(map[string]config.RigEntry)}
	}

	// Create rig manager
	g := git.NewGit(townRoot)
	mgr := rig.NewManager(townRoot, rigsConfig, g)

	// Discover rigs
	rigs, err := mgr.DiscoverRigs()
	if err != nil {
		return fmt.Errorf("discovering rigs: %w", err)
	}

	// Build status
	status := TownStatus{
		Name:     townConfig.Name,
		Location: townRoot,
		Rigs:     make([]RigStatus, 0, len(rigs)),
	}

	for _, r := range rigs {
		rs := RigStatus{
			Name:         r.Name,
			Polecats:     r.Polecats,
			PolecatCount: len(r.Polecats),
			HasWitness:   r.HasWitness,
			HasRefinery:  r.HasRefinery,
		}

		// Count crew workers
		crewGit := git.NewGit(r.Path)
		crewMgr := crew.NewManager(r, crewGit)
		if workers, err := crewMgr.List(); err == nil {
			for _, w := range workers {
				rs.Crews = append(rs.Crews, w.Name)
			}
			rs.CrewCount = len(workers)
		}

		// Discover hooks for all agents in this rig
		rs.Hooks = discoverRigHooks(r, rs.Crews)
		for _, hook := range rs.Hooks {
			if hook.HasWork {
				status.Summary.ActiveHooks++
			}
		}

		status.Rigs = append(status.Rigs, rs)

		// Update summary
		status.Summary.PolecatCount += len(r.Polecats)
		status.Summary.CrewCount += rs.CrewCount
		if r.HasWitness {
			status.Summary.WitnessCount++
		}
		if r.HasRefinery {
			status.Summary.RefineryCount++
		}
	}
	status.Summary.RigCount = len(rigs)

	// Output
	if statusJSON {
		return outputStatusJSON(status)
	}
	return outputStatusText(status)
}

func outputStatusJSON(status TownStatus) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(status)
}

func outputStatusText(status TownStatus) error {
	// Header
	fmt.Printf("%s %s\n", style.Bold.Render("⚙️  Gas Town:"), status.Name)
	fmt.Printf("   Location: %s\n\n", style.Dim.Render(status.Location))

	// Summary
	fmt.Printf("%s\n", style.Bold.Render("Summary"))
	fmt.Printf("   Rigs:      %d\n", status.Summary.RigCount)
	fmt.Printf("   Polecats:  %d\n", status.Summary.PolecatCount)
	fmt.Printf("   Crews:     %d\n", status.Summary.CrewCount)
	fmt.Printf("   Witnesses: %d\n", status.Summary.WitnessCount)
	fmt.Printf("   Refineries: %d\n", status.Summary.RefineryCount)
	fmt.Printf("   Active Hooks: %d\n", status.Summary.ActiveHooks)

	if len(status.Rigs) == 0 {
		fmt.Printf("\n%s\n", style.Dim.Render("No rigs registered. Use 'gt rig add' to add one."))
		return nil
	}

	// Rigs detail
	fmt.Printf("\n%s\n", style.Bold.Render("Rigs"))
	for _, r := range status.Rigs {
		// Rig name with indicators
		indicators := ""
		if r.HasWitness {
			indicators += " " + AgentTypeIcons[AgentWitness]
		}
		if r.HasRefinery {
			indicators += " " + AgentTypeIcons[AgentRefinery]
		}
		if r.CrewCount > 0 {
			indicators += " " + AgentTypeIcons[AgentCrew]
		}

		fmt.Printf("   %s%s\n", style.Bold.Render(r.Name), indicators)

		if len(r.Polecats) > 0 {
			fmt.Printf("      Polecats: %v\n", r.Polecats)
		} else {
			fmt.Printf("      %s\n", style.Dim.Render("No polecats"))
		}

		if len(r.Crews) > 0 {
			fmt.Printf("      Crews: %v\n", r.Crews)
		}

		// Show active hooks
		activeHooks := []AgentHookInfo{}
		for _, h := range r.Hooks {
			if h.HasWork {
				activeHooks = append(activeHooks, h)
			}
		}
		if len(activeHooks) > 0 {
			fmt.Printf("      %s\n", style.Bold.Render("Hooks:"))
			for _, h := range activeHooks {
				if h.Molecule != "" {
					fmt.Printf("         %s %s → %s\n", AgentTypeIcons[AgentPolecat], h.Agent, h.Molecule)
				} else if h.Title != "" {
					fmt.Printf("         %s %s → %s\n", AgentTypeIcons[AgentPolecat], h.Agent, h.Title)
				} else {
					fmt.Printf("         %s %s → (work attached)\n", AgentTypeIcons[AgentPolecat], h.Agent)
				}
			}
		}
	}

	return nil
}

// discoverRigHooks finds all hook attachments for agents in a rig.
// It scans polecats, crew workers, witness, and refinery for handoff beads.
func discoverRigHooks(r *rig.Rig, crews []string) []AgentHookInfo {
	var hooks []AgentHookInfo

	// Create beads instance for the rig
	b := beads.New(r.Path)

	// Check polecats
	for _, name := range r.Polecats {
		hook := getAgentHook(b, name, r.Name+"/"+name, "polecat")
		hooks = append(hooks, hook)
	}

	// Check crew workers
	for _, name := range crews {
		hook := getAgentHook(b, name, r.Name+"/crew/"+name, "crew")
		hooks = append(hooks, hook)
	}

	// Check witness
	if r.HasWitness {
		hook := getAgentHook(b, "witness", r.Name+"/witness", "witness")
		hooks = append(hooks, hook)
	}

	// Check refinery
	if r.HasRefinery {
		hook := getAgentHook(b, "refinery", r.Name+"/refinery", "refinery")
		hooks = append(hooks, hook)
	}

	return hooks
}

// getAgentHook retrieves hook status for a specific agent.
func getAgentHook(b *beads.Beads, role, agentAddress, roleType string) AgentHookInfo {
	hook := AgentHookInfo{
		Agent: agentAddress,
		Role:  roleType,
	}

	// Find handoff bead for this role
	handoff, err := b.FindHandoffBead(role)
	if err != nil || handoff == nil {
		return hook
	}

	// Check for attachment
	attachment := beads.ParseAttachmentFields(handoff)
	if attachment != nil && attachment.AttachedMolecule != "" {
		hook.HasWork = true
		hook.Molecule = attachment.AttachedMolecule
		hook.Title = handoff.Title
	} else if handoff.Description != "" {
		// Has content but no molecule - still has work
		hook.HasWork = true
		hook.Title = handoff.Title
	}

	return hook
}
