package config

type Config struct {
	Status      map[string]int
	IsRecurring map[string]int
}

func LoadConfig() Config {
	return Config{
		Status: map[string]int{
			"Cleared":   1,
			"Refunded":  2,
			"Cancelled": 3,
		},
		IsRecurring: map[string]int{
			"NoRecurring": 0,
			"Recurring":   1,
		},
	}
}
