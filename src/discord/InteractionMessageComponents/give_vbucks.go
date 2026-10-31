package InteractionMessageComponents

import (
	"context"
	"go-backend/src/database/models"
	"go-backend/src/global"
	"go-backend/src/services"
	"strconv"
	"time"

	"github.com/bwmarrin/discordgo"
	"go.mongodb.org/mongo-driver/bson"
)

func (c *InteractionMessageComponent) HandleGiveVbucks(s *discordgo.Session, i *discordgo.InteractionCreate, targetDiscordID string) {
	options := i.ApplicationCommandData().Options
	amount := options[1].StringValue()

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

	profileId := "common_core"
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

	amountFloat, err := strconv.ParseFloat(amount, 64)
	if err != nil {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Flags:   discordgo.MessageFlagsEphemeral,
				Content: "Server Panic! Invalid amount defined.",
			},
		})
		return
	}

	items := profileData["items"].(map[string]interface{})
	mtxPurchased := items["Currency:MtxPurchased"].(map[string]interface{})

	if qty, ok := mtxPurchased["quantity"].(float64); ok {
		mtxPurchased["quantity"] = qty + amountFloat
	} else {
		mtxPurchased["quantity"] = amountFloat
	}

	funcs.GiveItemToAccount("GiftBox:FoundersRewards", "Yw!", targetDiscordID, "Currency:MtxPurchased", 1)

	_, err = profilesCollection.UpdateOne(context.Background(), bson.M{"accountid": targetDiscordID}, bson.M{
		"$set": bson.M{"profiledata.common_core.items": items},
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
			Content: "successfully gave V-Bucks to the targeted user!",
		},
	})
}
