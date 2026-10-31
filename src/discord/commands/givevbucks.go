package commands

import (
	"context"
	"fmt"

	"go-backend/src/database/models"
	"go-backend/src/discord/InteractionMessageComponents"
	"go-backend/src/discord/special"
	"go-backend/src/global"

	"github.com/bwmarrin/discordgo"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

type GiveVbucksCommand struct {
	MongoClient *mongo.Client
}

func (c *GiveVbucksCommand) Register(s *discordgo.Session, appID string) error {
	cmd := &discordgo.ApplicationCommand{
		Name:        "givevbucks",
		Description: "gives vbucks to a certain user.",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "user",
				Description: "The username or Discord tag to give a item to.",
				Required:    false,
			},
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "amount",
				Description: "The amount of vbucks to add.",
				Required:    false,
			},
		},
	}

	_, err := s.ApplicationCommandCreate(appID, "", cmd)
	if err != nil {
		return fmt.Errorf("error creating ping command: %v", err)
	}

	return nil
}

func (c *GiveVbucksCommand) Handle(s *discordgo.Session, i *discordgo.InteractionCreate) {
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

	user := i.Interaction.User
	if user == nil {
		user = i.Interaction.Member.User
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

	if err == nil {
		InteractionMessageComponent := &InteractionMessageComponents.InteractionMessageComponent{}
		InteractionMessageComponent.HandleGiveVbucks(s, i, *&existingAccount.AccountID)
	} else {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Flags: discordgo.MessageFlagsEphemeral,
				Embeds: []*discordgo.MessageEmbed{
					{
						Title:       "Account Missing!",
						Description: "The user you we're attempting to give a item to was not found.",
						Color:       0xff0000,
					},
				},
			},
		})
		return
	}
}
