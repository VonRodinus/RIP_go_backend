package main

import (
	"RIP/internal/db"
	"RIP/internal/handlers"
	"RIP/internal/session"
	"log"
	"net/http"
	"strings"

	_ "RIP/docs"

	httpSwagger "github.com/swaggo/http-swagger"
)

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")

		allowedOrigins := []string{
			"https://tauri.localhost",
			"http://tauri.localhost",
			"http://localhost:3000",
			"http://127.0.0.1:3000",
			"http://172.17.158.148",
			"https://vonrodinus.github.io",

			"null",
			"tauri://localhost",
			"http://tauri.localhost:3000",
			"http://localhost:8080",

			"chrome-extension://*",
		}

		allowOrigin := ""
		if origin != "" {
			for _, allowed := range allowedOrigins {
				if origin == allowed {
					allowOrigin = origin
					break
				}
			}
		}

		if origin == "" || origin == "null" || r.Header.Get("Sec-Fetch-Site") == "none" {

			allowOrigin = "*"

			w.Header().Set("Access-Control-Allow-Origin", "tauri://localhost")
		} else if allowOrigin != "" {
			w.Header().Set("Access-Control-Allow-Origin", allowOrigin)
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Credentials", "true")
		} else {

			w.Header().Set("Access-Control-Allow-Origin", "*")
		}

		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func main() {
	db.Init()

	if err := session.PingRedis(); err != nil {
		log.Fatal("Redis НЕ РАБОТАЕТ! Запусти: docker start rip-redis")
	}
	log.Println("Redis подключён")

	mux := http.NewServeMux()

	mux.HandleFunc("/swagger/", httpSwagger.WrapHandler)

	mux.HandleFunc("/", handlers.ArtifactCatalogHandler)
	mux.HandleFunc("/artifact/", handlers.ArtifactDetailHandler)
	mux.HandleFunc("/tpq_request/", handlers.BuildingTPQCalcHandler)
	mux.HandleFunc("/add_artifact/", handlers.AddArtifactToRequestHandler)
	mux.HandleFunc("/delete_request/", handlers.DeleteRequestHandler)

	// === API РОУТЫ ===
	mux.HandleFunc("/api/artifacts", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/artifacts" {
			http.NotFound(w, r)
			return
		}
		switch r.Method {
		case http.MethodGet:
			handlers.GetArtifacts(w, r)
		case http.MethodPost:
			handlers.CreateArtifact(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/api/artifacts/", func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/artifacts/"), "/")
		if len(parts) == 0 || (len(parts) == 1 && parts[0] == "") {
			http.NotFound(w, r)
			return
		}
		if len(parts) == 1 {
			switch r.Method {
			case http.MethodGet:
				handlers.GetArtifact(w, r)
			case http.MethodPut:
				handlers.UpdateArtifact(w, r)
			case http.MethodDelete:
				handlers.DeleteArtifact(w, r)
			default:
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			}
			return
		}
		if parts[1] == "add_to_request" && r.Method == http.MethodPost {
			handlers.AddArtifactToRequest(w, r)
		} else if parts[1] == "image" && r.Method == http.MethodPost {
			handlers.UploadArtifactImage(w, r)
		} else {
			http.NotFound(w, r)
		}
	})

	mux.HandleFunc("/api/tpq_requests/cart", handlers.GetCartInfo)
	mux.HandleFunc("/api/tpq_requests", handlers.GetTPQRequests)

	mux.HandleFunc("/api/tpq_requests/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			handlers.GetTPQRequest(w, r)
			return
		}

		if r.Method == http.MethodPut {
			p := r.URL.Path
			switch {
			case strings.HasSuffix(p, "/form"):
				handlers.FormTPQRequest(w, r)
			case strings.HasSuffix(p, "/complete"):
				handlers.CompleteTPQRequest(w, r)
			case strings.HasSuffix(p, "/reject"):
				handlers.RejectTPQRequest(w, r)
			case strings.HasSuffix(p, "/moderate"):
				handlers.ModerateTPQRequest(w, r)
			case strings.HasSuffix(p, "/update_result"):
				handlers.UpdateTPQResult(w, r)
			case strings.Contains(p, "/items/"):
				handlers.UpdateTPQRequestItem(w, r)
			default:
				handlers.UpdateTPQRequest(w, r)
			}
			return
		}

		if r.Method == http.MethodDelete {
			if strings.Contains(r.URL.Path, "/items/") {
				handlers.DeleteTPQRequestItem(w, r)
			} else {
				handlers.DeleteTPQRequest(w, r)
			}
			return
		}

		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	})

	mux.HandleFunc("/api/users/register", handlers.RegisterUser)
	mux.HandleFunc("/api/users/me", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			handlers.GetMe(w, r)
		case http.MethodPut:
			handlers.UpdateMe(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})
	mux.HandleFunc("/api/users/login", handlers.Login)
	mux.HandleFunc("/api/users/logout", handlers.Logout)

	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	log.Println("Server starting on :8080...")
	log.Println("CORS enabled for https://vonrodinus.github.io")
	log.Fatal(http.ListenAndServe("0.0.0.0:8080", corsMiddleware(mux)))
}
