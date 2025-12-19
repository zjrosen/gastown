package mail

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/steveyegge/gastown/internal/tmux"
)

// Router handles message delivery via beads.
type Router struct {
	workDir string // directory to run bd commands in
	tmux    *tmux.Tmux
}

// NewRouter creates a new mail router.
// workDir should be a directory containing a .beads database.
func NewRouter(workDir string) *Router {
	return &Router{
		workDir: workDir,
		tmux:    tmux.NewTmux(),
	}
}

// Send delivers a message via beads message.
func (r *Router) Send(msg *Message) error {
	// Convert addresses to beads identities
	toIdentity := addressToIdentity(msg.To)
	fromIdentity := addressToIdentity(msg.From)

	// Build command: bd mail send <recipient> -s <subject> -m <body>
	args := []string{"mail", "send", toIdentity,
		"-s", msg.Subject,
		"-m", msg.Body,
	}

	// Add priority flag
	beadsPriority := PriorityToBeads(msg.Priority)
	args = append(args, "--priority", fmt.Sprintf("%d", beadsPriority))

	// Add message type if set
	if msg.Type != "" && msg.Type != TypeNotification {
		args = append(args, "--type", string(msg.Type))
	}

	// Add thread ID if set
	if msg.ThreadID != "" {
		args = append(args, "--thread-id", msg.ThreadID)
	}

	// Add reply-to if set
	if msg.ReplyTo != "" {
		args = append(args, "--reply-to", msg.ReplyTo)
	}

	cmd := exec.Command("bd", args...)
	cmd.Env = append(cmd.Environ(), "BEADS_AGENT_NAME="+fromIdentity)
	cmd.Dir = r.workDir

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		errMsg := strings.TrimSpace(stderr.String())
		if errMsg != "" {
			return errors.New(errMsg)
		}
		return fmt.Errorf("sending message: %w", err)
	}

	// Notify recipient if they have an active session
	r.notifyRecipient(msg)

	return nil
}

// GetMailbox returns a Mailbox for the given address.
func (r *Router) GetMailbox(address string) (*Mailbox, error) {
	return NewMailboxFromAddress(address, r.workDir), nil
}

// notifyRecipient sends a notification to a recipient's tmux session.
// Uses send-keys to echo a visible banner to ensure notification is seen.
// Supports mayor/, rig/polecat, and rig/refinery addresses.
func (r *Router) notifyRecipient(msg *Message) error {
	sessionID := addressToSessionID(msg.To)
	if sessionID == "" {
		return nil // Unable to determine session ID
	}

	// Check if session exists
	hasSession, err := r.tmux.HasSession(sessionID)
	if err != nil || !hasSession {
		return nil // No active session, skip notification
	}

	// Send visible notification banner to the terminal
	return r.tmux.SendNotificationBanner(sessionID, msg.From, msg.Subject)
}

// addressToSessionID converts a mail address to a tmux session ID.
// Returns empty string if address format is not recognized.
func addressToSessionID(address string) string {
	// Mayor address: "mayor/" or "mayor"
	if strings.HasPrefix(address, "mayor") {
		return "gt-mayor"
	}

	// Rig-based address: "rig/target"
	parts := strings.SplitN(address, "/", 2)
	if len(parts) != 2 || parts[1] == "" {
		return ""
	}

	rig := parts[0]
	target := parts[1]

	// Polecat: gt-rig-polecat
	// Refinery: gt-rig-refinery (if refinery has its own session)
	return fmt.Sprintf("gt-%s-%s", rig, target)
}
