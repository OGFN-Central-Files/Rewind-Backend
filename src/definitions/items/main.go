package ItemDefinitions

type ItemDefinition struct {
	TemplateId string
}

type ItemDefinitionMap []ItemDefinition

func CreateNewItemDefinitionMap(templateIds []string) ItemDefinitionMap {
	var newItemDefinitionMap ItemDefinitionMap
	for _, id := range templateIds {
		newItemDefinitionMap = append(newItemDefinitionMap, ItemDefinition{TemplateId: id})
	}
	return newItemDefinitionMap
}
