package packs

type OgDonator struct{}

func (f *OgDonator) GetItems() []string {
	items := []string{"AthenaCharacter:CID_028_Athena_Commando_F", "AthenaCharacter:CID_017_Athena_Commando_M", "AthenaCharacter:CID_083_Athena_Commando_F_Tactical", "AthenaDance:EID_Floss", "AthenaDance:EID_TakeTheL", "AthenaPickaxe:Pickaxe_ID_294_CandyCane"}
	return items
}
