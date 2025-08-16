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

/*
===============================================================================
🔹 Tipos (nomes exclusivos p/ evitar colisão com outros pacotes)
   - Estruturas usadas como payload de entrada/saída nos endpoints do usuário.
===============================================================================
*/

// registerRequest representa o payload aceito em POST /register.
// Regras de validação aplicadas no handler:
// - Nome: obrigatório, mínimo 2 caracteres (trim aplicado).
// - Email: obrigatório, sem espaços (trim aplicado), formato válido por regex.
// - Senha: obrigatória, mínimo 8 caracteres, sem espaços.
type registerRequest struct {
	Nome  string `json:"nome"`
	Email string `json:"email"`
	Senha string `json:"senha"`
}

// loginRequest representa o payload aceito em POST /login.
// Regras de validação aplicadas no handler:
// - Email: obrigatório, sem espaços.
// - Senha: obrigatória, mínimo 8 caracteres, sem espaços.
type loginRequest struct {
	Email string `json:"email"`
	Senha string `json:"senha"`
}

// LoginUserResponse é a resposta de sucesso em POST /login.
// Retorna dados básicos do usuário autenticado.
type LoginUserResponse struct {
	ID      int    `json:"id"`
	Nome    string `json:"nome"`
	Email   string `json:"email"`
	FotoURL string `json:"fotoUrl"`
}

// UsuarioInfo é um modelo auxiliar (usado em outros handlers) para
// retorno de dados do usuário (inclui tutorial_visto).
// Não é utilizado diretamente neste arquivo, mas mantido por coesão.
type UsuarioInfo struct {
	ID            int64  `json:"id"`
	Nome          string `json:"nome"`
	Email         string `json:"email"`
	FotoURL       string `json:"fotoUrl"`
	TutorialVisto bool   `json:"tutorial_visto"`
}

