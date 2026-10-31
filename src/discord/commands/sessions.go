package commands

import (
	"fmt"
	"time"

	"go-backend/src/discord/special"
	"go-backend/src/global"

	"github.com/bwmarrin/discordgo"
	"go.mongodb.org/mongo-driver/mongo"
)

type SessionsCommand struct {
	MongoClient *mongo.Client
}

func (c *SessionsCommand) Register(s *discordgo.Session, appID string) error {
	cmd := &discordgo.ApplicationCommand{
		Name:        "sessions",
		Description: "Shows all active game sessions",
	}

	_, err := s.ApplicationCommandCreate(appID, "", cmd)
	if err != nil {
		return fmt.Errorf("error creating sessions command: %v", err)
	}

	return nil
}

func (c *SessionsCommand) Handle(s *discordgo.Session, i *discordgo.InteractionCreate) {
	user := i.Interaction.User
	if user == nil {
		user = i.Interaction.Member.User
	}

	Rights := &special.Rights{}
	valid := Rights.ValidateAdminRights(s, user)
	if !valid {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Flags: discordgo.MessageFlagsEphemeral,
				Embeds: []*discordgo.MessageEmbed{
					{
						Title:       "Validation Failed!",
						Description: "You do not have permissions to view session details.",
						Color:       0xff0000,
					},
				},
			},
		})
		return
	}

	if len(global.Sessions) == 0 {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Flags:   discordgo.MessageFlagsEphemeral,
				Content: "There are no active sessions at the moment.",
			},
		})
		return
	}

	now := time.Now()
	twoMinutesAgo := now.Add(-2 * time.Minute)

	fields := []*discordgo.MessageEmbedField{}
	activeCount := 0

	for id, session := range global.Sessions {
		parsedTime, err := time.Parse(time.RFC3339, session.LastUpdated)
		if err != nil || parsedTime.Before(twoMinutesAgo) {
			continue
		}

		activeCount++

		field := &discordgo.MessageEmbedField{
			Name: fmt.Sprintf("Session: %s", id),
			Value: fmt.Sprintf(
				"**Player Count:** %d\n**Session ID:** %s\n**Last Updated:** %s\n**Owner ID:** %s\n**Open Public Players:** %d\n**Started:** %t",
				len(session.PublicPlayers)+len(session.PrivatePlayers),
				session.SessionId,
				parsedTime.Format("2006-01-02 15:04:05"),
				session.OwnerId,
				session.OpenPublicPlayers,
				session.Started,
			),
			Inline: false,
		}
		fields = append(fields, field)
	}

	if activeCount == 0 {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Flags:   discordgo.MessageFlagsEphemeral,
				Content: "There are no recently active sessions (within the last 2 minutes).",
			},
		})
		return
	}

	embed := &discordgo.MessageEmbed{
		Title:  "Active Sessions (Last 2 Minutes)",
		Color:  0x00ffcc,
		Fields: fields,
		Footer: &discordgo.MessageEmbedFooter{
			Text: fmt.Sprintf("Total Recent Sessions: %d", activeCount),
		},
	}

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Flags:  discordgo.MessageFlagsEphemeral,
			Embeds: []*discordgo.MessageEmbed{embed},
		},
	})
}
