package utils

import (
	"encoding/json"
	"os"
)

// Contains checks if a slice contains a specific string
func Contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// SaveJSON saves any data structure to a JSON file
func SaveJSON(data interface{}, filename string) error {
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filename, jsonData, 0644)
}

// LoadJSON loads a JSON file into a data structure
func LoadJSON(filename string, v interface{}) error {
	data, err := os.ReadFile(filename)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, v)
}

// ExtractUniqueCurrencies extracts unique target currencies from opportunities
func ExtractUniqueCurrencies(opportunities interface{}) []string {
	// This would need to be implemented based on the specific type
	// For now, return empty slice
	return []string{}
}