/*
===============================================================================
🔹 POST /register
  - Objetivo: Cadastrar um novo usuário.
  - Entrada (JSON): { nome, email, senha }
  - Validações:
  - nome: trim + mínimo 2 chars
  - email: sem espaços, regex simples para formato
  - senha: mínimo 8 chars, sem espaços
  - Regras de negócio:
  - email deve ser único (verificação em banco)
  - senha salva com hash bcrypt
  - Respostas:
  - 201 Created + {"ok":true} em sucesso
  - 400 Bad Request para falhas de validação / JSON inválido
  - 409 Conflict se e-mail já existir
  - 500 Internal Server Error para falhas inesperadas

===============================================================================
*/
func RegisterHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Decodifica corpo JSON para estrutura tipada
		var req registerRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "JSON inválido", http.StatusBadRequest)
			return
		}

		// Validação de nome (mínimo 2 chars após trim)
		req.Nome = strings.TrimSpace(req.Nome)
		if len(req.Nome) < 2 {
			http.Error(w, "Nome muito curto", http.StatusBadRequest)
			return
		}

		// Validações de e-mail: sem espaços no início/fim e no meio
		if req.Email == "" || req.Email != strings.TrimSpace(req.Email) {
			http.Error(w, "E-mail não pode ter espaço no início/fim", http.StatusBadRequest)
			return
		}
		if strings.Contains(req.Email, " ") {
			http.Error(w, "E-mail não pode conter espaços", http.StatusBadRequest)
			return
		}

		// Regex simples para validar formato do e-mail
		emailRe := regexp.MustCompile(`^[\w\-.]+@([\w-]+\.)+[\w-]{2,4}$`)
		if !emailRe.MatchString(req.Email) {
			http.Error(w, "E-mail inválido", http.StatusBadRequest)
			return
		}

		// Validação de senha: mínimo 8, sem espaços
		if req.Senha == "" || len(req.Senha) < 8 {
			http.Error(w, "Senha muito curta (mínimo 8)", http.StatusBadRequest)
			return
		}
		if strings.Contains(req.Senha, " ") {
			http.Error(w, "Senha não pode conter espaços", http.StatusBadRequest)
			return
		}

		// Verifica duplicidade de e-mail no banco (unicidade lógica)
		var count int
		if err := db.QueryRow(`SELECT COUNT(*) FROM usuarios WHERE email=$1`, req.Email).Scan(&count); err != nil {
			http.Error(w, "Erro ao verificar e-mail", http.StatusInternalServerError)
			return
		}
		if count > 0 {
			http.Error(w, "E-mail já cadastrado", http.StatusConflict)
			return
		}

		// Gera hash seguro da senha com bcrypt
		hash, err := bcrypt.GenerateFromPassword([]byte(req.Senha), bcrypt.DefaultCost)
		if err != nil {
			http.Error(w, "Erro ao processar senha", http.StatusInternalServerError)
			return
		}

		// Insere usuário (tutorial_visto pode ter default false na tabela)
		if _, err := db.Exec(`
			INSERT INTO usuarios (nome, email, senha_hash)
			VALUES ($1, $2, $3)
		`, req.Nome, req.Email, string(hash)); err != nil {
			http.Error(w, "Erro ao salvar usuário", http.StatusInternalServerError)
			return
		}

		// Sucesso
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"ok":true}`))
	}
}

/*
===============================================================================
🔹 POST /login
  - Objetivo: Autenticar usuário com email/senha.
  - Entrada (JSON): { email, senha }
  - Validações:
  - email: obrigatório, sem espaços
  - senha: obrigatória, ≥ 8 chars, sem espaços
  - Regras:
  - Busca por e-mail, recupera hash e compara com bcrypt
  - Respostas:
  - 200 OK + LoginUserResponse em sucesso
  - 400 Bad Request para payload/validação inválidos
  - 401 Unauthorized para credenciais inválidas
  - 500 Internal Server Error para falhas de I/O

===============================================================================
*/
func LoginHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Decode do payload de login
		var req loginRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "JSON inválido", http.StatusBadRequest)
			return
		}

		// Validações básicas
		if req.Email == "" || strings.Contains(req.Email, " ") {
			http.Error(w, "E-mail inválido", http.StatusBadRequest)
			return
		}
		if req.Senha == "" || len(req.Senha) < 8 || strings.Contains(req.Senha, " ") {
			http.Error(w, "Senha inválida", http.StatusBadRequest)
			return
		}

		// Busca o usuário por e-mail (retorna hash e metadados)
		var id int
		var nome, hash, fotoURL string
		err := db.QueryRow(`
			SELECT id, nome, senha_hash, COALESCE(foto_url,'')
			  FROM usuarios
			 WHERE email=$1
		`, req.Email).Scan(&id, &nome, &hash, &fotoURL)

		// Trate "não encontrado" como 401 para não vazar existência de conta
		if err == sql.ErrNoRows {
			http.Error(w, "E-mail ou senha incorretos", http.StatusUnauthorized)
			return
		}
		if err != nil {
			http.Error(w, "Erro ao verificar usuário", http.StatusInternalServerError)
			return
		}

		// Compara senha informada com hash armazenado
		if bcrypt.CompareHashAndPassword([]byte(hash), []byte(req.Senha)) != nil {
			http.Error(w, "E-mail ou senha incorretos", http.StatusUnauthorized)
			return
		}

		// Resposta de sucesso (pode ser estendida com tokens/sessões se necessário)
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
===============================================================================
🔹 PUT /api/usuario/{id}/tutorial
  - Objetivo: Marcar/Desmarcar o flag tutorial_visto de um usuário.
  - Caminho: /api/usuario/{id}/tutorial  (id numérico, > 0)
  - Entrada (opcional) JSON: { "tutorial_visto": <bool> }
  - Se não enviar o corpo, assume true por padrão.
  - Regras:
  - Aceita somente método PUT.
  - Atualiza a coluna tutorial_visto na tabela usuarios.
  - Respostas:
  - 204 No Content em sucesso
  - 400 Bad Request para id inválido
  - 405 Method Not Allowed para método diferente de PUT
  - 500 Internal Server Error em falhas de I/O

===============================================================================
*/
func MarcarTutorialVistoHandler(db *sql.DB) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Restringe ao método PUT
		if r.Method != http.MethodPut {
			http.Error(w, "Método não permitido", http.StatusMethodNotAllowed)
			return
		}

		// Extrai /api/usuario/{id}/tutorial → isola {id}
		p := strings.TrimPrefix(r.URL.Path, "/api/usuario/")
		if !strings.HasSuffix(p, "/tutorial") {
			http.NotFound(w, r)
			return
		}
		idStr := strings.TrimSuffix(p, "/tutorial")
		idStr = strings.Trim(idStr, "/")

		// Valida id numérico e positivo
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil || id <= 0 {
			http.Error(w, "id inválido", http.StatusBadRequest)
			return
		}

		// Corpo opcional — default = true
		var body struct {
			TutorialVisto *bool `json:"tutorial_visto"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		val := true
		if body.TutorialVisto != nil {
			val = *body.TutorialVisto
		}

		// Atualiza a flag no banco
		if _, err := db.Exec(`UPDATE usuarios SET tutorial_visto=$1 WHERE id=$2`, val, id); err != nil {
			http.Error(w, "erro ao atualizar", http.StatusInternalServerError)
			return
		}

		// 204 → sem payload
		w.WriteHeader(http.StatusNoContent)
	})
}
