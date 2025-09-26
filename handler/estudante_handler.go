// ============================================================================
// 📄 handler/estudante_handler.go
// ============================================================================
// 🎯 Responsabilidade
// - Handlers HTTP para estudantes: criar, listar, editar, excluir e checagens
//   de duplicidade (CPF/E-mail).
// - Todas as rotas exigem autenticação via Header `X-User-Email`.
//
// 🛡️ Segurança e Escopo
// - Todas as operações são filtradas por `usuario_id` (dono do registro).
// - Usa o mesmo timeout de DB definido em `handler/ano_handler.go` (dbTimeout).
//
// ============================================================================

package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"backend/model"

	"github.com/lib/pq"
)

// ==========================
// Helpers
// ==========================

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeJSONError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// mapPQError converte erros do Postgres (pq.Error) para mensagens amigáveis
// (ex.: violação de unicidade em CPF/E-mail por usuário)
func mapPQError(err error) (status int, message string, handled bool) {
	if err == nil {
		return 0, "", false
	}
	if pqErr, ok := err.(*pq.Error); ok {
		if string(pqErr.Code) == "23505" { // unique_violation
			switch pqErr.Constraint {
			case "estudantes_cpf_usuario_unique":
				return http.StatusConflict, "CPF já cadastrado para este usuário.", true
			case "estudantes_email_usuario_unique":
				return http.StatusConflict, "E-mail já cadastrado para este usuário.", true
			}
			return http.StatusConflict, "Registro já existente (violação de unicidade).", true
		}
	}
	return 0, "", false
}

// remove tudo que não for dígito (para checagem de CPF)
func digitsOnly(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// =============================================
// 🔹 Criar Estudante (POST) — /api/estudantes
// =============================================
//
// • Exige Nome, CPF, Email e DataNascimento
// • Insere no banco vinculado ao usuario_id
// • Retorna o estudante criado em JSON
func CriarEstudanteHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSONError(w, http.StatusMethodNotAllowed, "Método não permitido")
			return
		}

		// 🔐 Dono (reutiliza helper do mesmo package)
		uid, err := usuarioIDFromHeader(db, r)
		if err != nil {
			writeJSONError(w, http.StatusUnauthorized, "Usuário não autenticado")
			return
		}

		// 📨 Decodifica & valida (usa DTO do model)
		var in model.EstudanteCreateRequest
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			writeJSONError(w, http.StatusBadRequest, "JSON inválido")
			return
		}
		in.Sanitize()
		if err := in.Validate(); err != nil {
			writeJSONError(w, http.StatusBadRequest, err.Error())
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), dbTimeout)
		defer cancel()

		// 🧱 Insere e retorna o id criado
		var novoID int
		err = db.QueryRowContext(ctx, `
			INSERT INTO estudantes (nome, cpf, email, data_nascimento, telefone, foto_url, ano_id, turma_id, usuario_id)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
			RETURNING id
		`,
			in.Nome, in.CPF, in.Email, in.DataNascimento, in.Telefone, in.FotoURL, in.AnoID, in.TurmaID, uid,
		).Scan(&novoID)
		if status, msg, ok := mapPQError(err); ok {
			writeJSONError(w, status, msg)
			return
		}
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "Erro ao criar estudante")
			return
		}

		// Monta retorno compatível (sem usuario_id)
		out := model.Estudante{
			ID:             novoID,
			Nome:           in.Nome,
			CPF:            in.CPF,
			Email:          in.Email,
			DataNascimento: in.DataNascimento,
			Telefone:       in.Telefone,
			FotoURL:        in.FotoURL,
			AnoID:          in.AnoID,
			TurmaID:        in.TurmaID,
		}
		writeJSON(w, http.StatusCreated, out)
	}
}

// ====================================================
// 🔹 Listar Estudantes (GET) — /api/estudantes
// ====================================================
//
// • Lista todos os estudantes do usuário autenticado
// • Ordena pelo ID crescente
func ListarEstudantesHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeJSONError(w, http.StatusMethodNotAllowed, "Método não permitido")
			return
		}

		uid, err := usuarioIDFromHeader(db, r)
		if err != nil {
			writeJSONError(w, http.StatusUnauthorized, "Usuário não autenticado")
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), dbTimeout)
		defer cancel()

		rows, err := db.QueryContext(ctx, `
			SELECT id, nome, cpf, email, data_nascimento, telefone, foto_url, ano_id, turma_id
			  FROM estudantes
			 WHERE usuario_id = $1
			 ORDER BY id ASC
		`, uid)
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "Erro ao buscar estudantes")
			return
		}
		defer rows.Close()

		var estudantes []model.Estudante
		for rows.Next() {
			var est model.Estudante
			if err := rows.Scan(
				&est.ID, &est.Nome, &est.CPF, &est.Email, &est.DataNascimento,
				&est.Telefone, &est.FotoURL, &est.AnoID, &est.TurmaID,
			); err != nil {
				writeJSONError(w, http.StatusInternalServerError, "Erro ao ler dados")
				return
			}
			estudantes = append(estudantes, est)
		}
		if err := rows.Err(); err != nil {
			writeJSONError(w, http.StatusInternalServerError, "Erro ao iterar dados")
			return
		}

		writeJSON(w, http.StatusOK, estudantes)
	}
}

