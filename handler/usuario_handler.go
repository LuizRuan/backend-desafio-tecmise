/*
/// Projeto: Tecmise
/// Arquivo: backend/handler/usuario_handler.go
/// Responsabilidade: Handlers HTTP para cadastro, login e atualização do flag de tutorial do usuário.
/// Dependências principais: database/sql (Postgres), backend/model (DTOs), bcrypt (hash de senha), github.com/lib/pq (erros PG).
/// Pontos de atenção:
/// - Não há aplicação dos middlewares de validação em main.go para /register e /login; este handler faz validação "defensiva".
/// - Divergência potencial com model.MinPasswordLen (6) — aqui exigimos 8 caracteres (alinhado ao frontend).
/// - Igualdade por LOWER(email) depende de índice/estratégia no banco; CITEXT pode ser mais eficiente.
/// - writeJSON / writeJSONError e dbTimeout são dependências implícitas deste pacote (definidas em outro arquivo do package).
/// - Retorno de login inclui FotoURL como "fotoUrl" (camelCase), compatível com o contrato atual do frontend.
/// - Erros são propositadamente genéricos para não vazar detalhes sensíveis (e.g., distinção de usuário inexistente).
*/

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

/**
 * RegisterHandler registra novos usuários.
 *
 * Regras de validação (defensivas):
 * - Nome: trim e tamanho mínimo 2.
 * - E-mail: validação via net/mail.ParseAddress (case-insensitive no banco).
 * - Senha: mínimo 8 caracteres e sem espaços (alinhado ao frontend).
 *
 * Persistência:
 * - Confere unicidade por LOWER(email).
 * - Hash de senha com bcrypt.DefaultCost.
 * - Em conflito (unique constraint 23505), retorna 409.
 *
 * Erros e respostas:
 * - 201 com {"ok": true} em sucesso.
 * - 400/409/500 com mensagens simples em texto via writeJSONError.
 *
 * Dependências:
 * - dbTimeout (context deadline), writeJSON e writeJSONError (helpers locais do pacote).
 */
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

/**
 * LoginHandler autentica o usuário por e-mail e senha.
 *
 * Validação:
 * - E-mail com mail.ParseAddress.
 * - Senha com mínimo 8 caracteres e sem espaços.
 *
 * Fluxo:
 * - Busca usuário por LOWER(email).
 * - Compara senha via bcrypt.CompareHashAndPassword.
 * - Em sucesso, retorna {id, nome, email, fotoUrl}.
 *
 * Respostas:
 * - 200 OK com JSON do usuário essencial.
 * - 400 para payload inválido.
 * - 401 para credenciais incorretas.
 * - 500 para erros internos/DB.
 *
 * Observações:
 * - Campo FotoURL vem de COALESCE(foto_url,'') no select.
 * - E-mail retornado é o normalizado do request (lowercase por Sanitize()).
 */
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

/**
 * MarcarTutorialVistoHandler atualiza o flag tutorial_visto de um usuário.
 *
 * Rota:
 * - PUT /api/usuario/{id}/tutorial
 *
 * Regras:
 * - {id} deve ser inteiro > 0.
 * - Body opcional {"tutorial_visto": bool}; default=true quando ausente.
 *
 * Respostas:
 * - 204 (No Content) em sucesso.
 * - 400 para id inválido/JSON inválido.
 * - 404 quando o usuário não for encontrado.
 * - 405 para método diferente de PUT.
 * - 500 em falhas de atualização.
 *
 * Observações:
 * - O parsing do path é manual; mudanças de rota exigem cuidado.
 */
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

// TODO: considerar logs estruturados (com request id) para falhas 5xx.
// TODO: avaliar rate limiting em /login para mitigar brute force.
// TODO: alinhar política de mensagens de erro (localização/i18n) com o frontend.
