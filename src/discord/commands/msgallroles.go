package commands

import (
	"fmt"
	"go-backend/src/discord/special"

	"github.com/bwmarrin/discordgo"
)

type MsgAllCommand struct{}

func (c *MsgAllCommand) Register(s *discordgo.Session, appID string) error {
	cmd := &discordgo.ApplicationCommand{
		Name:        "msgallroles",
		Description: "Sends a DM to everyone with the specified role.",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionRole,
				Name:        "role",
				Description: "The role to target",
				Required:    true,
			},
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "message",
				Description: "The message to send",
				Required:    true,
			},
		},
	}

	_, err := s.ApplicationCommandCreate(appID, "", cmd)
	if err != nil {
		return fmt.Errorf("error creating command: %v", err)
	}

	return nil
}

func (c *MsgAllCommand) Handle(s *discordgo.Session, i *discordgo.InteractionCreate) {
	options := i.ApplicationCommandData().Options
	var roleID, message string

	for _, opt := range options {
		if opt.Name == "role" {
			roleID = opt.RoleValue(nil, "").ID
		}
		if opt.Name == "message" {
			message = opt.StringValue()
		}
	}

	var user *discordgo.User
	if i.Interaction.User != nil {
		user = i.Interaction.User
	} else if i.Interaction.Member != nil {
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
						Description: "You do not have permissions to validate this account.",
						Color:       0xff0000,
					},
				},
			},
		})
		return
	}

	guildID := i.GuildID
	members := []*discordgo.Member{}
	after := ""
	for {
		chunk, err := s.GuildMembers(guildID, after, 1000)
		if err != nil || len(chunk) == 0 {
			break
		}
		members = append(members, chunk...)
		after = chunk[len(chunk)-1].User.ID
		if len(chunk) < 1000 {
			break
		}
	}

	count := 0
	for _, member := range members {
		for _, r := range member.Roles {
			if r == roleID {
				channel, err := s.UserChannelCreate(member.User.ID)
				if err == nil {
					_, err = s.ChannelMessageSend(channel.ID, message)
					if err == nil {
						count++
					}
				}
				break
			}
		}
	}

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Flags:   discordgo.MessageFlagsEphemeral,
			Content: fmt.Sprintf("Message sent to %d members with the specified role.", count),
		},
	})
}
