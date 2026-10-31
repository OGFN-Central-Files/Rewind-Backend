package special

import (
	"go-backend/src/services"

	"github.com/bwmarrin/discordgo"
)

type Rights struct{}

var adminIDs = map[string]bool{
	"917699408374677504":  true,
	"1313439286560952424": true,
	"1353778830082314270": true,
}

func (f *Rights) ValidateAdminRights(s *discordgo.Session, user *discordgo.User) bool {
	if user == nil {
		funcs := &services.Functions{}
		funcs.Log(services.INFO, "LogRights", "User is nil")
		return false
	}

	return adminIDs[user.ID]
}
