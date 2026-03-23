package server

import (
	"encoding/json"
	"net/http"
)

type meResponse struct {
	User string `json:"user"`
	Role string `json:"role"`
}

func HandleMe(store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := UserFromContext(r.Context())
		store.TouchSession(user)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(meResponse{
			User: user,
			Role: RoleFromContext(r.Context()),
		})
	}
}
