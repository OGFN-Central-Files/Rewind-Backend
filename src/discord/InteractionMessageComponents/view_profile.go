package InteractionMessageComponents

import (
	"context"
	"go-backend/src/database/models"
	"go-backend/src/global"
	"strings"

	"github.com/bwmarrin/discordgo"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

type InteractionMessageComponent struct {
	MongoClient *mongo.Client
}

func (c *InteractionMessageComponent) HandleVie(s *discordgo.Session, i *discordgo.InteractionCreate) {
	user := i.Interaction.User
	if user == nil {
		user = i.Interaction.Member.User
	}

	customID := i.MessageComponentData().CustomID
	if !strings.HasPrefix(customID, "view_profile:") {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Flags:   discordgo.MessageFlagsEphemeral,
				Content: "Invalid CustomID format.",
			},
		})
		return
	}

	targetDiscordID := customID[len("view_profile:"):]

	accountsCollection := global.MongoDBClient.Database("fortnite").Collection("accounts")
	var existingAccount models.Account
	err := accountsCollection.FindOne(context.Background(), bson.M{"discordid": targetDiscordID}).Decode(&existingAccount)

	if err != nil {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Flags:   discordgo.MessageFlagsEphemeral,
				Content: "Could not retrieve profile data.",
			},
		})
		return
	}

	embed := &discordgo.MessageEmbed{
		Title: "User Profile",
		Color: 0x00ff00,
		Fields: []*discordgo.MessageEmbedField{
			{Name: "Email", Value: existingAccount.Email, Inline: false},
			{Name: "Username", Value: existingAccount.Username, Inline: false},
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
