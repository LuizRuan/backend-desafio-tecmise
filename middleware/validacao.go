package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"regexp"
	"strings"
)

/*
===========================================

	Estruturas de request (existentes)
	===========================================
*/
type RegisterRequest struct {
	Nome  string `json:"nome"`
	Email string `json:"email"`
	Senha string `json:"senha"`
}
type LoginRequest struct {
	Email string `json:"email"`
	Senha string `json:"senha"`
}

/*
===========================================

	Middleware para validação de cadastro (existente)
	===========================================
*/
func ValidarCadastroMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req RegisterRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "JSON inválido", http.StatusBadRequest)
			return
		}
		// === Validação Nome ===
		req.Nome = strings.TrimSpace(req.Nome)
		if len(req.Nome) < 2 {
			http.Error(w, "Nome muito curto", http.StatusBadRequest)
			return
		}
		// === Validação E-mail ===
		if req.Email == "" || req.Email != strings.TrimSpace(req.Email) {
			http.Error(w, "E-mail não pode começar ou terminar com espaço!", http.StatusBadRequest)
			return
		}
		if strings.Contains(req.Email, " ") {
			http.Error(w, "E-mail não pode conter espaços!", http.StatusBadRequest)
			return
		}
		emailRe := regexp.MustCompile(`^[\w\-.]+@([\w-]+\.)+[\w-]{2,4}$`)
		if !emailRe.MatchString(req.Email) {
			http.Error(w, "E-mail inválido", http.StatusBadRequest)
			return
		}
		// === Validação Senha ===
		if req.Senha == "" || len(req.Senha) < 8 {
			http.Error(w, "Senha muito curta (mínimo 8 caracteres)", http.StatusBadRequest)
			return
		}
		if strings.Contains(req.Senha, " ") {
			http.Error(w, "Senha não pode conter espaços!", http.StatusBadRequest)
			return
		}
		// --- Passa para o handler ---
		bodyBytes, _ := json.Marshal(req)
		r.Body = io.NopCloser(strings.NewReader(string(bodyBytes)))
		next(w, r)
	}
}

/*
===========================================

	Middleware para validação de login (existente)
	===========================================
*/
func ValidarLoginMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req LoginRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "JSON inválido", http.StatusBadRequest)
			return
		}
		// --- E-mail ---
		if req.Email == "" || req.Email != strings.TrimSpace(req.Email) {
			http.Error(w, "E-mail não pode começar ou terminar com espaço!", http.StatusBadRequest)
			return
		}
		if strings.Contains(req.Email, " ") {
			http.Error(w, "E-mail não pode conter espaços!", http.StatusBadRequest)
			return
		}
		emailRe := regexp.MustCompile(`^[\w\-.]+@([\w-]+\.)+[\w-]{2,4}$`)
		if !emailRe.MatchString(req.Email) {
			http.Error(w, "E-mail inválido", http.StatusBadRequest)
			return
		}
		// --- Senha ---
		if req.Senha == "" || len(req.Senha) < 8 {
			http.Error(w, "Senha deve ter pelo menos 8 caracteres.", http.StatusBadRequest)
			return
		}
		if strings.Contains(req.Senha, " ") {
			http.Error(w, "Senha não pode conter espaços!", http.StatusBadRequest)
			return
		}
		// --- Passa para o handler ---
		bodyBytes, _ := json.Marshal(req)
		r.Body = io.NopCloser(strings.NewReader(string(bodyBytes)))
		next(w, r)
	}
}

/*
=======================================================================

	NOVO: Middleware para Estudante — validação SOMENTE do e-mail do aluno
	- Não valida nome (a seu pedido)
	- Não mexe no CPF (você já valida em outro lugar)
	- Preserva o restante do JSON (nome, cpf, data_nascimento, etc.)
	- Normaliza o e-mail (trim e checa espaços/formato)
	Como usar:
	  mux.Handle("/api/estudantes", corsMiddleware(http.HandlerFunc(func(w,r){
	    switch r.Method {
	      case http.MethodPost:
	        middleware.ValidarEstudanteEmailMiddleware(handler.CriarEstudanteHandler(db))(w,r)
	      // ...
	    }
	  })))
	  mux.Handle("/api/estudantes/", corsMiddleware(http.HandlerFunc(func(w,r){
	    switch r.Method {
	      case http.MethodPut:
	        middleware.ValidarEstudanteEmailMiddleware(handler.EditarEstudanteHandler(db))(w,r)
	      // ...
	    }
	  })))
	=======================================================================
*/
func ValidarEstudanteEmailMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 1) Lê o corpo original inteiro (para não perder campos)
		orig, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Falha ao ler corpo da requisição", http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		// 2) Deserializa em map para preservar TODOS os campos
		var payload map[string]any
		if err := json.Unmarshal(orig, &payload); err != nil {
			http.Error(w, "JSON inválido", http.StatusBadRequest)
			return
		}

		// 3) Extrai e valida o e-mail (aceita "email" camel/snake — mas aqui o campo do aluno é "email")
		rawEmail, _ := payload["email"].(string)
		email := strings.TrimSpace(rawEmail)

		if email == "" {
			http.Error(w, "E-mail do estudante é obrigatório", http.StatusBadRequest)
			return
		}
		if strings.Contains(email, " ") {
			http.Error(w, "E-mail do estudante não pode conter espaços", http.StatusBadRequest)
			return
		}
		emailRe := regexp.MustCompile(`^[\w\-.]+@([\w-]+\.)+[\w-]{2,4}$`)
		if !emailRe.MatchString(email) {
			http.Error(w, "E-mail do estudante inválido", http.StatusBadRequest)
			return
		}

		// 4) Reinsere o e-mail normalizado e remonta o body
		payload["email"] = email
		normBody, _ := json.Marshal(payload)

		// 5) Reatribui o corpo e segue para o handler
		r.Body = io.NopCloser(bytes.NewReader(normBody))
		next(w, r)
	}
}
