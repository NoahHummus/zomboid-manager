package bot

import (
	"context"
	"fmt"
	"log"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
)

type handler struct {
	cfg     Config
	allowed map[string]struct{}
}

var pastTense = map[string]string{
	"start":   "started",
	"stop":    "stopped",
	"restart": "restarted",
}

func (h *handler) onInteraction(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if i.Type != discordgo.InteractionApplicationCommand {
		return
	}

	user := i.User
	if i.Member != nil && i.Member.User != nil {
		user = i.Member.User
	}
	if !h.isAllowed(user.ID) {
		respondEphemeral(s, i, "You are not authorized to use this command.")
		return
	}

	// Defer immediately: shelling out to systemctl/journalctl can take a
	// moment longer than Discord's 3s initial-response window allows.
	if err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
	}); err != nil {
		log.Printf("failed to defer response: %v", err)
		return
	}

	switch i.ApplicationCommandData().Name {
	case "start", "stop", "restart":
		h.handleServiceAction(s, i, i.ApplicationCommandData().Name)
	case "logs":
		h.handleLogs(s, i)
	}
}

func (h *handler) isAllowed(userID string) bool {
	_, ok := h.allowed[userID]
	return ok
}

func (h *handler) handleServiceAction(s *discordgo.Session, i *discordgo.InteractionCreate, action string) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "sudo", "systemctl", action, h.cfg.ServiceName)
	output, err := cmd.CombinedOutput()

	var msg string
	if err != nil {
		msg = fmt.Sprintf("Failed to %s **%s**: %v\n```\n%s\n```", action, h.cfg.ServiceName, err, truncate(string(output), 1500))
	} else {
		msg = fmt.Sprintf(":white_check_mark: **%s** %s.", h.cfg.ServiceName, pastTense[action])
	}

	followUp(s, i, msg)
}

func (h *handler) handleLogs(s *discordgo.Session, i *discordgo.InteractionCreate) {
	lines := 50
	for _, opt := range i.ApplicationCommandData().Options {
		if opt.Name == "lines" {
			lines = int(opt.IntValue())
		}
	}
	if lines <= 0 {
		lines = 50
	}
	if lines > 200 {
		lines = 200
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	args := []string{"-u", h.cfg.ServiceName, "-n", strconv.Itoa(lines), "--no-pager"}
	var cmd *exec.Cmd
	if h.cfg.JournalctlSudo {
		cmd = exec.CommandContext(ctx, "sudo", append([]string{"journalctl"}, args...)...)
	} else {
		cmd = exec.CommandContext(ctx, "journalctl", args...)
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		followUp(s, i, fmt.Sprintf("Failed to fetch logs: %v\n```\n%s\n```", err, truncate(string(output), 1500)))
		return
	}

	content := string(output)
	if strings.TrimSpace(content) == "" {
		followUp(s, i, "(no log output)")
		return
	}

	if len(content) <= 1900 {
		followUp(s, i, fmt.Sprintf("```\n%s\n```", content))
		return
	}

	_, err = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
		Content: fmt.Sprintf("Last %d lines from **%s** (too long to inline, see attachment):", lines, h.cfg.ServiceName),
		Files: []*discordgo.File{
			{
				Name:        h.cfg.ServiceName + ".log",
				ContentType: "text/plain",
				Reader:      strings.NewReader(content),
			},
		},
	})
	if err != nil {
		log.Printf("failed to send followup with file: %v", err)
	}
}

func followUp(s *discordgo.Session, i *discordgo.InteractionCreate, content string) {
	if _, err := s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{Content: content}); err != nil {
		log.Printf("failed to send followup: %v", err)
	}
}

func respondEphemeral(s *discordgo.Session, i *discordgo.InteractionCreate, content string) {
	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: content,
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})
	if err != nil {
		log.Printf("failed to send ephemeral response: %v", err)
	}
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "...(truncated)"
}
