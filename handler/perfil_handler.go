// handler/perfil_handler.go
//
// Handlers de perfil de usuário:
// - Atualiza nome e foto (PUT /api/perfil) + senha opcional
// - Busca usuário pelo e-mail (GET /api/usuario?email=...)
//   → retorna também tutorial_visto para o front decidir mostrar (ou não) o tutorial.
//
// Projeto: TecMise

package handler

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

// ========== Atualizar Perfil ==========
// PUT /api/perfil
// Body: { "nome": "...", "foto_url": "...", "fotoUrl": "...", "senha": "..." }  // senha é opcional
// Header: X-User-Email
func AtualizarPerfilHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			http.Error(w, "Método não permitido", http.StatusMethodNotAllowed)
			return
		}

		var req struct {
			Nome    string `json:"nome"`
			FotoURL string `json:"foto_url"` // snake_case
			FotoUrl string `json:"fotoUrl"`  // camelCase
			Senha   string `json:"senha"`    // opcional
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "JSON inválido", http.StatusBadRequest)
			return
		}

		email := strings.TrimSpace(r.Header.Get("X-User-Email"))
		if email == "" {
			http.Error(w, "Usuário não autenticado", http.StatusUnauthorized)
			return
		}
		nome := strings.TrimSpace(req.Nome)
		if len(nome) < 2 {
			http.Error(w, "Nome muito curto", http.StatusBadRequest)
			return
		}

		// foto: prioriza foto_url; se vier vazio e houver fotoUrl, usa ela
		fotoFinal := strings.TrimSpace(req.FotoURL)
		if fotoFinal == "" && req.FotoUrl != "" {
			fotoFinal = strings.TrimSpace(req.FotoUrl)
		}

		// Verifica existência do usuário
		var exists bool
		if err := db.QueryRow(`SELECT EXISTS(SELECT 1 FROM usuarios WHERE email=$1)`, email).Scan(&exists); err != nil || !exists {
			http.Error(w, "Usuário não encontrado", http.StatusNotFound)
			return
		}

		// Monta UPDATE (com ou sem senha)
		if strings.TrimSpace(req.Senha) != "" {
			// validação mínima (mesma linha dos outros handlers)
			s := req.Senha
			if len(s) < 8 || strings.Contains(s, " ") {
				http.Error(w, "Senha inválida (mínimo 8 caracteres e sem espaços)", http.StatusBadRequest)
				return
			}
			hash, err := bcrypt.GenerateFromPassword([]byte(s), bcrypt.DefaultCost)
			if err != nil {
				http.Error(w, "Erro ao processar senha", http.StatusInternalServerError)
				return
			}
			if _, err := db.Exec(
				`UPDATE usuarios SET nome=$1, foto_url=$2, senha_hash=$3 WHERE email=$4`,
				nome, fotoFinal, string(hash), email,
			); err != nil {
				log.Println("[perfil] ERRO update (com senha):", err)
				http.Error(w, "Erro ao atualizar perfil", http.StatusInternalServerError)
				return
			}
		} else {
			if _, err := db.Exec(
				`UPDATE usuarios SET nome=$1, foto_url=$2 WHERE email=$3`,
				nome, fotoFinal, email,
			); err != nil {
				log.Println("[perfil] ERRO update:", err)
				http.Error(w, "Erro ao atualizar perfil", http.StatusInternalServerError)
				return
			}
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}
}

// ========== Buscar Usuário por E-mail ==========
// GET /api/usuario?email=...
// Retorna também tutorial_visto para o front bloquear o tutorial após a 1ª vez.
func BuscarUsuarioPorEmailHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		email := strings.TrimSpace(r.URL.Query().Get("email"))
		if email == "" {
			http.Error(w, "E-mail não informado", http.StatusBadRequest)
			return
		}

		var user struct {
			ID            int    `json:"id"`
			Nome          string `json:"nome"`
			Email         string `json:"email"`
			FotoUrl       string `json:"fotoUrl"`
			TutorialVisto bool   `json:"tutorial_visto"`
		}

		err := db.QueryRow(`
				SELECT id,
					nome,
					email,
					COALESCE(foto_url, ''),
					COALESCE(tutorial_visto, false)
				FROM usuarios
				WHERE email=$1
			`, email).Scan(&user.ID, &user.Nome, &user.Email, &user.FotoUrl, &user.TutorialVisto)

		if err != nil {
			if err == sql.ErrNoRows {
				http.Error(w, "Usuário não encontrado", http.StatusNotFound)
			} else {
				log.Println("[perfil] ERRO select:", err)
				http.Error(w, "Erro ao buscar usuário", http.StatusInternalServerError)
			}
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(user)
	}
}
