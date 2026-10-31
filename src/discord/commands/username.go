package commands

import (
	"context"
	"fmt"
	"time"

	"go-backend/src/database/models"
	"go-backend/src/global"

	"github.com/bwmarrin/discordgo"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

type UsernameCommand struct {
	MongoClient *mongo.Client
}

func (c *UsernameCommand) Register(s *discordgo.Session, appID string) error {
	cmd := &discordgo.ApplicationCommand{
		Name:        "changeusername",
		Description: "Lets you change your username (every 7 days)",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "username",
				Description: "Your new username",
				Required:    true,
			},
		},
	}

	_, err := s.ApplicationCommandCreate(appID, "", cmd)
	if err != nil {
		return fmt.Errorf("error creating changeusername command: %v", err)
	}

	return nil
}

func (c *UsernameCommand) Handle(s *discordgo.Session, i *discordgo.InteractionCreate) {
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

	accountsCollection := global.MongoDBClient.Database("fortnite").Collection("accounts")
	var existingAccount models.Account
	err := accountsCollection.FindOne(context.Background(), bson.M{"discordid": user.ID}).Decode(&existingAccount)
	if err != nil {
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

	var additionalData map[string]interface{}
	if existingAccount.Additional != nil {
		additionalData = existingAccount.Additional
	} else {
		additionalData = make(map[string]interface{})
	}

	if privileged, ok := additionalData["bTestingPriviliged"].(bool); !ok || !privileged {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Flags: discordgo.MessageFlagsEphemeral,
				Embeds: []*discordgo.MessageEmbed{
					{
						Title:       "Permission Denied",
						Description: "You are not authorized to change your username at this time.",
						Color:       0xff0000,
					},
				},
			},
		})
		return
	}

	options := i.ApplicationCommandData().Options
	if len(options) == 0 {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Flags: discordgo.MessageFlagsEphemeral,
				Embeds: []*discordgo.MessageEmbed{
					{
						Title:       "Invalid Input",
						Description: "Please provide a new username.",
						Color:       0xff0000,
					},
				},
			},
		})
		return
	}

	newUsername := options[0].StringValue()
	count, err := accountsCollection.CountDocuments(context.Background(), bson.M{"username": newUsername})
	if err != nil {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Flags: discordgo.MessageFlagsEphemeral,
				Embeds: []*discordgo.MessageEmbed{
					{
						Title:       "Error",
						Description: "Error checking username availability.",
						Color:       0xff0000,
					},
				},
			},
		})
		return
	}
	if count > 0 {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Flags: discordgo.MessageFlagsEphemeral,
				Embeds: []*discordgo.MessageEmbed{
					{
						Title:       "Username Unavailable",
						Description: "That username is already taken. Please choose another.",
						Color:       0xff0000,
					},
				},
			},
		})
		return
	}

	if lastChange, ok := additionalData["LastUsernameChange"].(int64); ok {
		currentTime := time.Now().Unix()
		if timeDiff := currentTime - lastChange; timeDiff < 7*24*60*60 {
			remaining := time.Duration((7*24*60*60 - timeDiff)) * time.Second
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Flags: discordgo.MessageFlagsEphemeral,
					Embeds: []*discordgo.MessageEmbed{
						{
							Title:       "Username Change Cooldown",
							Description: fmt.Sprintf("You can change your username again in %s.", remaining.String()),
							Color:       0xff0000,
						},
					},
				},
			})
			return
		}
	}

	additionalData["LastUsernameChange"] = time.Now().Unix()
	update := bson.M{
		"$set": bson.M{
			"username":   newUsername,
			"additional": additionalData,
		},
	}
	_, err = accountsCollection.UpdateOne(context.Background(), bson.M{"discordid": user.ID}, update)
	if err != nil {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Flags: discordgo.MessageFlagsEphemeral,
				Embeds: []*discordgo.MessageEmbed{
					{
						Title:       "Error",
						Description: "Failed to update your username. Please try again later.",
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
					Title:       "Username Changed",
					Description: fmt.Sprintf("Your username has been updated to **%s**.", newUsername),
					Color:       0x00ff00,
				},
			},
		},
	})
}
