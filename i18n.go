package main

import (
	"encoding/json"
	"log"
	"os"
)

var translations = make(map[Language]map[string]string)

func loadLanguage(lang Language, path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		log.Fatal(err)
	}

	var values map[string]string

	if err := json.Unmarshal(data, &values); err != nil {
		log.Fatal(err)
	}

	translations[lang] = values
}

func LoadTranslations() {
	loadLanguage(LanguageEN, "i18n/en.json")
	loadLanguage(LanguageDE, "i18n/de.json")
	loadLanguage(LanguageZHTW, "i18n/zh-TW.json")
}

func translate(lang Language, key string) string {
	if text, ok := translations[lang][key]; ok {
		return text
	}

	if text, ok := translations[LanguageEN][key]; ok {
		return text
	}

	return key
}
