// main.go
//
// 🔧 Ponto de entrada do backend TecMise (HTTP + PostgreSQL)
// Mantém o comportamento atual, mas com melhorias de organização,
// middlewares encadeáveis, CORS configurável por ambiente e shutdown
// mais robusto.

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

//
// ==============================
// Helpers de configuração (.env)
// ==============================
//

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getEnvAsInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func getEnvAsDuration(key string, def time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return def
}

//
// ===================
// Middlewares comuns
// ===================
//

// Encadeia middlewares (o último na lista roda mais "externo")
func apply(h http.Handler, mws ...func(http.Handler) http.Handler) http.Handler {
	for i := len(mws) - 1; i >= 0; i-- {
		h = mws[i](h)
	}
	return h
}

// CORS simples com controle por ambiente
// CORS_ALLOW_ORIGINS="*"                → libera tudo (dev)
// CORS_ALLOW_ORIGINS="http://a.com,... "→ lista de origens permitidas
func corsMiddleware(next http.Handler) http.Handler {
	allowed := strings.Split(strings.TrimSpace(getEnv("CORS_ALLOW_ORIGINS", "*")), ",")
	for i := range allowed {
		allowed[i] = strings.TrimSpace(allowed[i])
	}

	isAllowed := func(origin string) bool {
		if len(allowed) == 0 {
			return false
		}
		if allowed[0] == "*" {
			return true
		}
		for _, o := range allowed {
			if o == origin {
				return true
			}
		}
		return false
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin == "" && len(allowed) == 1 && allowed[0] == "*" {
			// sem origem (ex.: curl/healthz) e modo aberto
			w.Header().Set("Access-Control-Allow-Origin", "*")
		} else if origin != "" && isAllowed(origin) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
		}

		w.Header().Set("Vary", "Origin")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-User-Email")
		w.Header().Set("Access-Control-Max-Age", "86400") // 24h

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// Cabeçalhos de segurança básicos
func securityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-XSS-Protection", "0")
		next.ServeHTTP(w, r)
	})
}

