/*
/// Projeto: Tecmise
/// Arquivo: main.go
/// Responsabilidade: Ponto de entrada do backend HTTP (Go), configuração de infraestrutura (DB, middlewares, CORS, rotas) e graceful shutdown.
/// Dependências principais: net/http, database/sql (Postgres), github.com/joho/godotenv, github.com/lib/pq, pacotes locais (handler, middleware, model).
/// Pontos de atenção:
/// - CORS: somente "Content-Type, X-User-Email" permitidos; se futuramente usar Authorization/Bearer ou credenciais, ajustar cabeçalhos.
/// - Wildcard CORS ("*"): quando Origin presente, estratégia atual espelha o Origin ao invés de usar "*".
/// - godotenv.Load() é chamado no main e também em conectarBanco() (carregamento duplicado; aceitável, porém redundante).
/// - Fechamento do DB ocorre via defer e também em RegisterOnShutdown (fechamento duplicado; seguro, porém redundante).
/// - recoverMiddleware registra apenas o valor do panic, sem stack trace detalhado.
/// - Rotas com parsing manual (e.g., /api/usuario/{id}/tutorial) exigem cuidado com sufixos e validações.
/// - Segurança de cabeçalhos: X-Frame-Options=DENY; X-XSS-Protection=0; CSP não configurado aqui (pode ser tratado por proxy/reverse).
*/

// main.go — ponto de entrada (resumo para foco no ajuste do repo do Google)
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
	"backend/model" // << usa o repo no package model

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

/// ============ Funções Internas (helpers) ============

// getEnv retorna o valor de uma variável de ambiente ou um padrão se não definido.
// Parâmetros:
//   - key: nome da variável
//   - def: valor padrão
//
// Retorno: string com o valor encontrado ou def.
func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// getEnvAsInt retorna uma env como inteiro, fallback para def em caso de ausência/erro.
// Parâmetros:
//   - key: nome da variável
//   - def: valor padrão
//
// Retorno: int com o valor convertido ou def.
func getEnvAsInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

// getEnvAsDuration retorna uma env parseada como time.Duration (ex: "5m", "30s").
// Parâmetros:
//   - key: nome da variável
//   - def: valor padrão
//
// Retorno: time.Duration com o valor convertido ou def.
func getEnvAsDuration(key string, def time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return def
}

/// ============ Middlewares ============

// apply encadeia middlewares do último para o primeiro sobre um http.Handler.
// Parâmetros:
//   - h: handler base
//   - mws: lista de middlewares (func(http.Handler) http.Handler)
//
// Retorno: http.Handler resultante após aplicação em cadeia.
func apply(h http.Handler, mws ...func(http.Handler) http.Handler) http.Handler {
	for i := len(mws) - 1; i >= 0; i-- {
		h = mws[i](h)
	}
	return h
}

// corsMiddleware aplica regras CORS com base na env CORS_ALLOW_ORIGINS (lista separada por vírgula).
// - Se "*" e Origin ausente: define Access-Control-Allow-Origin: *.
// - Se Origin presente e permitido: espelha o Origin.
// - Allow-Methods: GET, POST, PUT, DELETE, OPTIONS
// - Allow-Headers: Content-Type, X-User-Email
// Observação: Não habilita credenciais (sem Access-Control-Allow-Credentials).
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
			w.Header().Set("Access-Control-Allow-Origin", "*")
		}
		if origin != "" && isAllowed(origin) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
		}
		w.Header().Set("Vary", "Origin")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-User-Email")
		w.Header().Set("Access-Control-Max-Age", "86400")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// securityHeadersMiddleware adiciona cabeçalhos de segurança básicos.
// - X-Content-Type-Options: nosniff
// - X-Frame-Options: DENY
// - X-XSS-Protection: 0 (desabilita filtro legado)
// Observação: Política de Conteúdo (CSP) pode ser configurada em camada superior (proxy).
func securityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-XSS-Protection", "0")
		next.ServeHTTP(w, r)
	})
}

// recoverMiddleware captura panics e responde 500 com log de erro.
// Observação: Apenas registra o valor do panic; para stack trace, considerar runtime/debug.PrintStack.
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

/// ============ Banco de Dados ============

// conectarBanco inicializa conexão com Postgres a partir de DATABASE_URL (.env/env).
// Efeitos colaterais: carrega .env, abre pool, faz ping de verificação e configura pool.
// Falhas: log.Fatal em erros críticos (encerra o processo).
func conectarBanco() *sql.DB {
	_ = godotenv.Load()
	connStr := os.Getenv("DATABASE_URL")
	if connStr == "" {
		log.Fatal("DATABASE_URL não setada no .env")
	}
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatal("Erro ao abrir conexão:", err)
	}
	if err = db.Ping(); err != nil {
		log.Fatal("Não foi possível conectar ao banco:", err)
	}
	db.SetMaxOpenConns(getEnvAsInt("DB_MAX_OPEN_CONNS", 10))
	db.SetMaxIdleConns(getEnvAsInt("DB_MAX_IDLE_CONNS", 5))
	db.SetConnMaxLifetime(getEnvAsDuration("DB_CONN_MAX_LIFETIME", 5*time.Minute))
	log.Println("Conectado ao banco de dados!")
	return db
}

