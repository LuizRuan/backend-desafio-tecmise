package handler

import (
	"backend/model"
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/lib/pq"
)

/* =====================================================
   Helpers de resposta JSON e mapeamento de erros
   ===================================================== */

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeJSONError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// Mapeia erros do Postgres (pq.Error) para mensagens amigáveis
func mapPQError(err error) (status int, message string, handled bool) {
	if err == nil {
		return 0, "", false
	}
	if pqErr, ok := err.(*pq.Error); ok {
		// 23505 = unique_violation
		if string(pqErr.Code) == "23505" {
			switch pqErr.Constraint {
			case "estudantes_cpf_usuario_unique":
				return http.StatusConflict, "CPF já cadastrado para este usuário.", true
			}
			// fallback genérico para outras viol. de unicidade
			return http.StatusConflict, "Registro já existente (violação de unicidade).", true
		}
	}
	return 0, "", false
}

/*
=====================================================

	CRIAR ESTUDANTE (POST)
	=====================================================
*/
func CriarEstudanteHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSONError(w, http.StatusMethodNotAllowed, "Método não permitido")
			return
		}

		var estudante model.Estudante
		if err := json.NewDecoder(r.Body).Decode(&estudante); err != nil {
			writeJSONError(w, http.StatusBadRequest, "Dados inválidos")
			return
		}

		email := strings.TrimSpace(r.Header.Get("X-User-Email"))
		if email == "" {
			writeJSONError(w, http.StatusUnauthorized, "Usuário não autenticado")
			return
		}

		var usuarioID int
		if err := db.QueryRow("SELECT id FROM usuarios WHERE email = $1", email).Scan(&usuarioID); err != nil {
			writeJSONError(w, http.StatusUnauthorized, "Usuário não encontrado")
			return
		}

		// ==== Validação obrigatória ====
		if strings.TrimSpace(estudante.Nome) == "" ||
			strings.TrimSpace(estudante.CPF) == "" ||
			strings.TrimSpace(estudante.Email) == "" ||
			strings.TrimSpace(estudante.DataNascimento) == "" {
			writeJSONError(w, http.StatusBadRequest, "Nome, CPF, email e data de nascimento são obrigatórios!")
			return
		}

		_, err := db.Exec(`
			INSERT INTO estudantes (nome, cpf, email, data_nascimento, telefone, foto_url, ano_id, turma_id, usuario_id)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		`,
			estudante.Nome,
			estudante.CPF,
			estudante.Email,
			estudante.DataNascimento, // "YYYY-MM-DD"
			estudante.Telefone,
			estudante.FotoURL,
			estudante.AnoID,
			estudante.TurmaID,
			usuarioID,
		)
		if status, msg, ok := mapPQError(err); ok {
			writeJSONError(w, status, msg)
			return
		}
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "Erro ao criar estudante")
			return
		}

		writeJSON(w, http.StatusCreated, map[string]string{"message": "Estudante criado com sucesso"})
	}
}

/*
=====================================================

	LISTAR ESTUDANTES (GET)
	=====================================================
*/
func ListarEstudantesHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeJSONError(w, http.StatusMethodNotAllowed, "Método não permitido")
			return
		}

		email := strings.TrimSpace(r.Header.Get("X-User-Email"))
		if email == "" {
			writeJSONError(w, http.StatusUnauthorized, "Usuário não autenticado")
			return
		}

		var usuarioID int
		if err := db.QueryRow("SELECT id FROM usuarios WHERE email = $1", email).Scan(&usuarioID); err != nil {
			writeJSONError(w, http.StatusUnauthorized, "Usuário não encontrado")
			return
		}

		rows, err := db.Query(`
			SELECT id, nome, cpf, email, data_nascimento, telefone, foto_url, ano_id, turma_id
			  FROM estudantes
			 WHERE usuario_id = $1
			 ORDER BY id ASC
		`, usuarioID)
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "Erro ao buscar estudantes")
			return
		}
		defer rows.Close()

		estudantes := []model.Estudante{}
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

		writeJSON(w, http.StatusOK, estudantes)
	}
}

