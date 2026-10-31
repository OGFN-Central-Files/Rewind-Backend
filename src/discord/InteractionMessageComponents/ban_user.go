package InteractionMessageComponents

import (
	"context"
	"fmt"
	"go-backend/src/database/models"
	"go-backend/src/global"
	"log"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"go.mongodb.org/mongo-driver/bson"
)

func (c *InteractionMessageComponent) HandleBanUser(s *discordgo.Session, i *discordgo.InteractionCreate) {
	user := i.Interaction.User
	if user == nil {
		user = i.Interaction.Member.User
	}

	customID := i.MessageComponentData().CustomID
	if !strings.HasPrefix(customID, "ban_user:") {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Flags:   discordgo.MessageFlagsEphemeral,
				Content: "Invalid CustomID format.",
			},
		})
		return
	}

	targetDiscordID := customID[len("ban_user:"):]

	accountsCollection := global.MongoDBClient.Database("fortnite").Collection("accounts")
	var existingAccount models.Account
	err := accountsCollection.FindOne(context.Background(), bson.M{"discordid": targetDiscordID}).Decode(&existingAccount)
	if err != nil {

	}
	newBannedStatus := !existingAccount.Banned
	_, err = accountsCollection.UpdateOne(
		context.Background(),
		bson.M{"discordid": targetDiscordID},
		bson.M{"$set": bson.M{"banned": newBannedStatus}},
	)

	if err != nil {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Flags:   discordgo.MessageFlagsEphemeral,
				Content: "Failed to ban the user.",
			},
		})
		return
	}

	profilesCollection := global.MongoDBClient.Database("fortnite").Collection("profiles")
	var existingProfile *models.Profile
	err = profilesCollection.FindOne(context.Background(), bson.M{"accountid": existingAccount.AccountID}).Decode(&existingProfile)
	if err != nil {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Flags:   discordgo.MessageFlagsEphemeral,
				Content: "User was **banned** but we *failed* to add the ban_status to common_core!",
			},
		})
		return
	}

	profileData, ok := existingProfile.ProfileData["common_core"].(map[string]interface{})
	if !ok {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Flags:   discordgo.MessageFlagsEphemeral,
				Content: "User was **banned** but we *failed* to find the common_core profile *(could not add ban_status)*!",
			},
		})
		return
	}

	stats, ok := profileData["stats"].(map[string]interface{})
	if !ok {
		log.Fatal("profileData['stats'] is not a map[string]interface{}")
	}

	if newBannedStatus {
		stats["attributes"] = map[string]interface{}{
			"ban_status": map[string]interface{}{
				"banReasons":           []interface{}{"Exploiting"},
				"exploitProgramName":   "",
				"additionalInfo":       "",
				"banStartTimeUtc":      time.Now().UTC().Format(time.RFC3339),
				"banDurationDays":      1.0,
				"competitiveBanReason": "None",
				"bRequiresUserAck":     false,
				"bBanHasStarted":       true,
			},
		}
	} else {
		stats["attributes"] = map[string]interface{}{
			"ban_status": map[string]interface{}{},
		}
	}

	_, err = profilesCollection.UpdateOne(
		context.Background(),
		bson.M{"accountid": existingAccount.AccountID},
		bson.M{"$set": bson.M{
			"profiledata.common_core": profileData,
		}},
	)

	if err != nil {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Flags:   discordgo.MessageFlagsEphemeral,
				Content: "User was **banned** but we *failed* to update common_core profile *(could not add ban_status)*!",
			},
		})
		return
	}

	if newBannedStatus {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Flags:   discordgo.MessageFlagsEphemeral,
				Content: fmt.Sprintf("User <@%s> has been banned.", targetDiscordID),
			},
		})
	} else {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Flags:   discordgo.MessageFlagsEphemeral,
				Content: fmt.Sprintf("User <@%s> has been unbanned.", targetDiscordID),
			},
		})
	}
}
