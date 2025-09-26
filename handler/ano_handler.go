// ============================================================================
// 📄 handler/ano_handler.go
// ============================================================================
// 🎯 Responsabilidade
// - Endpoints REST para gerenciamento de "Ano/Turma" (tabela: anos)
//   * Listar anos do usuário autenticado
//   * Criar novo ano vinculado ao usuário
//   * Remover ano do usuário (com remoção em cascata dos estudantes do mesmo dono)
//
// 🔐 Autenticação
// - Baseada no cabeçalho HTTP `X-User-Email` (email do usuário já autenticado).
// - O helper `usuarioIDFromHeader` resolve o `usuario_id` a partir desse e-mail.
// - Todas as rotas retornam 401 quando o cabeçalho não existe ou não encontra usuário.
//
// 🧱 Regras de escopo/segurança
// - Todas as queries incluem `usuario_id = $UID` para isolar os dados por dono.
// - A remoção é transacional: apaga estudantes do ano e, depois, o ano.
// - Retorna 404 quando o ano não pertencer ao usuário ou não existir.
//
// 📤 Formato das respostas
// - JSON (`Content-Type: application/json; charset=utf-8`) para retornos com corpo.
// - 204 (No Content) para deleção bem-sucedida.
// - Erros com mensagens claras e status apropriados.
// ============================================================================

package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Ano representa um registro da tabela `anos`.
type Ano struct {
	ID   int    `json:"id"`   // identificador do ano/turma
	Nome string `json:"nome"` // nome exibido (ex.: "8º A")
}

// timeout padrão para chamadas ao banco
const dbTimeout = 5 * time.Second

// usuarioIDFromHeader resolve o id do usuário a partir do cabeçalho X-User-Email.
//
// Fluxo:
//  1. Lê e normaliza o valor de "X-User-Email".
//  2. Busca o id na tabela `usuarios`.
//  3. Retorna (id, nil) quando encontra; caso contrário retorna erro.
//
// Retorna:
//   - (0, sql.ErrNoRows) quando o header está vazio ou não encontra usuário.
//   - Outros erros de banco quando a query falha.
func usuarioIDFromHeader(db *sql.DB, r *http.Request) (int, error) {
	email := strings.TrimSpace(strings.ToLower(r.Header.Get("X-User-Email")))
	if email == "" {
		return 0, sql.ErrNoRows
	}
	ctx, cancel := context.WithTimeout(r.Context(), dbTimeout)
	defer cancel()

	var id int
	err := db.QueryRowContext(ctx, "SELECT id FROM usuarios WHERE email=$1", email).Scan(&id)
	return id, err
}

// ListarAnosHandler trata GET /api/anos
//
// Regras/erros:
//   - 401 se não conseguir resolver o usuário pelo header.
//   - 500 se houver falha ao consultar/iterar o banco.
//   - 200 + JSON com array de anos quando OK.
func ListarAnosHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid, err := usuarioIDFromHeader(db, r)
		if err != nil {
			http.Error(w, "Usuário não autenticado", http.StatusUnauthorized)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), dbTimeout)
		defer cancel()

		rows, err := db.QueryContext(ctx, `
			SELECT id, nome
			  FROM anos
			 WHERE usuario_id = $1
			 ORDER BY id ASC
		`, uid)
		if err != nil {
			http.Error(w, "Erro ao listar anos: "+err.Error(), http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		var anos []Ano
		for rows.Next() {
			var a Ano
			if err := rows.Scan(&a.ID, &a.Nome); err != nil {
				http.Error(w, "Erro ao ler ano: "+err.Error(), http.StatusInternalServerError)
				return
			}
			anos = append(anos, a)
		}
		if err := rows.Err(); err != nil {
			http.Error(w, "Erro ao iterar anos: "+err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(anos)
	}
}

// CriarAnoHandler trata POST /api/anos
//
// Corpo esperado (JSON):
//
//	{ "nome": "8º A" }
//
// Regras/erros:
//   - 401 se não resolver usuário.
//   - 400 se JSON inválido ou nome vazio.
//   - 500 em erro de inserção.
//   - 201 + JSON { id, nome } quando criado.
func CriarAnoHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid, err := usuarioIDFromHeader(db, r)
		if err != nil {
			http.Error(w, "Usuário não autenticado", http.StatusUnauthorized)
			return
		}

		var input struct {
			Nome string `json:"nome"`
		}
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			http.Error(w, "JSON inválido: "+err.Error(), http.StatusBadRequest)
			return
		}
		input.Nome = strings.TrimSpace(input.Nome)
		if input.Nome == "" {
			http.Error(w, "Nome do ano obrigatório", http.StatusBadRequest)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), dbTimeout)
		defer cancel()

		var novoID int
		err = db.QueryRowContext(ctx, `
			INSERT INTO anos (nome, usuario_id)
			VALUES ($1, $2) RETURNING id
		`, input.Nome, uid).Scan(&novoID)
		if err != nil {
			http.Error(w, "Erro ao criar ano: "+err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":   novoID,
			"nome": input.Nome,
		})
	}
}

// RemoverAnoHandler trata DELETE /api/anos/{id}
//
// Regras/erros:
//   - 405 se método != DELETE.
//   - 401 se não resolver usuário.
//   - 400 se id ausente ou inválido.
//   - 500 se falhar iniciar/execução/commit da transação.
//   - 404 se o ano não existir para esse usuário.
//   - 204 (No Content) quando removido com sucesso.
func RemoverAnoHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			http.Error(w, "Método não permitido", http.StatusMethodNotAllowed)
			return
		}

		uid, err := usuarioIDFromHeader(db, r)
		if err != nil {
			http.Error(w, "Usuário não autenticado", http.StatusUnauthorized)
			return
		}

		// Extrai o id da URL e valida
		idStr := strings.TrimSpace(strings.TrimPrefix(r.URL.Path, "/api/anos/"))
		if idStr == "" {
			http.Error(w, "ID do ano/turma não informado", http.StatusBadRequest)
			return
		}
		id, err := strconv.Atoi(idStr)
		if err != nil || id <= 0 {
			http.Error(w, "ID do ano/turma inválido", http.StatusBadRequest)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), dbTimeout)
		defer cancel()

		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			http.Error(w, "Erro ao iniciar transação", http.StatusInternalServerError)
			return
		}
		defer func() { _ = tx.Rollback() }()

		// 1) apaga estudantes do mesmo dono e ano
		if _, err := tx.ExecContext(ctx,
			`DELETE FROM estudantes WHERE ano_id=$1 AND usuario_id=$2`,
			id, uid,
		); err != nil {
			http.Error(w, "Erro ao remover estudantes vinculados", http.StatusInternalServerError)
			return
		}

		// 2) apaga o ano pertencente ao dono
		res, err := tx.ExecContext(ctx,
			`DELETE FROM anos WHERE id=$1 AND usuario_id=$2`,
			id, uid,
		)
		if err != nil {
			http.Error(w, "Erro ao remover ano/turma", http.StatusInternalServerError)
			return
		}

		// Se nenhuma linha foi afetada, o registro não existe/pertence ao usuário
		aff, _ := res.RowsAffected()
		if aff == 0 {
			http.Error(w, "Ano/Turma não encontrado", http.StatusNotFound)
			return
		}

		if err := tx.Commit(); err != nil {
			http.Error(w, "Erro ao confirmar exclusão", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}
