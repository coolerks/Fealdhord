package database

import "github.com/coolerks/Fealdhord/internal/database/sqlc"

// AIProviderQueries supplies the generated repository to the AI configuration service.
func (s *Store) AIProviderQueries() *sqlc.Queries { return s.queries }
