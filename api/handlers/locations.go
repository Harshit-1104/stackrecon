package handlers

import (
	"log"
	"encoding/json"
	"net/http"
)

type LocationResponse struct {
	Countries []Country `json:"countries"`
}

type Country struct {
	Name   string   `json:"name"`
	Cities []string `json:"cities"`
}

func (h *Handler) GetLocations(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.Query(r.Context(), "SELECT DISTINCT location_country, location_city FROM job_posting WHERE location_country IS NOT NULL AND active = true")
	if err != nil {
		log.Printf("Internal server error: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	countryMap := make(map[string]map[string]bool)
	for rows.Next() {
		var country string
		var city *string
		if err := rows.Scan(&country, &city); err != nil {
			continue
		}
		if _, ok := countryMap[country]; !ok {
			countryMap[country] = make(map[string]bool)
		}
		if city != nil && *city != "" {
			countryMap[country][*city] = true
		}
	}

	res := LocationResponse{Countries: []Country{}}
	for c, citiesMap := range countryMap {
		countryObj := Country{Name: c, Cities: []string{}}
		for city := range citiesMap {
			countryObj.Cities = append(countryObj.Cities, city)
		}
		res.Countries = append(res.Countries, countryObj)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(res)
}
