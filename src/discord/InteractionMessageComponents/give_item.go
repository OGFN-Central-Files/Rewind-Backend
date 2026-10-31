package InteractionMessageComponents

import (
	"context"
	"go-backend/src/database/models"
	ItemDefinitions "go-backend/src/definitions/items"
	"go-backend/src/global"
	"go-backend/src/services"
	"time"

	"github.com/bwmarrin/discordgo"
	"go.mongodb.org/mongo-driver/bson"
)

var CreateNewItemDefinitionMap = ItemDefinitions.CreateNewItemDefinitionMap

func (c *InteractionMessageComponent) HandleGiveItem(s *discordgo.Session, i *discordgo.InteractionCreate, targetDiscordID string) {
	options := i.ApplicationCommandData().Options
	itemName := options[1].StringValue()

	funcs := &services.Functions{}

	profilesCollection := global.MongoDBClient.Database("fortnite").Collection("profiles")
	var existingProfile *models.Profile
	err := profilesCollection.FindOne(context.Background(), bson.M{"accountid": targetDiscordID}).Decode(&existingProfile)
	if err != nil {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Flags:   discordgo.MessageFlagsEphemeral,
				Content: "Profile not found.",
			},
		})
		return
	}

	profileId := "athena"
	profileData, ok := existingProfile.ProfileData[profileId].(map[string]interface{})
	if !ok {
		profileData = map[string]interface{}{
			"items":           make(map[string]interface{}),
			"stats":           map[string]interface{}{"attributes": make(map[string]interface{})},
			"rvn":             0,
			"commandRevision": 0,
			"updated":         time.Now().UTC().Format(time.RFC3339),
		}
		existingProfile.ProfileData[profileId] = profileData
	}

	items := profileData["items"].(map[string]interface{})
	items[itemName] = map[string]interface{}{
		"templateId":  itemName,
		"purchasedAt": time.Now().UTC().Format(time.RFC3339),
		"attributes": map[string]interface{}{
			"item_seen": false,
			"variants":  []interface{}{},
		},
		"quantity": 1,
	}

	funcs.GiveItemToAccount("GiftBox:GB_MakeGoodAthena", "Yw!", targetDiscordID, itemName, 1)

	_, err = profilesCollection.UpdateOne(context.Background(), bson.M{"accountid": targetDiscordID}, bson.M{
		"$set": bson.M{"profiledata.athena.items": items},
	})
	if err != nil {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Flags:   discordgo.MessageFlagsEphemeral,
				Content: "Failed to update the profile.",
			},
		})
		return
	}

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Flags:   discordgo.MessageFlagsEphemeral,
			Content: "Item added successfully to your Athena profile!",
		},
	})
}
