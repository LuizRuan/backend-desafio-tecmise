// backend/handler/usuario_handler.go
package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/mail"
	"strconv"
	"strings"

	"backend/model"

	"github.com/lib/pq"
	"golang.org/x/crypto/bcrypt"
)

// -----------------------------------------------------------------------------
// 🔹 POST /register
//   - Cadastra novo usuário (nome, email, senha).
//   - Valida dados, confere unicidade (case-insensitive) e salva hash da senha.
//   - Respostas: 201 (ok), 400 (validação), 409 (e-mail já existe), 500 (erro).
//
// -----------------------------------------------------------------------------
func RegisterHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req model.RegisterRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSONError(w, http.StatusBadRequest, "JSON inválido")
			return
		}

		// Normaliza & valida (defensivo, mesmo com middleware)
		req.Sanitize()
		if strings.TrimSpace(req.Nome) == "" || len(req.Nome) < 2 {
			writeJSONError(w, http.StatusBadRequest, "Nome muito curto")
			return
		}
		if _, err := mail.ParseAddress(req.Email); err != nil {
			writeJSONError(w, http.StatusBadRequest, "E-mail inválido")
			return
		}
		// Projeto vinha usando mínimo 8 caracteres
		if len(req.Senha) < 8 || strings.Contains(req.Senha, " ") {
			writeJSONError(w, http.StatusBadRequest, "Senha muito curta (mínimo 8 caracteres e sem espaços)")
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), dbTimeout)
		defer cancel()

		// Confere unicidade (case-insensitive)
		var exists bool
		if err := db.QueryRowContext(ctx,
			`SELECT EXISTS(SELECT 1 FROM usuarios WHERE LOWER(email)=LOWER($1))`, req.Email,
		).Scan(&exists); err != nil {
			writeJSONError(w, http.StatusInternalServerError, "Erro ao verificar e-mail")
			return
		}
		if exists {
			writeJSONError(w, http.StatusConflict, "E-mail já cadastrado")
			return
		}

		hash, err := bcrypt.GenerateFromPassword([]byte(req.Senha), bcrypt.DefaultCost)
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "Erro ao processar senha")
			return
		}

		_, err = db.ExecContext(ctx,
			`INSERT INTO usuarios (nome, email, senha_hash) VALUES ($1, $2, $3)`,
			req.Nome, req.Email, string(hash),
		)
		if err != nil {
			// fallback se o banco tiver unique constraint
			if pqErr, ok := err.(*pq.Error); ok && string(pqErr.Code) == "23505" {
				writeJSONError(w, http.StatusConflict, "E-mail já cadastrado")
				return
			}
			writeJSONError(w, http.StatusInternalServerError, "Erro ao salvar usuário")
			return
		}

		writeJSON(w, http.StatusCreated, map[string]bool{"ok": true})
	}
}

// -----------------------------------------------------------------------------
// 🔹 POST /login
//   - Autentica com email/senha.
//   - Respostas: 200 (dados do usuário), 400/401/500.
//
// -----------------------------------------------------------------------------
func LoginHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req model.LoginRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSONError(w, http.StatusBadRequest, "JSON inválido")
			return
		}
		req.Sanitize()

		if _, err := mail.ParseAddress(req.Email); err != nil {
			writeJSONError(w, http.StatusBadRequest, "E-mail inválido")
			return
		}
		if len(req.Senha) < 8 || strings.Contains(req.Senha, " ") {
			writeJSONError(w, http.StatusBadRequest, "Senha inválida")
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), dbTimeout)
		defer cancel()

		var (
			id     int
			nome   string
			hash   string
			foto   string
			emailQ = req.Email
		)
		err := db.QueryRowContext(ctx, `
			SELECT id, nome, senha_hash, COALESCE(foto_url,'')
			  FROM usuarios
			 WHERE LOWER(email)=LOWER($1)
		`, emailQ).Scan(&id, &nome, &hash, &foto)

		if err == sql.ErrNoRows {
			writeJSONError(w, http.StatusUnauthorized, "E-mail ou senha incorretos")
			return
		}
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "Erro ao verificar usuário")
			return
		}

		if bcrypt.CompareHashAndPassword([]byte(hash), []byte(req.Senha)) != nil {
			writeJSONError(w, http.StatusUnauthorized, "E-mail ou senha incorretos")
			return
		}

		resp := struct {
			ID      int    `json:"id"`
			Nome    string `json:"nome"`
			Email   string `json:"email"`
			FotoURL string `json:"fotoUrl"`
		}{
			ID:      id,
			Nome:    nome,
			Email:   req.Email,
			FotoURL: foto,
		}
		writeJSON(w, http.StatusOK, resp)
	}
}

// -----------------------------------------------------------------------------
// 🔹 PUT /api/usuario/{id}/tutorial
//   - Marca/Desmarca `tutorial_visto`.
//   - Aceita body opcional: { "tutorial_visto": <bool> } (default=true).
//
// -----------------------------------------------------------------------------
func MarcarTutorialVistoHandler(db *sql.DB) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			writeJSONError(w, http.StatusMethodNotAllowed, "Método não permitido")
			return
		}

		// Extrai /api/usuario/{id}/tutorial → {id}
		p := strings.TrimPrefix(r.URL.Path, "/api/usuario/")
		if !strings.HasSuffix(p, "/tutorial") {
			http.NotFound(w, r)
			return
		}
		idStr := strings.Trim(strings.TrimSuffix(p, "/tutorial"), "/")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil || id <= 0 {
			writeJSONError(w, http.StatusBadRequest, "id inválido")
			return
		}

		var body struct {
			TutorialVisto *bool `json:"tutorial_visto"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		val := true
		if body.TutorialVisto != nil {
			val = *body.TutorialVisto
		}

		ctx, cancel := context.WithTimeout(r.Context(), dbTimeout)
		defer cancel()

		res, err := db.ExecContext(ctx,
			`UPDATE usuarios SET tutorial_visto=$1 WHERE id=$2`, val, id,
		)
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "Erro ao atualizar")
			return
		}
		if rows, _ := res.RowsAffected(); rows == 0 {
			writeJSONError(w, http.StatusNotFound, "Usuário não encontrado")
			return
		}

		w.WriteHeader(http.StatusNoContent)
	})
}
