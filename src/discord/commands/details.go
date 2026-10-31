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

type DetailsCommand struct {
	MongoClient *mongo.Client
}

func (c *DetailsCommand) Register(s *discordgo.Session, appID string) error {
	cmd := &discordgo.ApplicationCommand{
		Name:        "details",
		Description: "Shows your account details.",
	}

	_, err := s.ApplicationCommandCreate(appID, "", cmd)
	if err != nil {
		return fmt.Errorf("error creating ping command: %v", err)
	}

	return nil
}

func (c *DetailsCommand) Handle(s *discordgo.Session, i *discordgo.InteractionCreate) {
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
						Description: "You do not have permissions to validate this account.",
						Color:       0xff0000,
					},
				},
			},
		})
		return
	}

	accountsCollection := global.MongoDBClient.Database("fortnite").Collection("accounts")
	var existingAccount models.Account
	err := accountsCollection.FindOne(context.Background(), bson.M{"discordid": user.ID}).Decode(&existingAccount)

	if err == nil {
		embed := &discordgo.MessageEmbed{
			Title: "Account Details",
			Color: 0x00ff00,
			Fields: []*discordgo.MessageEmbedField{
				{
					Name:   "Email",
					Value:  existingAccount.Email,
					Inline: false,
				},
				{
					Name:   "Password",
					Value:  existingAccount.Password,
					Inline: false,
				},
				{
					Name:   "Account ID",
					Value:  existingAccount.AccountID,
					Inline: false,
				},
				{
					Name:   "Username",
					Value:  existingAccount.Username,
					Inline: false,
				},
			},
			Footer: &discordgo.MessageEmbedFooter{
				Text: "Account details retrieved successfully.",
			},
		}

		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Flags:  discordgo.MessageFlagsEphemeral,
				Embeds: []*discordgo.MessageEmbed{embed},
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
						Description: "You don't have an account yet, or an error occurred.",
						Color:       0xff0000,
					},
				},
			},
		})
		return
	}
}
