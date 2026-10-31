package packs

type SweatDonator struct{}

func (f *SweatDonator) GetItems() []string {
	items := []string{"AthenaCharacter:CID_511_Athena_Commando_M_CubePaintWildCard", "AthenaCharacter:CID_715_Athena_Commando_F_TwinDark", "AthenaCharacter:CID_061_Athena_Commando_F_SkiGirl", "AthenaDance:EID_JanuaryBop", "AthenaPickaxe:Pickaxe_ID_179_StarWand", "AthenaPickaxe:Pickaxe_ID_014_WinterCamo"}
	return items
}
