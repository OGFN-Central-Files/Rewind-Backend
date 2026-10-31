package discord

import (
	"context"
	"fmt"
	"go-backend/src/global"
	"go-backend/src/services"
	"log"
	"strconv"
	"strings"

	"go-backend/src/discord/InteractionMessageComponents"
	"go-backend/src/discord/commands"

	"github.com/bwmarrin/discordgo"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var BotToken = "MTQ2MTg1NjMyNTcxNjY3NjY3OQ.GOb_Qf.cIQjoAV8vmBn5cVqQZxpqMTYzAbQvka6nVf9PY"
var session *discordgo.Session
var mongoClient *mongo.Client

func RegisterCommands(s *discordgo.Session, appID string) error {
	existingCommands, err := s.ApplicationCommands(appID, "")
	if err != nil {
		return fmt.Errorf("error fetching existing commands: %v", err)
	}
	for _, cmd := range existingCommands {
		err := s.ApplicationCommandDelete(appID, "", cmd.ID)
		if err != nil {
			log.Printf("error deleting command %s: %v", cmd.Name, err)
		}
	}

	commandsList := []struct {
		command interface {
			Register(s *discordgo.Session, appID string) error
		}
	}{
		{&commands.RegisterGSCommand{MongoClient: mongoClient, ProfileDir: "./static/DefaultProfiles"}},
		{&commands.RegisterCommand{MongoClient: mongoClient, ProfileDir: "./static/DefaultProfiles"}},
		{&commands.DetailsCommand{MongoClient: mongoClient}},
		{&commands.UsernameCommand{MongoClient: mongoClient}},
		{&commands.LookupCommand{MongoClient: mongoClient}},
		{&commands.GiveItemCommand{MongoClient: mongoClient}},
		{&commands.GiveVbucksCommand{MongoClient: mongoClient}},
		{&commands.SessionsCommand{MongoClient: mongoClient}},
		{&commands.MsgAllCommand{}},
	}

	for _, cmd := range commandsList {
		if err := cmd.command.Register(s, appID); err != nil {
			return fmt.Errorf("error registering command: %v", err)
		}
	}

	return nil
}

func UpdatePlayerStatus() {
	err := session.UpdateStatusComplex(discordgo.UpdateStatusData{
		Activities: []*discordgo.Activity{
			{
				Name: strconv.FormatInt(global.PlayerCount, 10) + " players",
				Type: discordgo.ActivityTypeWatching,
			},
		},
		Status: "online",
		AFK:    false,
	})

	if err != nil {
		log.Printf("error setting status: %v", err)
	}
}

func StartBot() {
	var err error

	mongoClient, err = mongo.Connect(context.Background(), options.Client().ApplyURI("fuck you"))
	if err != nil {
		log.Fatalf("Failed to connect to MongoDB: %v", err)
	}

	err = mongoClient.Ping(context.Background(), nil)
	if err != nil {
		log.Fatalf("Failed to ping MongoDB: %v", err)
	}

	session, err = discordgo.New("Bot " + BotToken)
	if err != nil {
		log.Fatalf("error creating Discord session: %v", err)
	}

	session.Identify.Intents = discordgo.IntentsGuildMembers | discordgo.IntentsGuilds

	err = session.Open()
	if err != nil {
		log.Fatalf("error opening connection to Discord: %v", err)
	}

	appID := session.State.User.ID
	if appID == "" {
		log.Fatalf("error retrieving bot application ID")
	}

	go func() {
		err := RegisterCommands(session, appID)
		if err != nil {
			log.Fatalf("Error registering commands: %v", err)
		}

		funcs := &services.Functions{}
		funcs.Log(services.INFO, "LogDiscord", "Finished setting up commands!")
	}()

	global.MongoDBClient = mongoClient

	session.AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		switch i.Type {
		case discordgo.InteractionApplicationCommand:
			switch i.ApplicationCommandData().Name {
			case "register":
				cmd := &commands.RegisterCommand{MongoClient: mongoClient, ProfileDir: "./static/DefaultProfiles"}
				cmd.Handle(s, i)
			case "registergs":
				cmd := &commands.RegisterGSCommand{MongoClient: mongoClient, ProfileDir: "./static/DefaultProfiles"}
				cmd.Handle(s, i)
			case "details":
				cmd := &commands.DetailsCommand{MongoClient: mongoClient}
				cmd.Handle(s, i)
			case "changeusername":
				cmd := &commands.UsernameCommand{MongoClient: mongoClient}
				cmd.Handle(s, i)
			case "lookup":
				cmd := &commands.LookupCommand{MongoClient: mongoClient}
				cmd.Handle(s, i)
			case "giveitem":
				cmd := &commands.GiveItemCommand{MongoClient: mongoClient}
				cmd.Handle(s, i)
			case "givevbucks":
				cmd := &commands.GiveVbucksCommand{MongoClient: mongoClient}
				cmd.Handle(s, i)
			case "sessions":
				cmd := &commands.SessionsCommand{MongoClient: mongoClient}
				cmd.Handle(s, i)
			case "msgallroles":
				cmd := &commands.MsgAllCommand{}
				cmd.Handle(s, i)
			}

		case discordgo.InteractionMessageComponent:
			customID := i.MessageComponentData().CustomID

			if strings.HasPrefix(customID, "view_profile:") {
				component := &InteractionMessageComponents.InteractionMessageComponent{}
				component.HandleVie(s, i)
				return
			}

			if strings.HasPrefix(customID, "ban_user:") {
				component := &InteractionMessageComponents.InteractionMessageComponent{}
				component.HandleBanUser(s, i)
				return
			}
		}
	})
}
