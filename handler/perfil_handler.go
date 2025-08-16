//
// ======================================================================
// 📌 handler/perfil_handler.go
//
// 🎯 Responsabilidade
//    - Handlers HTTP relacionados ao PERFIL do usuário.
//    - Atualiza nome/foto e, opcionalmente, a senha do usuário logado.
//    - Busca dados do usuário por e-mail (inclui `tutorial_visto` para
//      o front decidir se exibe o onboarding).
//
// 🔒 Autenticação
//    - Para atualização de perfil (PUT /api/perfil) é obrigatório o
//      header `X-User-Email` contendo o e-mail do usuário logado.
//    - Para busca por e-mail (GET /api/usuario?email=...) o e-mail é
//      passado via query string.
//
// 🧱 Banco
//    - Tabela `usuarios` com colunas relevantes: id, nome, email,
//      foto_url, senha_hash, tutorial_visto.
//
// 📦 Rotas
//    - PUT  /api/perfil
//      • Body JSON: { "nome": "...", "foto_url": "...", "fotoUrl": "...", "senha": "..." }
//        (senha é OPCIONAL; se vier vazia, não é alterada)
//      • Header: X-User-Email
//
//    - GET  /api/usuario?email=...
//      • Retorna: { id, nome, email, fotoUrl, tutorial_visto }
//
// 🧪 Regras/Validações
//    - Nome: mínimo 2 caracteres.
//    - Senha (quando enviada):
//        • Mínimo 8 caracteres
//        • Não pode conter espaço
//
// 🔐 Segurança
//    - Atualização condicionada à existência do usuário (email).
//    - Hash de senha com bcrypt quando senha for enviada.
//
// Projeto: TecMise
// ======================================================================
//

package handler

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

// ======================================================================
// 🔄 Atualizar Perfil
// ----------------------------------------------------------------------
// PUT /api/perfil
//
// Requisitos:
//   - Header: X-User-Email
//   - Body JSON:
//     {
//     "nome": "Nome do usuário",
//     "foto_url": "https://.../avatar.jpg",  // (snake_case)
//     "fotoUrl" : "https://.../avatar.jpg",  // (camelCase) — fallback
//     "senha": "opcional, se fornecida será atualizada"
//     }
//
// Regras:
//   - Nome >= 2 caracteres
//   - Se senha vier preenchida:
//   - >= 8 caracteres
//   - sem espaços
//   - será persistida como bcrypt hash em `senha_hash`
//
// Respostas:
//   - 200 OK           → {"ok":true}
//   - 400 Bad Request  → erros de validação / JSON inválido
//   - 401 Unauthorized → header ausente / usuário não autenticado
//   - 404 Not Found    → usuário não encontrado
//   - 405 Method Not Allowed
//   - 500 Internal Server Error
//
// ======================================================================
func AtualizarPerfilHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Método permitido apenas PUT
		if r.Method != http.MethodPut {
			http.Error(w, "Método não permitido", http.StatusMethodNotAllowed)
			return
		}

		// Estrutura do payload de entrada
		var req struct {
			Nome    string `json:"nome"`
			FotoURL string `json:"foto_url"` // snake_case
			FotoUrl string `json:"fotoUrl"`  // camelCase (compatibilidade)
			Senha   string `json:"senha"`    // opcional
		}

		// Decodifica JSON do body
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "JSON inválido", http.StatusBadRequest)
			return
		}

		// Autenticação via header
		email := strings.TrimSpace(r.Header.Get("X-User-Email"))
		if email == "" {
			http.Error(w, "Usuário não autenticado", http.StatusUnauthorized)
			return
		}

		// Validação do nome
		nome := strings.TrimSpace(req.Nome)
		if len(nome) < 2 {
			http.Error(w, "Nome muito curto", http.StatusBadRequest)
			return
		}

		// Foto: prioriza `foto_url`; se vazio e existir `fotoUrl`, usa-a
		fotoFinal := strings.TrimSpace(req.FotoURL)
		if fotoFinal == "" && req.FotoUrl != "" {
			fotoFinal = strings.TrimSpace(req.FotoUrl)
		}

		// Verifica se usuário existe
		var exists bool
		if err := db.QueryRow(`SELECT EXISTS(SELECT 1 FROM usuarios WHERE email=$1)`, email).Scan(&exists); err != nil || !exists {
			http.Error(w, "Usuário não encontrado", http.StatusNotFound)
			return
		}

		// Monta UPDATE conforme presença (ou não) de senha
		if strings.TrimSpace(req.Senha) != "" {
			// Validação mínima de senha
			s := req.Senha
			if len(s) < 8 || strings.Contains(s, " ") {
				http.Error(w, "Senha inválida (mínimo 8 caracteres e sem espaços)", http.StatusBadRequest)
				return
			}

			// Gera hash bcrypt
			hash, err := bcrypt.GenerateFromPassword([]byte(s), bcrypt.DefaultCost)
			if err != nil {
				http.Error(w, "Erro ao processar senha", http.StatusInternalServerError)
				return
			}

			// Atualiza com senha
			if _, err := db.Exec(
				`UPDATE usuarios SET nome=$1, foto_url=$2, senha_hash=$3 WHERE email=$4`,
				nome, fotoFinal, string(hash), email,
			); err != nil {
				log.Println("[perfil] ERRO update (com senha):", err)
				http.Error(w, "Erro ao atualizar perfil", http.StatusInternalServerError)
				return
			}
		} else {
			// Atualiza sem senha
			if _, err := db.Exec(
				`UPDATE usuarios SET nome=$1, foto_url=$2 WHERE email=$3`,
				nome, fotoFinal, email,
			); err != nil {
				log.Println("[perfil] ERRO update:", err)
				http.Error(w, "Erro ao atualizar perfil", http.StatusInternalServerError)
				return
			}
		}

		// Resposta OK
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}
}

// ======================================================================
// 🔎 Buscar Usuário por E-mail
// ----------------------------------------------------------------------
// GET /api/usuario?email=...
//
// Uso: permitir que o front recupere perfil inicial e o estado
//
//	do tutorial (se já foi visto).
//
// Respostas:
//   - 200 OK → { id, nome, email, fotoUrl, tutorial_visto }
//   - 400 Bad Request  → e-mail não informado
//   - 404 Not Found    → usuário não encontrado
//   - 500 Internal Server Error
//
// ======================================================================
func BuscarUsuarioPorEmailHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Lê e valida e-mail da query string
		email := strings.TrimSpace(r.URL.Query().Get("email"))
		if email == "" {
			http.Error(w, "E-mail não informado", http.StatusBadRequest)
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

		// Busca dados no banco
		err := db.QueryRow(`
				SELECT id,
				       nome,
				       email,
				       COALESCE(foto_url, ''),
				       COALESCE(tutorial_visto, false)
				  FROM usuarios
				 WHERE email=$1
			`, email).Scan(&user.ID, &user.Nome, &user.Email, &user.FotoUrl, &user.TutorialVisto)

		// Tratamento de erro/busca
		if err != nil {
			if err == sql.ErrNoRows {
				http.Error(w, "Usuário não encontrado", http.StatusNotFound)
			} else {
				log.Println("[perfil] ERRO select:", err)
				http.Error(w, "Erro ao buscar usuário", http.StatusInternalServerError)
			}
			return
		}

		// Retorna JSON
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(user)
	}
}