/*
=====================================================

	EDITAR ESTUDANTE (PUT)
	=====================================================
*/
func EditarEstudanteHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			writeJSONError(w, http.StatusMethodNotAllowed, "Método não permitido")
			return
		}

		email := strings.TrimSpace(r.Header.Get("X-User-Email"))
		if email == "" {
			writeJSONError(w, http.StatusUnauthorized, "Usuário não autenticado")
			return
		}

		var usuarioID int
		if err := db.QueryRow("SELECT id FROM usuarios WHERE email = $1", email).Scan(&usuarioID); err != nil {
			writeJSONError(w, http.StatusUnauthorized, "Usuário não encontrado")
			return
		}

		idStr := strings.TrimPrefix(r.URL.Path, "/api/estudantes/")
		if idStr == "" {
			writeJSONError(w, http.StatusBadRequest, "ID do estudante não informado")
			return
		}

		var estudante model.Estudante
		if err := json.NewDecoder(r.Body).Decode(&estudante); err != nil {
			writeJSONError(w, http.StatusBadRequest, "Dados inválidos")
			return
		}

		// Validação obrigatória ao editar também!
		if strings.TrimSpace(estudante.Nome) == "" ||
			strings.TrimSpace(estudante.CPF) == "" ||
			strings.TrimSpace(estudante.Email) == "" ||
			strings.TrimSpace(estudante.DataNascimento) == "" {
			writeJSONError(w, http.StatusBadRequest, "Nome, CPF, email e data de nascimento são obrigatórios!")
			return
		}

		_, err := db.Exec(`
			UPDATE estudantes
			   SET nome=$1, cpf=$2, email=$3, data_nascimento=$4, telefone=$5, foto_url=$6, ano_id=$7, turma_id=$8
			 WHERE id=$9 AND usuario_id=$10
		`,
			estudante.Nome, estudante.CPF, estudante.Email, estudante.DataNascimento,
			estudante.Telefone, estudante.FotoURL, estudante.AnoID, estudante.TurmaID,
			idStr, usuarioID,
		)
		if status, msg, ok := mapPQError(err); ok {
			writeJSONError(w, status, msg)
			return
		}
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "Erro ao editar estudante")
			return
		}

		writeJSON(w, http.StatusOK, map[string]string{"message": "Estudante editado com sucesso"})
	}
}

/*
=====================================================

	REMOVER ESTUDANTE (DELETE)
	=====================================================
*/
func RemoverEstudanteHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			writeJSONError(w, http.StatusMethodNotAllowed, "Método não permitido")
			return
		}

		email := strings.TrimSpace(r.Header.Get("X-User-Email"))
		if email == "" {
			writeJSONError(w, http.StatusUnauthorized, "Usuário não autenticado")
			return
		}

		var usuarioID int
		if err := db.QueryRow("SELECT id FROM usuarios WHERE email = $1", email).Scan(&usuarioID); err != nil {
			writeJSONError(w, http.StatusUnauthorized, "Usuário não encontrado")
			return
		}

		idStr := strings.TrimPrefix(r.URL.Path, "/api/estudantes/")
		if idStr == "" {
			writeJSONError(w, http.StatusBadRequest, "ID do estudante não informado")
			return
		}

		res, err := db.Exec(`DELETE FROM estudantes WHERE id = $1 AND usuario_id = $2`, idStr, usuarioID)
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

// Verifica se já existe um estudante com o CPF informado para o usuário logado.
// GET /api/estudantes/check-cpf?cpf=XXXXXXXXXXX[&excludeId=123]
func VerificarCpfHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Método não permitido", http.StatusMethodNotAllowed)
			return
		}

		email := strings.TrimSpace(r.Header.Get("X-User-Email"))
		if email == "" {
			http.Error(w, "Usuário não autenticado", http.StatusUnauthorized)
			return
		}

		var usuarioID int
		if err := db.QueryRow("SELECT id FROM usuarios WHERE email=$1", email).Scan(&usuarioID); err != nil {
			http.Error(w, "Usuário não encontrado", http.StatusUnauthorized)
			return
		}

		cpf := strings.TrimSpace(r.URL.Query().Get("cpf"))
		excludeID := strings.TrimSpace(r.URL.Query().Get("excludeId"))

		if cpf == "" {
			http.Error(w, `{"error":"cpf é obrigatório"}`, http.StatusBadRequest)
			return
		}

		// Se excludeId vier (edição), ignora o próprio registro.
		query := `SELECT 1 FROM estudantes WHERE usuario_id=$1 AND cpf=$2`
		args := []interface{}{usuarioID, cpf}
		if excludeID != "" {
			query += ` AND id<>$3`
			args = append(args, excludeID)
		}

		var dummy int
		err := db.QueryRow(query, args...).Scan(&dummy)
		exists := (err == nil)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]bool{"exists": exists})
	}
}
