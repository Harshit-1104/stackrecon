package middleware

import (
	"github.com/go-chi/cors"
)

var CORS = cors.Handler(cors.Options{
	AllowedOrigins:   []string{"https://stackrecon.dev", "http://localhost:3000", "http://localhost:3001"},
	AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
	AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
	ExposedHeaders:   []string{"Link"},
	AllowCredentials: true,
	MaxAge:           300,
})
