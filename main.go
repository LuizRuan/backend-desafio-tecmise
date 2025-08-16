// main.go
//
// 🔧 Ponto de entrada do backend TecMise (HTTP + PostgreSQL)
//
// O que este arquivo faz:
// 1) Carrega variáveis de ambiente (.env) e conecta ao Postgres.
// 2) Registra todas as rotas/handlers (auth, perfil, estudantes, anos).
// 3) Sobe um servidor HTTP com CORS (modo dev) e timeouts.
// 4) Implementa graceful shutdown (SIGINT/SIGTERM).
//
// Dependências diretas (ver go.mod):
// - github.com/joho/godotenv: carregar .env
// - github.com/lib/pq: driver PostgreSQL
//
// Portas/URLs padrão:
// - Servidor: http://localhost:8080
// - Healthcheck: GET /healthz

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
===============================================================================

	CORS (modo dev)

-------------------------------------------------------------------------------

	Em desenvolvimento deixamos CORS permissivo para facilitar o front local.
	Em produção, **recomenda-se restringir** o `Access-Control-Allow-Origin` para
	o(s) domínio(s) do seu frontend.

	Como usar:
	  mux.Handle("/rota", corsMiddleware(handler))

===============================================================================
*/
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// ⛳️ Ajuste `*` para o domínio do seu front em produção.
		w.Header().Set("Access-Control-Allow-Origin", "*")
		// Métodos aceitos
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		// Cabeçalhos aceitos (inclui nosso header de pseudo-auth)
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-User-Email")
		// Cache do preflight (OPTIONS)
		w.Header().Set("Access-Control-Max-Age", "86400") // 24h

		// Responde preflight sem passar adiante
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}

/*
===============================================================================

	Conexão com o banco

-------------------------------------------------------------------------------

  - Lê DATABASE_URL do .env (ex.: postgres://user:pass@host:5432/db?sslmode=disable)

  - Abre o pool `*sql.DB`, valida com `Ping()` e configura limites básicos.

    Boas práticas:

  - Em produção, ajuste pool conforme sua infra (máx conexões, idle, lifetime).

  - Trate secrets via variáveis de ambiente (não commitar .env).

===============================================================================
*/
func conectarBanco() *sql.DB {
	// Carrega .env (silencioso se não existir, OK em produção)
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

	// 🔧 Pool básico (ajuste esses valores conforme seu ambiente)
	db.SetMaxOpenConns(10)                 // máx conexões abertas
	db.SetMaxIdleConns(5)                  // máx conexões ociosas
	db.SetConnMaxLifetime(5 * time.Minute) // reciclagem de conexões

	log.Println("Conectado ao banco de dados!")
	return db
}

/*
===============================================================================

	Registro de Rotas

-------------------------------------------------------------------------------

	Centraliza toda a definição de endpoints. Cada rota é envelopada pelo CORS
	e chama os handlers específicos. Alguns endpoints usam middlewares de validação.

===============================================================================
*/
func registrarRotas(mux *http.ServeMux, db *sql.DB) {
	// ----------------------- Autenticação -----------------------
	// POST /register → cria usuário (nome, email, senha)
	mux.Handle("/register", corsMiddleware(handler.RegisterHandler(db)))
	// POST /login → autentica e retorna dados básicos
	mux.Handle("/login", corsMiddleware(handler.LoginHandler(db)))

	// -------------------- Perfil / Usuário ----------------------
	// PUT  /api/perfil      → atualiza nome/foto (e senha opcional)
	mux.Handle("/api/perfil", corsMiddleware(handler.AtualizarPerfilHandler(db)))
	// GET  /api/usuario?email=... → obtém dados do usuário + tutorial_visto
	mux.Handle("/api/usuario", corsMiddleware(handler.BuscarUsuarioPorEmailHandler(db)))

	// PUT /api/usuario/{id}/tutorial → marca tutorial_visto (true/false)
	mux.Handle("/api/usuario/", corsMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Espera exatamente: /api/usuario/{id}/tutorial
		path := strings.TrimPrefix(r.URL.Path, "/api/usuario/")
		parts := strings.Split(strings.Trim(path, "/"), "/")
		if len(parts) == 2 && parts[1] == "tutorial" && r.Method == http.MethodPut {
			handler.MarcarTutorialVistoHandler(db).ServeHTTP(w, r)
			return
		}
		http.NotFound(w, r)
	})))

	// --------------------- Validações (GET) ---------------------
	// GET /api/estudantes/check-cpf?cpf=...&ignoreId=...
	mux.Handle("/api/estudantes/check-cpf", corsMiddleware(handler.VerificarCpfHandler(db)))
	// GET /api/estudantes/check-email?email=...&ignoreId=...
	mux.Handle("/api/estudantes/check-email", corsMiddleware(handler.VerificarEmailHandler(db)))

	// ------------------------ Estudantes ------------------------
	// /api/estudantes
	//   GET  → lista do usuário autenticado (via header X-User-Email)
	//   POST → cria (valida e-mail do aluno via middleware sem perder o payload)
	mux.Handle("/api/estudantes", corsMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			handler.ListarEstudantesHandler(db)(w, r)
		case http.MethodPost:
			middleware.ValidarEstudanteEmailMiddleware(handler.CriarEstudanteHandler(db))(w, r)
		default:
			http.Error(w, "Método não permitido", http.StatusMethodNotAllowed)
		}
	})))

	// /api/estudantes/{id}
	//   PUT    → edita (também valida e-mail do aluno)
	//   DELETE → remove
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
			middleware.ValidarEstudanteEmailMiddleware(handler.EditarEstudanteHandler(db))(w, r)
		case http.MethodDelete:
			handler.RemoverEstudanteHandler(db)(w, r)
		default:
			http.Error(w, "Método não permitido", http.StatusMethodNotAllowed)
		}
	})))

	// ------------------------- Anos/Turmas ----------------------
	// /api/anos
	//   GET  → lista anos do usuário
	//   POST → cria novo ano para o usuário
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

	// /api/anos/{id}
	//   DELETE → remove ano (e estudantes vinculados daquele usuário)
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

	// -------------------- Estáticos / Utilidades ----------------
	// Servir uploads locais (se aplicável)
	mux.Handle("/uploads/", http.StripPrefix("/uploads/", http.FileServer(http.Dir("./uploads"))))

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

/*
===============================================================================

	main

-------------------------------------------------------------------------------
  - Conecta ao banco e registra rotas no *http.ServeMux.
  - Configura timeouts do servidor.
  - Inicia o HTTP server e implementa graceful shutdown (SIGINT/SIGTERM).

===============================================================================
*/
func main() {
	// 1) Banco
	db := conectarBanco()
	defer func() { _ = db.Close() }()

	// 2) Router
	mux := http.NewServeMux()
	registrarRotas(mux, db)

	// 3) Servidor HTTP com timeouts (resiliência/segurança)
	server := &http.Server{
		Addr:         ":8080",          // porta do backend
		Handler:      mux,              // multiplexer com nossas rotas
		ReadTimeout:  10 * time.Second, // limite leitura request
		WriteTimeout: 15 * time.Second, // limite escrita response
		IdleTimeout:  60 * time.Second, // keep-alive
	}

	log.Println("Servidor rodando em http://localhost:8080")

	// 4) Graceful shutdown: captura SIGINT/SIGTERM e encerra com timeout
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

	// 5) Start bloqueante (sai apenas por erro ou shutdown)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Erro ao iniciar servidor: %v", err)
	}
}
