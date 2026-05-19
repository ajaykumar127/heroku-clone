package main

import (
	"fmt"

	"github.com/google/uuid"
)

func generateID() string {
	return uuid.New().String()
}

// placeholder returns the SQL placeholder for position n (1-based).
// Postgres uses $1,$2,...; SQLite uses ?.
func placeholder(postgres bool, n int) string {
	if postgres {
		return fmt.Sprintf("$%d", n)
	}
	return "?"
}