// Protege contra panic em handlers
func recoverMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				log.Printf("panic: %v", rec)
				http.Error(w, "erro interno", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

//
// =====================
// Conexão com o banco
// =====================
//

func conectarBanco() *sql.DB {
	// Carrega .env (silencioso se não existir)
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

	// Pool (parametrizável por env)
	db.SetMaxOpenConns(getEnvAsInt("DB_MAX_OPEN_CONNS", 10))
	db.SetMaxIdleConns(getEnvAsInt("DB_MAX_IDLE_CONNS", 5))
	db.SetConnMaxLifetime(getEnvAsDuration("DB_CONN_MAX_LIFETIME", 5*time.Minute))

	log.Println("Conectado ao banco de dados!")
	return db
}

//
// ==================
// Registro de rotas
// ==================
//

func registrarRotas(mux *http.ServeMux, db *sql.DB) {
	// Middlewares padrão aplicados em (quase) todas as rotas
	defaultMW := []func(http.Handler) http.Handler{
		recoverMiddleware,
		securityHeadersMiddleware,
		corsMiddleware,
	}

	// ---------- Autenticação ----------
	mux.Handle("/register", apply(handler.RegisterHandler(db), defaultMW...))
	mux.Handle("/login", apply(handler.LoginHandler(db), defaultMW...))

	// ---------- Perfil / Usuário ----------
	mux.Handle("/api/perfil", apply(handler.AtualizarPerfilHandler(db), defaultMW...))
	mux.Handle("/api/usuario", apply(handler.BuscarUsuarioPorEmailHandler(db), defaultMW...))

	// /api/usuario/{id}/tutorial (PUT)
	mux.Handle("/api/usuario/", apply(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/api/usuario/")
		parts := strings.Split(strings.Trim(path, "/"), "/")
		if len(parts) == 2 && parts[1] == "tutorial" && r.Method == http.MethodPut {
			handler.MarcarTutorialVistoHandler(db).ServeHTTP(w, r)
			return
		}
		http.NotFound(w, r)
	}), defaultMW...))

	// ---------- Validações ----------
	mux.Handle("/api/estudantes/check-cpf", apply(handler.VerificarCpfHandler(db), defaultMW...))
	mux.Handle("/api/estudantes/check-email", apply(handler.VerificarEmailHandler(db), defaultMW...))

	// ---------- Estudantes ----------
	// /api/estudantes (GET/POST)
	mux.Handle("/api/estudantes", apply(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			handler.ListarEstudantesHandler(db)(w, r)
		case http.MethodPost:
			// mantém o middleware de validação existente
			middleware.ValidarEstudanteEmailMiddleware(handler.CriarEstudanteHandler(db))(w, r)
		default:
			http.Error(w, "Método não permitido", http.StatusMethodNotAllowed)
		}
	}), defaultMW...))

	// /api/estudantes/{id} (PUT/DELETE)
	mux.Handle("/api/estudantes/", apply(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
			middleware.ValidarEstudanteEmailMiddleware(handler.EditarEstudanteHandler(db))(w, r)
		case http.MethodDelete:
			handler.RemoverEstudanteHandler(db)(w, r)
		default:
			http.Error(w, "Método não permitido", http.StatusMethodNotAllowed)
		}
	}), defaultMW...))

	// ---------- Anos/Turmas ----------
	// /api/anos (GET/POST)
	mux.Handle("/api/anos", apply(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			handler.ListarAnosHandler(db)(w, r)
		case http.MethodPost:
			handler.CriarAnoHandler(db)(w, r)
		default:
			http.Error(w, "Método não permitido", http.StatusMethodNotAllowed)
		}
	}), defaultMW...))

	// /api/anos/{id} (DELETE)
	mux.Handle("/api/anos/", apply(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	}), defaultMW...))

	// ---------- Estáticos / Utilidades ----------
	// Servir uploads locais (se a pasta existir)
	if fi, err := os.Stat("./uploads"); err == nil && fi.IsDir() {
		mux.Handle("/uploads/", http.StripPrefix("/uploads/", http.FileServer(http.Dir("./uploads"))))
	}

	// Healthcheck simples (para Docker/CI/K8s)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	// 404 padrão para demais rotas
	mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Endpoint não encontrado", http.StatusNotFound)
	}))
}

//
// =====
// main
// =====
//

func main() {
	// 1) Banco
	db := conectarBanco()
	defer func() { _ = db.Close() }()

	// 2) Router
	mux := http.NewServeMux()
	registrarRotas(mux, db)

	// 3) Servidor HTTP com timeouts (parametrizáveis)
	port := getEnv("PORT", "8080")
	server := &http.Server{
		Addr:              ":" + port,
		Handler:           mux,
		ReadTimeout:       getEnvAsDuration("HTTP_READ_TIMEOUT", 10*time.Second),
		ReadHeaderTimeout: getEnvAsDuration("HTTP_READ_HEADER_TIMEOUT", 5*time.Second),
		WriteTimeout:      getEnvAsDuration("HTTP_WRITE_TIMEOUT", 15*time.Second),
		IdleTimeout:       getEnvAsDuration("HTTP_IDLE_TIMEOUT", 60*time.Second),
	}

	log.Printf("Servidor rodando em http://localhost:%s", port)

	// 4) Graceful shutdown: captura SIGINT/SIGTERM
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// fecha recursos no desligamento
	server.RegisterOnShutdown(func() {
		_ = db.Close()
	})

	go func() {
		<-quit
		log.Println("Desligando o servidor...")
		ctx, cancel := context.WithTimeout(context.Background(), getEnvAsDuration("HTTP_SHUTDOWN_TIMEOUT", 10*time.Second))
		defer cancel()
		if err := server.Shutdown(ctx); err != nil {
			log.Printf("Erro ao desligar servidor: %v", err)
		}
	}()

	// 5) Start bloqueante
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Erro ao iniciar servidor: %v", err)
	}
}
