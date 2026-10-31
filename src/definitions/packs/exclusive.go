package packs

type ExclusiveDonator struct{}

func (f *ExclusiveDonator) GetItems() []string {
	items := []string{"AthenaCharacter:CID_515_Athena_Commando_M_BarbequeLarry", "AthenaCharacter:CID_530_Athena_Commando_F_BlackMonday_1BV6J", "AthenaCharacter:CID_704_Athena_Commando_F_LollipopTrickster", "AthenaCharacter:CID_703_Athena_Commando_M_Cyclone", "AthenaPickaxe:Pickaxe_ID_508_HistorianMale_6BQSW", "AthenaPickaxe:Pickaxe_ID_363_LollipopTricksterFemale", "AthenaDance:EID_JulyBooks"}
	return items
}
