package commands

import (
	"context"
	"fmt"

	"go-backend/src/database/models"
	"go-backend/src/global"
	"go-backend/src/services"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/mongo"
)

type RegisterGSCommand struct {
	MongoClient *mongo.Client
	ProfileDir  string
}

func (c *RegisterGSCommand) Register(s *discordgo.Session, appID string) error {
	cmd := &discordgo.ApplicationCommand{
		Name:        "registergs",
		Description: "Register a gameserver account",
	}

	_, err := s.ApplicationCommandCreate(appID, "", cmd)
	if err != nil {
		return fmt.Errorf("error creating ping command: %v", err)
	}

	return nil
}

func (c *RegisterGSCommand) Handle(s *discordgo.Session, i *discordgo.InteractionCreate) {
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

	accountID := uuid.New().String()
	username := uuid.New().String()
	password := "gs"
	email := fmt.Sprintf("%s@go.org", uuid.New().String())

	account := models.NewAccount(
		accountID,
		username,
		email,
		password,
	)
	account.DiscordID = &user.ID

	accountsCollection := global.MongoDBClient.Database("fortnite").Collection("accounts")
	_, err := accountsCollection.InsertOne(context.Background(), account)
	if err != nil {
		funcs := &services.Functions{}
		funcs.Log(services.ERROR, "REGISTER", err.Error())
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Flags: discordgo.MessageFlagsEphemeral,
				Embeds: []*discordgo.MessageEmbed{
					{
						Title:       "Registration Failed",
						Description: "Failed to create your account. Please try again later.",
						Color:       0xff0000,
					},
				},
			},
		})
		return
	}

	profile, err := models.NewProfile(accountID, c.ProfileDir)
	if err != nil {
		funcs := &services.Functions{}
		funcs.Log(services.ERROR, "REGISTER", fmt.Sprintf("Failed to create profile: %v", err))
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Flags: discordgo.MessageFlagsEphemeral,
				Embeds: []*discordgo.MessageEmbed{
					{
						Title:       "Registration Failed",
						Description: "Failed to create your profile. Please try again later.",
						Color:       0xff0000,
					},
				},
			},
		})
		return
	}

	profilesCollection := global.MongoDBClient.Database("fortnite").Collection("profiles")
	_, err = profilesCollection.InsertOne(context.Background(), profile)
	if err != nil {
		funcs := &services.Functions{}
		funcs.Log(services.ERROR, "REGISTER", fmt.Sprintf("Failed to insert profile: %v", err))
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Flags: discordgo.MessageFlagsEphemeral,
				Embeds: []*discordgo.MessageEmbed{
					{
						Title:       "Registration Failed",
						Description: "Failed to save your profile. Please try again later.",
						Color:       0xff0000,
					},
				},
			},
		})
		return
	}

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Flags: discordgo.MessageFlagsEphemeral,
			Embeds: []*discordgo.MessageEmbed{
				{
					Title:       "Account Generated",
					Description: fmt.Sprintf("🎉 Your account has been created!\n\n**Email:** `%s`\n**Password:** `%s`", email, password),
					Color:       0x00cc66,
				},
			},
		},
	})
}