// =========================================================
// 🔹 Editar Estudante (PUT) — /api/estudantes/{id}
// =========================================================
//
// • Valida campos obrigatórios (mantém contrato atual)
// • Atualiza dados apenas se pertencer ao usuário
func EditarEstudanteHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			writeJSONError(w, http.StatusMethodNotAllowed, "Método não permitido")
			return
		}

		uid, err := usuarioIDFromHeader(db, r)
		if err != nil {
			writeJSONError(w, http.StatusUnauthorized, "Usuário não autenticado")
			return
		}

		// ID do path
		idStr := strings.TrimPrefix(r.URL.Path, "/api/estudantes/")
		id, err := strconv.Atoi(strings.TrimSpace(idStr))
		if err != nil || id <= 0 {
			writeJSONError(w, http.StatusBadRequest, "ID do estudante inválido")
			return
		}

		// Decodifica & valida (usamos DTO de criação para manter "todos obrigatórios")
		var in model.EstudanteCreateRequest
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			writeJSONError(w, http.StatusBadRequest, "JSON inválido")
			return
		}
		in.Sanitize()
		if err := in.Validate(); err != nil {
			writeJSONError(w, http.StatusBadRequest, err.Error())
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), dbTimeout)
		defer cancel()

		res, err := db.ExecContext(ctx, `
			UPDATE estudantes
			   SET nome=$1, cpf=$2, email=$3, data_nascimento=$4, telefone=$5, foto_url=$6, ano_id=$7, turma_id=$8
			 WHERE id=$9 AND usuario_id=$10
		`,
			in.Nome, in.CPF, in.Email, in.DataNascimento,
			in.Telefone, in.FotoURL, in.AnoID, in.TurmaID,
			id, uid,
		)
		if status, msg, ok := mapPQError(err); ok {
			writeJSONError(w, status, msg)
			return
		}
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "Erro ao editar estudante")
			return
		}
		if rows, _ := res.RowsAffected(); rows == 0 {
			writeJSONError(w, http.StatusNotFound, "Estudante não encontrado")
			return
		}

		writeJSON(w, http.StatusOK, map[string]string{"message": "Estudante editado com sucesso"})
	}
}

// ==========================================================
// 🔹 Remover Estudante (DELETE) — /api/estudantes/{id}
// ==========================================================
//
// • Exclui estudante apenas se pertencer ao usuário
func RemoverEstudanteHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			writeJSONError(w, http.StatusMethodNotAllowed, "Método não permitido")
			return
		}

		uid, err := usuarioIDFromHeader(db, r)
		if err != nil {
			writeJSONError(w, http.StatusUnauthorized, "Usuário não autenticado")
			return
		}

		idStr := strings.TrimPrefix(r.URL.Path, "/api/estudantes/")
		id, err := strconv.Atoi(strings.TrimSpace(idStr))
		if err != nil || id <= 0 {
			writeJSONError(w, http.StatusBadRequest, "ID do estudante inválido")
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), dbTimeout)
		defer cancel()

		res, err := db.ExecContext(ctx, `DELETE FROM estudantes WHERE id=$1 AND usuario_id=$2`, id, uid)
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "Erro ao excluir estudante")
			return
		}
		if rows, _ := res.RowsAffected(); rows == 0 {
			writeJSONError(w, http.StatusNotFound, "Estudante não encontrado")
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

// =============================================================
// 🔹 Verificar CPF duplicado (GET)
//
//	/api/estudantes/check-cpf?cpf=...&ignoreId=...
//
// =============================================================
func VerificarCpfHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Método não permitido", http.StatusMethodNotAllowed)
			return
		}

		uid, err := usuarioIDFromHeader(db, r)
		if err != nil {
			http.Error(w, "Usuário não autenticado", http.StatusUnauthorized)
			return
		}

		cpf := digitsOnly(strings.TrimSpace(r.URL.Query().Get("cpf")))
		ignoreID := strings.TrimSpace(r.URL.Query().Get("ignoreId"))
		if ignoreID == "" {
			ignoreID = strings.TrimSpace(r.URL.Query().Get("excludeId"))
		}
		if cpf == "" {
			writeJSONError(w, http.StatusBadRequest, "cpf é obrigatório")
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), dbTimeout)
		defer cancel()

		query := `SELECT 1 FROM estudantes WHERE usuario_id=$1 AND cpf=$2`
		args := []any{uid, cpf}
		if ignoreID != "" {
			query += ` AND id<>$3`
			args = append(args, ignoreID)
		}

		var dummy int
		err = db.QueryRowContext(ctx, query, args...).Scan(&dummy)
		exists := (err == nil)

		writeJSON(w, http.StatusOK, map[string]bool{"exists": exists})
	}
}

// =============================================================
// 🔹 Verificar E-mail duplicado (GET)
//
//	/api/estudantes/check-email?email=...&ignoreId=...
//
// =============================================================
func VerificarEmailHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Método não permitido", http.StatusMethodNotAllowed)
			return
		}

		uid, err := usuarioIDFromHeader(db, r)
		if err != nil {
			http.Error(w, "Usuário não autenticado", http.StatusUnauthorized)
			return
		}

		emailParam := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("email")))
		ignoreID := strings.TrimSpace(r.URL.Query().Get("ignoreId"))
		if ignoreID == "" {
			ignoreID = strings.TrimSpace(r.URL.Query().Get("excludeId"))
		}
		if emailParam == "" {
			writeJSONError(w, http.StatusBadRequest, "email é obrigatório")
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), dbTimeout)
		defer cancel()

		query := `SELECT 1 FROM estudantes WHERE usuario_id=$1 AND LOWER(email)=LOWER($2)`
		args := []any{uid, emailParam}
		if ignoreID != "" {
			query += ` AND id<>$3`
			args = append(args, ignoreID)
		}

		var dummy int
		err = db.QueryRowContext(ctx, query, args...).Scan(&dummy)
		exists := (err == nil)

		writeJSON(w, http.StatusOK, map[string]bool{"exists": exists})
	}
}
