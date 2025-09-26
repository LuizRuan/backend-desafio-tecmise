//
// ======================================================================
// 📌 handler/perfil_handler.go
// ======================================================================
// 🎯 Responsabilidade
//    - Atualiza nome/foto e, opcionalmente, a senha do usuário logado.
//    - Busca dados do usuário por e-mail (inclui `tutorial_visto`).
//
// 🔒 Autenticação
//    - PUT /api/perfil exige header `X-User-Email`.
//
// 🧱 Banco
//    - Tabela `usuarios`: id, nome, email, foto_url, senha_hash, tutorial_visto.
//
// 💡 Notas
//    - Reutiliza helpers `writeJSON` e `writeJSONError` já definidos no package.
//    - Usa `dbTimeout` (definido no package) para operações de banco.
//    - Usa `model.MinPasswordLen` para validar a senha.
// ======================================================================
//

package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"

	"backend/model"

	"golang.org/x/crypto/bcrypt"
)

// ======================================================================
// 🔄 Atualizar Perfil
// ----------------------------------------------------------------------
// PUT /api/perfil
//
// Body JSON (senha é opcional):
//
//	{ "nome": "...", "foto_url": "...", "fotoUrl": "...", "senha": "..." }
//
// Regras:
//   - Nome >= 2 caracteres
//   - Se senha vier preenchida: >= model.MinPasswordLen e sem espaços
//
// ======================================================================
func AtualizarPerfilHandler(db *sql.DB) http.HandlerFunc {
	type perfilInput struct {
		Nome    string `json:"nome"`
		FotoURL string `json:"foto_url"` // snake_case
		FotoUrl string `json:"fotoUrl"`  // camelCase (compat)
		Senha   string `json:"senha"`    // opcional
	}

	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			writeJSONError(w, http.StatusMethodNotAllowed, "Método não permitido")
			return
		}

		// Autenticação via header
		email := strings.TrimSpace(strings.ToLower(r.Header.Get("X-User-Email")))
		if email == "" {
			writeJSONError(w, http.StatusUnauthorized, "Usuário não autenticado")
			return
		}

		// Decodifica JSON
		var req perfilInput
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSONError(w, http.StatusBadRequest, "JSON inválido")
			return
		}

		// Validações
		nome := strings.TrimSpace(req.Nome)
		if len(nome) < 2 {
			writeJSONError(w, http.StatusBadRequest, "Nome muito curto")
			return
		}

		// Foto: prioriza `foto_url`; se vazio e existir `fotoUrl`, usa-a
		fotoFinal := strings.TrimSpace(req.FotoURL)
		if fotoFinal == "" && strings.TrimSpace(req.FotoUrl) != "" {
			fotoFinal = strings.TrimSpace(req.FotoUrl)
		}

		ctx, cancel := context.WithTimeout(r.Context(), dbTimeout)
		defer cancel()

		// Se senha foi enviada, validar e atualizar com hash
		if s := strings.TrimSpace(req.Senha); s != "" {
			if len(s) < model.MinPasswordLen || strings.Contains(s, " ") {
				writeJSONError(
					w,
					http.StatusBadRequest,
					"Senha inválida (mínimo "+strconv.Itoa(model.MinPasswordLen)+" caracteres e sem espaços)",
				)
				return
			}
			hash, err := bcrypt.GenerateFromPassword([]byte(s), bcrypt.DefaultCost)
			if err != nil {
				writeJSONError(w, http.StatusInternalServerError, "Erro ao processar senha")
				return
			}

			res, err := db.ExecContext(ctx,
				`UPDATE usuarios SET nome=$1, foto_url=$2, senha_hash=$3 WHERE LOWER(email)=LOWER($4)`,
				nome, fotoFinal, string(hash), email,
			)
			if err != nil {
				log.Println("[perfil] ERRO update (com senha):", err)
				writeJSONError(w, http.StatusInternalServerError, "Erro ao atualizar perfil")
				return
			}
			if rows, _ := res.RowsAffected(); rows == 0 {
				writeJSONError(w, http.StatusNotFound, "Usuário não encontrado")
				return
			}
		} else {
			// Atualiza sem senha
			res, err := db.ExecContext(ctx,
				`UPDATE usuarios SET nome=$1, foto_url=$2 WHERE LOWER(email)=LOWER($3)`,
				nome, fotoFinal, email,
			)
			if err != nil {
				log.Println("[perfil] ERRO update:", err)
				writeJSONError(w, http.StatusInternalServerError, "Erro ao atualizar perfil")
				return
			}
			if rows, _ := res.RowsAffected(); rows == 0 {
				writeJSONError(w, http.StatusNotFound, "Usuário não encontrado")
				return
			}
		}

		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	}
}

// ======================================================================
// 🔎 Buscar Usuário por E-mail
// ----------------------------------------------------------------------
// GET /api/usuario?email=...
//
// Retorna: { id, nome, email, fotoUrl, tutorial_visto }
// ======================================================================
func BuscarUsuarioPorEmailHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		email := strings.TrimSpace(r.URL.Query().Get("email"))
		if email == "" {
			writeJSONError(w, http.StatusBadRequest, "E-mail não informado")
			return
		}

		// Struct de resposta
		var user struct {
			ID            int    `json:"id"`
			Nome          string `json:"nome"`
			Email         string `json:"email"`
			FotoUrl       string `json:"fotoUrl"`
			TutorialVisto bool   `json:"tutorial_visto"`
		}

		ctx, cancel := context.WithTimeout(r.Context(), dbTimeout)
		defer cancel()

		err := db.QueryRowContext(ctx, `
			SELECT id,
			       nome,
			       email,
			       COALESCE(foto_url, ''),
			       COALESCE(tutorial_visto, false)
			  FROM usuarios
			 WHERE LOWER(email)=LOWER($1)
		`, email).Scan(&user.ID, &user.Nome, &user.Email, &user.FotoUrl, &user.TutorialVisto)

		if err != nil {
			if err == sql.ErrNoRows {
				writeJSONError(w, http.StatusNotFound, "Usuário não encontrado")
			} else {
				log.Println("[perfil] ERRO select:", err)
				writeJSONError(w, http.StatusInternalServerError, "Erro ao buscar usuário")
			}
			return
		}

		writeJSON(w, http.StatusOK, user)
	}
}