/// ============ Rotas & Handlers ============

// registrarRotas mapeia endpoints na mux com middlewares padrão.
// Parâmetros:
//   - mux: *http.ServeMux alvo
//   - db: *sql.DB para injeção nos handlers
//
// Rotas principais: /register, /login, /login/google, /api/*, estáticos (/uploads), /healthz, fallback 404.
func registrarRotas(mux *http.ServeMux, db *sql.DB) {
	defaultMW := []func(http.Handler) http.Handler{recoverMiddleware, securityHeadersMiddleware, corsMiddleware}

	// Auth tradicional
	mux.Handle("/register", apply(handler.RegisterHandler(db), defaultMW...))
	mux.Handle("/login", apply(handler.LoginHandler(db), defaultMW...))

	// Google Login
	userRepo := model.NewUserRepo(db)
	googleH := handler.NewAuthGoogleHandler(userRepo)
	mux.Handle("/login/google", apply(http.HandlerFunc(googleH.LoginGoogle), defaultMW...))

	// Perfil / Usuário
	mux.Handle("/api/perfil", apply(handler.AtualizarPerfilHandler(db), defaultMW...))
	mux.Handle("/api/usuario", apply(handler.BuscarUsuarioPorEmailHandler(db), defaultMW...))
	mux.Handle("/api/usuario/", apply(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/api/usuario/")
		parts := strings.Split(strings.Trim(path, "/"), "/")
		if len(parts) == 2 && parts[1] == "tutorial" && r.Method == http.MethodPut {
			handler.MarcarTutorialVistoHandler(db).ServeHTTP(w, r)
			return
		}
		http.NotFound(w, r)
	}), defaultMW...))

	// Validações
	mux.Handle("/api/estudantes/check-cpf", apply(handler.VerificarCpfHandler(db), defaultMW...))
	mux.Handle("/api/estudantes/check-email", apply(handler.VerificarEmailHandler(db), defaultMW...))

	// Estudantes
	mux.Handle("/api/estudantes", apply(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			handler.ListarEstudantesHandler(db)(w, r)
		case http.MethodPost:
			middleware.ValidarEstudanteEmailMiddleware(handler.CriarEstudanteHandler(db))(w, r)
		default:
			http.Error(w, "Método não permitido", http.StatusMethodNotAllowed)
		}
	}), defaultMW...))
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

	// Anos
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
		if r.Method == http.MethodDelete {
			handler.RemoverAnoHandler(db)(w, r)
			return
		}
		http.Error(w, "Método não permitido", http.StatusMethodNotAllowed)
	}), defaultMW...))

	// estáticos e health
	if fi, err := os.Stat("./uploads"); err == nil && fi.IsDir() {
		mux.Handle("/uploads/", http.StripPrefix("/uploads/", http.FileServer(http.Dir("./uploads"))))
	}
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Endpoint não encontrado", http.StatusNotFound)
	}))
}

/// ============ Inicialização/Bootstrap ============

// main inicializa configuração via .env, conecta no banco, registra rotas e inicia HTTP server.
// Implementa graceful shutdown em SIGINT/SIGTERM com timeout configurável via HTTP_SHUTDOWN_TIMEOUT.
// Logs básicos informam porta e eventos de desligamento.
func main() {
	if err := godotenv.Load(".env"); err != nil {
		log.Println("(.env) não encontrado; seguindo com variáveis do ambiente")
	}
	db := conectarBanco()
	defer func() { _ = db.Close() }()

	mux := http.NewServeMux()
	registrarRotas(mux, db)

	port := getEnv("PORT", "8080")
	server := &http.Server{
		Addr: ":" + port, Handler: mux,
		ReadTimeout:       getEnvAsDuration("HTTP_READ_TIMEOUT", 10*time.Second),
		ReadHeaderTimeout: getEnvAsDuration("HTTP_READ_HEADER_TIMEOUT", 5*time.Second),
		WriteTimeout:      getEnvAsDuration("HTTP_WRITE_TIMEOUT", 15*time.Second),
		IdleTimeout:       getEnvAsDuration("HTTP_IDLE_TIMEOUT", 60*time.Second),
	}
	log.Printf("Servidor rodando em http://localhost:%s", port)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	server.RegisterOnShutdown(func() { _ = db.Close() })
	go func() {
		<-quit
		log.Println("Desligando o servidor...")
		ctx, cancel := context.WithTimeout(context.Background(), getEnvAsDuration("HTTP_SHUTDOWN_TIMEOUT", 10*time.Second))
		defer cancel()
		if err := server.Shutdown(ctx); err != nil {
			log.Printf("Erro ao desligar servidor: %v", err)
		}
	}()
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Erro ao iniciar servidor: %v", err)
	}
}
