// Package bot implements a minimal Discord bot for controlling a Project
// Zomboid server that runs as a systemd service on the same host.
package bot

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/gorilla/websocket"
)

// Config holds everything the bot needs to run.
type Config struct {
	Token          string
	GuildID        string
	ServiceName    string
	AllowedUserIDs []string
	JournalctlSudo bool
	IPv4Only       bool
}

var commands = []*discordgo.ApplicationCommand{
	{Name: "start", Description: "Start the Zomboid server"},
	{Name: "stop", Description: "Stop the Zomboid server"},
	{Name: "restart", Description: "Restart the Zomboid server"},
	{
		Name:        "logs",
		Description: "Show recent Zomboid server logs",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionInteger,
				Name:        "lines",
				Description: "Number of log lines to show (default 50, max 200)",
				Required:    false,
			},
		},
	},
}

// Run starts the bot and blocks until it receives an interrupt/terminate signal.
func Run(cfg Config) error {
	h := &handler{cfg: cfg, allowed: toSet(cfg.AllowedUserIDs)}

	sess, err := discordgo.New("Bot " + cfg.Token)
	if err != nil {
		return fmt.Errorf("create session: %w", err)
	}

	if cfg.IPv4Only {
		forceIPv4(sess)
	}

	sess.AddHandler(h.onInteraction)

	if err := sess.Open(); err != nil {
		return fmt.Errorf("open session: %w", err)
	}
	defer sess.Close()

	registered, err := registerCommands(sess, cfg.GuildID)
	if err != nil {
		return fmt.Errorf("register commands: %w", err)
	}

	log.Printf("zomboid-manager is running (service=%s, allowed users=%d). Press Ctrl+C to exit.", cfg.ServiceName, len(cfg.AllowedUserIDs))
	waitForShutdown()

	log.Println("removing commands...")
	for _, c := range registered {
		if err := sess.ApplicationCommandDelete(sess.State.User.ID, cfg.GuildID, c.ID); err != nil {
			log.Printf("failed to delete command %q: %v", c.Name, err)
		}
	}

	return nil
}

// forceIPv4 dials Discord over tcp4 only, for both the REST client and the
// gateway websocket. Some hosts have an IPv6 route that's routed but
// black-holed rather than absent, so a dual-stack dial hangs for the full
// connect timeout on IPv6 before falling back to IPv4 — long enough to blow
// past Discord's ~3s interaction ack window. Discord doesn't need IPv6 for
// anything the bot does, so this has no downside even on healthy hosts.
func forceIPv4(s *discordgo.Session) {
	dialer := &net.Dialer{Timeout: 15 * time.Second}
	dial4 := func(ctx context.Context, network, addr string) (net.Conn, error) {
		return dialer.DialContext(ctx, "tcp4", addr)
	}

	s.Client.Transport = &http.Transport{DialContext: dial4}

	wsDialer := *websocket.DefaultDialer
	wsDialer.NetDialContext = dial4
	s.Dialer = &wsDialer
}

func registerCommands(s *discordgo.Session, guildID string) ([]*discordgo.ApplicationCommand, error) {
	created := make([]*discordgo.ApplicationCommand, 0, len(commands))
	for _, cmd := range commands {
		c, err := s.ApplicationCommandCreate(s.State.User.ID, guildID, cmd)
		if err != nil {
			return created, fmt.Errorf("create command %q: %w", cmd.Name, err)
		}
		created = append(created, c)
	}
	return created, nil
}

func waitForShutdown() {
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, os.Interrupt, syscall.SIGTERM)
	<-sc
}

func toSet(ids []string) map[string]struct{} {
	set := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		set[id] = struct{}{}
	}
	return set
}
