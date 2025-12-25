package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
	"github.com/steveyegge/gastown/internal/style"
	"github.com/steveyegge/gastown/internal/wisp"
)

var hookCmd = &cobra.Command{
	Use:     "hook <bead-id>",
	GroupID: GroupWork,
	Short:   "Attach work to your hook (durable across restarts)",
	Long: `Attach a bead (issue) to your hook for durable work tracking.

The hook is the "durability primitive" - work on your hook survives session
restarts, context compaction, and handoffs. When you restart (via gt handoff),
your SessionStart hook finds the attached work and you continue from where
you left off.

This is "assign without action" - use gt sling to also start immediately,
or gt handoff to hook and restart with fresh context.

Examples:
  gt hook gt-abc                    # Attach issue gt-abc to your hook
  gt hook gt-abc -s "Fix the bug"   # With subject for handoff mail
  gt hook gt-abc -m "Check tests"   # With context message

Related commands:
  gt mol status      # See what's on your hook
  gt sling <bead>    # Hook + start now (keep context)
  gt handoff <bead>  # Hook + restart (fresh context)
  gt nudge <agent>   # Send message to trigger execution`,
	Args: cobra.ExactArgs(1),
	RunE: runHook,
}

var (
	hookSubject string
	hookMessage string
	hookDryRun  bool
)

func init() {
	hookCmd.Flags().StringVarP(&hookSubject, "subject", "s", "", "Subject for handoff mail (optional)")
	hookCmd.Flags().StringVarP(&hookMessage, "message", "m", "", "Message for handoff mail (optional)")
	hookCmd.Flags().BoolVarP(&hookDryRun, "dry-run", "n", false, "Show what would be done")
	rootCmd.AddCommand(hookCmd)
}

func runHook(cmd *cobra.Command, args []string) error {
	beadID := args[0]

	// Polecats cannot hook - they use gt done for lifecycle
	if polecatName := os.Getenv("GT_POLECAT"); polecatName != "" {
		return fmt.Errorf("polecats cannot hook work (use gt done for handoff)")
	}

	// Verify the bead exists
	if err := verifyBeadExists(beadID); err != nil {
		return err
	}

	// Determine agent identity
	agentID, err := detectAgentIdentity()
	if err != nil {
		return fmt.Errorf("detecting agent identity: %w", err)
	}

	// Get cwd for wisp storage (use clone root, not town root)
	cloneRoot, err := detectCloneRoot()
	if err != nil {
		return fmt.Errorf("detecting clone root: %w", err)
	}

	// Create the slung work wisp
	sw := wisp.NewSlungWork(beadID, agentID)
	sw.Subject = hookSubject
	sw.Context = hookMessage

	fmt.Printf("%s Hooking %s...\n", style.Bold.Render("🪝"), beadID)

	if hookDryRun {
		fmt.Printf("Would create wisp: %s\n", wisp.HookPath(cloneRoot, agentID))
		fmt.Printf("  bead_id: %s\n", beadID)
		fmt.Printf("  agent: %s\n", agentID)
		if hookSubject != "" {
			fmt.Printf("  subject: %s\n", hookSubject)
		}
		if hookMessage != "" {
			fmt.Printf("  context: %s\n", hookMessage)
		}
		return nil
	}

	// Write the wisp to the hook
	if err := wisp.WriteSlungWork(cloneRoot, agentID, sw); err != nil {
		return fmt.Errorf("writing wisp: %w", err)
	}

	fmt.Printf("%s Work attached to hook\n", style.Bold.Render("✓"))
	fmt.Printf("  Use 'gt handoff' to restart with this work\n")
	fmt.Printf("  Use 'gt mol status' to see hook status\n")

	return nil
}

// verifyBeadExists checks that the bead exists using bd show.
// Defined in sling.go but duplicated here for clarity. Will be consolidated
// when sling.go is removed.
func verifyBeadExistsForHook(beadID string) error {
	cmd := exec.Command("bd", "show", beadID, "--json")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("bead '%s' not found (bd show failed)", beadID)
	}
	return nil
}

// detectAgentIdentityForHook figures out who we are (crew/joe, witness, etc).
// Duplicated from sling.go - will be consolidated when sling.go is removed.
func detectAgentIdentityForHook() (string, error) {
	// Check environment first
	if crew := os.Getenv("GT_CREW"); crew != "" {
		if rig := os.Getenv("GT_RIG"); rig != "" {
			return fmt.Sprintf("%s/crew/%s", rig, crew), nil
		}
	}

	// Check if we're a polecat
	if polecat := os.Getenv("GT_POLECAT"); polecat != "" {
		if rig := os.Getenv("GT_RIG"); rig != "" {
			return fmt.Sprintf("%s/polecats/%s", rig, polecat), nil
		}
	}

	// Try to detect from cwd
	detected, err := detectCrewFromCwd()
	if err == nil {
		return fmt.Sprintf("%s/crew/%s", detected.rigName, detected.crewName), nil
	}

	// Check for other role markers in session name
	if session := os.Getenv("TMUX"); session != "" {
		sessionName, err := getCurrentTmuxSession()
		if err == nil {
			if sessionName == "gt-mayor" {
				return "mayor", nil
			}
			if sessionName == "gt-deacon" {
				return "deacon", nil
			}
			if strings.HasSuffix(sessionName, "-witness") {
				rig := strings.TrimSuffix(strings.TrimPrefix(sessionName, "gt-"), "-witness")
				return fmt.Sprintf("%s/witness", rig), nil
			}
			if strings.HasSuffix(sessionName, "-refinery") {
				rig := strings.TrimSuffix(strings.TrimPrefix(sessionName, "gt-"), "-refinery")
				return fmt.Sprintf("%s/refinery", rig), nil
			}
		}
	}

	return "", fmt.Errorf("cannot determine agent identity - set GT_RIG/GT_CREW or run from clone directory")
}

// detectCloneRootForHook finds the root of the current git clone.
// Duplicated from sling.go - will be consolidated when sling.go is removed.
func detectCloneRootForHook() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("not in a git repository")
	}
	return strings.TrimSpace(string(out)), nil
}
