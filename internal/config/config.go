package config

type Config struct {
	Status map[string]int
}

func LoadConfig() Config {
	return Config{
		Status: map[string]int{
			"Cleared":   1,
			"Refunded":  2,
			"Cancelled": 3,
		},
	}
}
