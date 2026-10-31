package commands

import (
	"context"
	"fmt"
	"go-backend/src/database/models"
	"go-backend/src/discord/special"
	"go-backend/src/global"

	"github.com/bwmarrin/discordgo"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

type LookupCommand struct {
	MongoClient *mongo.Client
}

func (c *LookupCommand) Register(s *discordgo.Session, appID string) error {
	cmd := &discordgo.ApplicationCommand{
		Name:        "lookup",
		Description: "Shows account details.",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "user",
				Description: "The username or Discord tag to lookup",
				Required:    false,
			},
		},
	}

	_, err := s.ApplicationCommandCreate(appID, "", cmd)
	if err != nil {
		return fmt.Errorf("error creating details command: %v", err)
	}

	return nil
}

func getString(ptr *string) string {
	if ptr != nil {
		return *ptr
	}
	return "unknown"
}

func (c *LookupCommand) Handle(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if global.MongoDBClient == nil {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Flags:   discordgo.MessageFlagsEphemeral,
				Content: "Database connection is not available.",
			},
		})
		return
	}

	var query bson.M
	var searchUser string
	options := i.ApplicationCommandData().Options
	if len(options) > 0 {
		searchUser = options[0].StringValue()

		query = bson.M{"$or": []bson.M{
			{"discordid": searchUser},
			{"username": bson.M{"$regex": fmt.Sprintf("^%s$", searchUser), "$options": "i"}},
		}}
	} else {
		user := i.Interaction.User
		if user == nil {
			user = i.Interaction.Member.User
		}
		query = bson.M{"discordid": user.ID}
	}

	accountsCollection := global.MongoDBClient.Database("fortnite").Collection("accounts")
	var existingAccount models.Account
	err := accountsCollection.FindOne(context.Background(), query).Decode(&existingAccount)

	Rights := &special.Rights{}
	valid := Rights.ValidateAdminRights(s, i.Interaction.User)
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

	if err == nil {
		embed := &discordgo.MessageEmbed{
			Title:       "Account Lookup",
			Color:       0x00ff00,
			Description: "Select an action:",
		}

		buttonLabel := "Ban"
		if existingAccount.Banned {
			buttonLabel = "Unban"
		}

		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Flags:  discordgo.MessageFlagsEphemeral,
				Embeds: []*discordgo.MessageEmbed{embed},
				Components: []discordgo.MessageComponent{
					discordgo.ActionsRow{
						Components: []discordgo.MessageComponent{
							discordgo.Button{
								Label:    "View Profile Data",
								Style:    discordgo.PrimaryButton,
								CustomID: "view_profile:" + getString(existingAccount.DiscordID),
							},
							discordgo.Button{
								Label:    "Give Items",
								Style:    discordgo.SecondaryButton,
								CustomID: "add_item:" + getString(existingAccount.DiscordID),
							},
							discordgo.Button{
								Label:    buttonLabel,
								Style:    discordgo.DangerButton,
								CustomID: "ban_user:" + getString(existingAccount.DiscordID),
							},
						},
					},
				},
			},
		})
	} else {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Flags: discordgo.MessageFlagsEphemeral,
				Embeds: []*discordgo.MessageEmbed{
					{
						Title:       "Account Missing!",
						Description: "No account found for the given user.",
						Color:       0xff0000,
					},
				},
			},
		})
	}
}
