package commands

import (
	"context"
	"fmt"

	"go-backend/src/database/models"
	"go-backend/src/discord/special"
	"go-backend/src/global"
	"go-backend/src/services"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

type RegisterCommand struct {
	MongoClient *mongo.Client
	ProfileDir  string
}

func (c *RegisterCommand) Register(s *discordgo.Session, appID string) error {
	cmd := &discordgo.ApplicationCommand{
		Name:        "register",
		Description: "Register an account",
	}

	_, err := s.ApplicationCommandCreate(appID, "", cmd)
	if err != nil {
		return fmt.Errorf("error creating ping command: %v", err)
	}

	return nil
}

func (c *RegisterCommand) Handle(s *discordgo.Session, i *discordgo.InteractionCreate) {
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
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Flags: discordgo.MessageFlagsEphemeral,
				Embeds: []*discordgo.MessageEmbed{
					{
						Title:       "Account Already Exists",
						Description: "You already have an account registered.",
						Color:       0xff0000,
					},
				},
			},
		})
		return
	} else if err != mongo.ErrNoDocuments {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Flags: discordgo.MessageFlagsEphemeral,
				Embeds: []*discordgo.MessageEmbed{
					{
						Title:       "Error",
						Description: "An error occurred while checking your account.",
						Color:       0xff0000,
					},
				},
			},
		})
		return
	}

	accountID := uuid.New().String()
	password := uuid.New().String()
	email := fmt.Sprintf("%s@go.org", user.Username)

	account := models.NewAccount(
		accountID,
		user.Username,
		email,
		password,
	)

	account.DiscordID = &user.ID

	_, err = accountsCollection.InsertOne(context.Background(), account)
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
					Title:       "Account Created Successfully",
					Description: "Your account and profile have been successfully registered, Use `/details` to view your login data!",
					Color:       0x0099ff,
				},
			},
		},
	})
}
