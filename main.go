package main

import (
	"context"
	"database/sql"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"backend/handler"
	"backend/middleware"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

/*
===========================
CORS (liberal para dev)
===========================
*/
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Em dev: liberar tudo. Em prod, prefira origem específica.
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-User-Email")
		w.Header().Set("Access-Control-Max-Age", "86400") // 24h (preflight cache)

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}

/*
===========================
Conexão com o banco
===========================
*/
func conectarBanco() *sql.DB {
	_ = godotenv.Load()

	connStr := os.Getenv("DATABASE_URL")
	if connStr == "" {
		log.Fatal("DATABASE_URL não setada no .env")
	}

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatal("Erro ao abrir conexão com banco:", err)
	}
	if err = db.Ping(); err != nil {
		log.Fatal("Não foi possível conectar ao banco de dados:", err)
	}
	// Pool básico (ajuste para sua infra)
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	log.Println("Conectado ao banco de dados!")
	return db
}

/*
===========================
Rotas
===========================
*/
func registrarRotas(mux *http.ServeMux, db *sql.DB) {
	// Usuário (auth)
	mux.Handle("/register", corsMiddleware(handler.RegisterHandler(db)))
	mux.Handle("/login", corsMiddleware(handler.LoginHandler(db)))

	// Perfil / Usuário
	mux.Handle("/api/perfil", corsMiddleware(handler.AtualizarPerfilHandler(db)))
	mux.Handle("/api/usuario", corsMiddleware(handler.BuscarUsuarioPorEmailHandler(db))) // GET ?email=

	// ✅ PUT /api/usuario/{id}/tutorial
	mux.Handle("/api/usuario/", corsMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// esperamos exatamente /api/usuario/{id}/tutorial
		path := strings.TrimPrefix(r.URL.Path, "/api/usuario/")
		parts := strings.Split(strings.Trim(path, "/"), "/")
		if len(parts) == 2 && parts[1] == "tutorial" && r.Method == http.MethodPut {
			handler.MarcarTutorialVistoHandler(db).ServeHTTP(w, r)
			return
		}
		http.NotFound(w, r)
	})))

	// ✅ Checagem de CPF (duplicidade por usuário) — registrar UMA vez só
	mux.Handle("/api/estudantes/check-cpf", corsMiddleware(handler.VerificarCpfHandler(db)))

	// Estudantes (lista + cria)
	mux.Handle("/api/estudantes", corsMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			handler.ListarEstudantesHandler(db)(w, r)
		case http.MethodPost:
			// valida e-mail do estudante via middleware sem remover o restante
			middleware.ValidarEstudanteEmailMiddleware(handler.CriarEstudanteHandler(db))(w, r)
		default:
			http.Error(w, "Método não permitido", http.StatusMethodNotAllowed)
		}
	})))

	// Estudantes (update/delete por id)
	mux.Handle("/api/estudantes/", corsMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		idStr := strings.TrimPrefix(r.URL.Path, "/api/estudantes/")
		if idStr == "" {
			http.Error(w, "ID não informado", http.StatusBadRequest)
			return
		}
		if _, err := strconv.Atoi(idStr); err != nil {
			http.Error(w, "ID inválido", http.StatusBadRequest)
			return
		}
		switch r.Method {
		case http.MethodPut:
			// valida e-mail do estudante também nas edições
			middleware.ValidarEstudanteEmailMiddleware(handler.EditarEstudanteHandler(db))(w, r)
		case http.MethodDelete:
			handler.RemoverEstudanteHandler(db)(w, r)
		default:
			http.Error(w, "Método não permitido", http.StatusMethodNotAllowed)
		}
	})))

	// Anos/Turmas (lista + cria)
	mux.Handle("/api/anos", corsMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			handler.ListarAnosHandler(db)(w, r)
		case http.MethodPost:
			handler.CriarAnoHandler(db)(w, r)
		default:
			http.Error(w, "Método não permitido", http.StatusMethodNotAllowed)
		}
	})))

	// Anos/Turmas (delete por id)
	mux.Handle("/api/anos/", corsMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		idStr := strings.TrimPrefix(r.URL.Path, "/api/anos/")
		if idStr == "" {
			http.Error(w, "ID do ano/turma não informado", http.StatusBadRequest)
			return
		}
		if _, err := strconv.Atoi(idStr); err != nil {
			http.Error(w, "ID do ano/turma inválido", http.StatusBadRequest)
			return
		}
		switch r.Method {
		case http.MethodDelete:
			handler.RemoverAnoHandler(db)(w, r)
		default:
			http.Error(w, "Método não permitido", http.StatusMethodNotAllowed)
		}
	})))

	// Uploads locais (se usar)
	mux.Handle("/uploads/", http.StripPrefix("/uploads/", http.FileServer(http.Dir("./uploads"))))

	// Healthcheck simples (útil para Docker/CI)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	// 404 padrão
	mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Endpoint não encontrado", http.StatusNotFound)
	}))
}

/*
===========================
main
===========================
*/
func main() {
	db := conectarBanco()
	defer func() { _ = db.Close() }()

	mux := http.NewServeMux()
	registrarRotas(mux, db)

	server := &http.Server{
		Addr:         ":8080",
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	log.Println("Servidor rodando em http://localhost:8080")

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-quit
		log.Println("Desligando o servidor...")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(ctx); err != nil {
			log.Printf("Erro ao desligar servidor: %v", err)
		}
	}()

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Erro ao iniciar servidor: %v", err)
	}
}
