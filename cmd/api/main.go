package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
	
	"github.com/stackrecon/api/handlers"
	apimiddleware "github.com/stackrecon/api/middleware"
)

func main() {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://postgres:password@localhost:5432/stackrecon"
	}

	pool, err := pgxpool.New(context.Background(), dbURL)
	if err != nil {
		log.Fatalf("Unable to connect to database: %v\n", err)
	}
	defer pool.Close()

	r := chi.NewRouter()

	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(apimiddleware.CORS)

	aliasMapPath := flag.String("alias-map", "./alias_map.json", "Path to alias map JSON file")
	blocklistPath := flag.String("blocklist", "./skill_blocklist_resolved.json", "Path to blocklist JSON file")
	flag.Parse()

	// Dependency inject the DB pool into handlers
	h := handlers.New(pool, *aliasMapPath, *blocklistPath)

	r.Get("/health", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})

	r.Route("/api/v0", func(r chi.Router) {
		r.Get("/skills/search", h.SearchSkills)
		r.Post("/resume/parse", h.ParseResume)
		r.Post("/match/companies", h.MatchCompanies)
		r.Get("/companies/search", h.SearchCompanies)
		r.Get("/companies/{id}/stack", h.CompanyStack)
		r.Get("/companies/{id}/postings", h.CompanyPostings)
		r.Post("/companies/{id}/fit", h.CompanyFit)
		r.Get("/companies/{id}/similar", h.CompanySimilar)
		r.Get("/locations", h.GetLocations)
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	srv := &http.Server{
		Addr:    ":" + port,
		Handler: r,
	}

	serverErrors := make(chan error, 1)

	go func() {
		fmt.Printf("Starting API server on port %s...\n", port)
		serverErrors <- srv.ListenAndServe()
	}()

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-serverErrors:
		log.Fatalf("Error starting server: %v", err)

	case sig := <-shutdown:
		fmt.Printf("Starting shutdown... Signal: %v\n", sig)

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := srv.Shutdown(ctx); err != nil {
			srv.Close()
			log.Fatalf("Could not stop server gracefully: %v", err)
		}
	}
}
