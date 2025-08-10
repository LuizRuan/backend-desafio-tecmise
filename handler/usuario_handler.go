// backend/handler/usuario_handler.go
package handler

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

/* =========================
   Tipos (nomes exclusivos p/ evitar colisão)
   ========================= */

type registerRequest struct {
	Nome  string `json:"nome"`
	Email string `json:"email"`
	Senha string `json:"senha"`
}

type loginRequest struct {
	Email string `json:"email"`
	Senha string `json:"senha"`
}

type LoginUserResponse struct {
	ID      int    `json:"id"`
	Nome    string `json:"nome"`
	Email   string `json:"email"`
	FotoURL string `json:"fotoUrl"`
}

// usado em GET /api/usuario?email=...
type UsuarioInfo struct {
	ID            int64  `json:"id"`
	Nome          string `json:"nome"`
	Email         string `json:"email"`
	FotoURL       string `json:"fotoUrl"`
	TutorialVisto bool   `json:"tutorial_visto"`
}

/*
=========================
/register
=========================
*/
func RegisterHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req registerRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "JSON inválido", http.StatusBadRequest)
			return
		}

		req.Nome = strings.TrimSpace(req.Nome)
		if len(req.Nome) < 2 {
			http.Error(w, "Nome muito curto", http.StatusBadRequest)
			return
		}

		if req.Email == "" || req.Email != strings.TrimSpace(req.Email) {
			http.Error(w, "E-mail não pode ter espaço no início/fim", http.StatusBadRequest)
			return
		}
		if strings.Contains(req.Email, " ") {
			http.Error(w, "E-mail não pode conter espaços", http.StatusBadRequest)
			return
		}
		emailRe := regexp.MustCompile(`^[\w\-.]+@([\w-]+\.)+[\w-]{2,4}$`)
		if !emailRe.MatchString(req.Email) {
			http.Error(w, "E-mail inválido", http.StatusBadRequest)
			return
		}

		if req.Senha == "" || len(req.Senha) < 8 {
			http.Error(w, "Senha muito curta (mínimo 8)", http.StatusBadRequest)
			return
		}
		if strings.Contains(req.Senha, " ") {
			http.Error(w, "Senha não pode conter espaços", http.StatusBadRequest)
			return
		}

		// duplicidade
		var count int
		if err := db.QueryRow(`SELECT COUNT(*) FROM usuarios WHERE email=$1`, req.Email).Scan(&count); err != nil {
			http.Error(w, "Erro ao verificar e-mail", http.StatusInternalServerError)
			return
		}
		if count > 0 {
			http.Error(w, "E-mail já cadastrado", http.StatusConflict)
			return
		}

		hash, err := bcrypt.GenerateFromPassword([]byte(req.Senha), bcrypt.DefaultCost)
		if err != nil {
			http.Error(w, "Erro ao processar senha", http.StatusInternalServerError)
			return
		}

		// se sua coluna tutorial_visto tiver default false, não precisa passar
		if _, err := db.Exec(`
			INSERT INTO usuarios (nome, email, senha_hash)
			VALUES ($1, $2, $3)
		`, req.Nome, req.Email, string(hash)); err != nil {
			http.Error(w, "Erro ao salvar usuário", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"ok":true}`))
	}
}

/*
=========================
/login
=========================
*/
func LoginHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req loginRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "JSON inválido", http.StatusBadRequest)
			return
		}
		if req.Email == "" || strings.Contains(req.Email, " ") {
			http.Error(w, "E-mail inválido", http.StatusBadRequest)
			return
		}
		if req.Senha == "" || len(req.Senha) < 8 || strings.Contains(req.Senha, " ") {
			http.Error(w, "Senha inválida", http.StatusBadRequest)
			return
		}

		var id int
		var nome, hash, fotoURL string
		err := db.QueryRow(`
			SELECT id, nome, senha_hash, COALESCE(foto_url,'')
			  FROM usuarios
			 WHERE email=$1
		`, req.Email).Scan(&id, &nome, &hash, &fotoURL)
		if err == sql.ErrNoRows {
			http.Error(w, "E-mail ou senha incorretos", http.StatusUnauthorized)
			return
		}
		if err != nil {
			http.Error(w, "Erro ao verificar usuário", http.StatusInternalServerError)
			return
		}
		if bcrypt.CompareHashAndPassword([]byte(hash), []byte(req.Senha)) != nil {
			http.Error(w, "E-mail ou senha incorretos", http.StatusUnauthorized)
			return
		}

		resp := LoginUserResponse{
			ID:      id,
			Nome:    nome,
			Email:   req.Email,
			FotoURL: fotoURL,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}

/*
=========================================================
PUT /api/usuario/{id}/tutorial   (marca tutorial_visto)
=========================================================
*/
func MarcarTutorialVistoHandler(db *sql.DB) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Só aceite PUT
		if r.Method != http.MethodPut {
			http.Error(w, "Método não permitido", http.StatusMethodNotAllowed)
			return
		}

		// caminho: /api/usuario/{id}/tutorial
		p := strings.TrimPrefix(r.URL.Path, "/api/usuario/")
		if !strings.HasSuffix(p, "/tutorial") {
			http.NotFound(w, r)
			return
		}
		idStr := strings.TrimSuffix(p, "/tutorial")
		idStr = strings.Trim(idStr, "/")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil || id <= 0 {
			http.Error(w, "id inválido", http.StatusBadRequest)
			return
		}

		// Corpo opcional — se não vier, marcamos como true por padrão
		var body struct {
			TutorialVisto *bool `json:"tutorial_visto"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		val := true
		if body.TutorialVisto != nil {
			val = *body.TutorialVisto
		}

		if _, err := db.Exec(`UPDATE usuarios SET tutorial_visto=$1 WHERE id=$2`, val, id); err != nil {
			http.Error(w, "erro ao atualizar", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
}
