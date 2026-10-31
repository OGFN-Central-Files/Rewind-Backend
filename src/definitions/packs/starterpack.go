package packs

type StarterPack struct{}

func (f *StarterPack) GetItems() []string {
	items := []string{"AthenaCharacter:CID_660_Athena_Commando_F_BandageNinjaBlue", "AthenaPickaxe:Pickaxe_ID_140_StreetGoth"}
	return items
}
