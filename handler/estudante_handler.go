//
// =====================================================
// 📌 estudante_handler.go
//
// 🎯 Responsabilidade:
//    - Implementa todos os handlers HTTP relacionados a
//      estudantes: criar, listar, editar, excluir e
//      verificar duplicidade de CPF/E-mail.
//    - Todas as rotas exigem autenticação do usuário
//      via Header `X-User-Email`.
//
// 📦 Fluxo Geral:
//    1. Valida o método HTTP permitido.
//    2. Autentica usuário pelo e-mail no Header.
//    3. Interage com o banco PostgreSQL (tabela `estudantes`).
//    4. Retorna resposta em formato JSON.
//
// 🔒 Segurança:
//    - Cada ação é vinculada ao `usuario_id`, garantindo
//      que um usuário só manipule seus próprios estudantes.
//
// =====================================================
//

package handler

import (
	"backend/model"
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/lib/pq"
)

//
// =====================================================
// 🔹 Helpers — Respostas JSON e Mapeamento de Erros
// =====================================================
//

// writeJSON envia uma resposta JSON genérica
func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

// writeJSONError envia uma resposta de erro no formato JSON
func writeJSONError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// mapPQError converte erros do Postgres (pq.Error) para mensagens amigáveis
//
// Exemplos tratados:
//   - 23505 (unique_violation)
//   - CPF já cadastrado para este usuário
//   - E-mail já cadastrado para este usuário
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
			case "estudantes_email_usuario_unique":
				return http.StatusConflict, "E-mail já cadastrado para este usuário.", true
			}
			// fallback genérico para outras violações de unicidade
			return http.StatusConflict, "Registro já existente (violação de unicidade).", true
		}
	}

	return 0, "", false
}

// =====================================================
// 🔹 Criar Estudante (POST) — /api/estudantes
// =====================================================
//
// • Valida corpo da requisição
// • Exige Nome, CPF, Email e DataNascimento
// • Insere no banco vinculado ao usuario_id
// • Retorna o estudante criado em JSON
func CriarEstudanteHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSONError(w, http.StatusMethodNotAllowed, "Método não permitido")
			return
		}

		// 1️⃣ Decodifica corpo da requisição
		var estudante model.Estudante
		if err := json.NewDecoder(r.Body).Decode(&estudante); err != nil {
			writeJSONError(w, http.StatusBadRequest, "Dados inválidos")
			return
		}

		// 2️⃣ Recupera usuário autenticado via Header
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

		// 3️⃣ Validação obrigatória
		if strings.TrimSpace(estudante.Nome) == "" ||
			strings.TrimSpace(estudante.CPF) == "" ||
			strings.TrimSpace(estudante.Email) == "" ||
			strings.TrimSpace(estudante.DataNascimento) == "" {
			writeJSONError(w, http.StatusBadRequest, "Nome, CPF, email e data de nascimento são obrigatórios!")
			return
		}

		// 4️⃣ Insere estudante e retorna dados criados
		err := db.QueryRow(`
			INSERT INTO estudantes (nome, cpf, email, data_nascimento, telefone, foto_url, ano_id, turma_id, usuario_id)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
			RETURNING id, nome, cpf, email, data_nascimento, telefone, foto_url, ano_id, turma_id
		`,
			estudante.Nome,
			estudante.CPF,
			estudante.Email,
			estudante.DataNascimento,
			estudante.Telefone,
			estudante.FotoURL,
			estudante.AnoID,
			estudante.TurmaID,
			usuarioID,
		).Scan(
			&estudante.ID,
			&estudante.Nome,
			&estudante.CPF,
			&estudante.Email,
			&estudante.DataNascimento,
			&estudante.Telefone,
			&estudante.FotoURL,
			&estudante.AnoID,
			&estudante.TurmaID,
		)

		if status, msg, ok := mapPQError(err); ok {
			writeJSONError(w, status, msg)
			return
		}
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "Erro ao criar estudante")
			return
		}

		// 5️⃣ Retorna o estudante criado
		writeJSON(w, http.StatusCreated, estudante)
	}
}

// =====================================================
// 🔹 Listar Estudantes (GET) — /api/estudantes
// =====================================================
//
// • Lista todos os estudantes vinculados ao usuário autenticado
// • Ordena pelo ID crescente
func ListarEstudantesHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeJSONError(w, http.StatusMethodNotAllowed, "Método não permitido")
			return
		}

		// 🔑 Autenticação
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

		// 📥 Busca estudantes
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

		writeJSON(w, http.StatusOK, estudantes)
	}
}

// =====================================================
// 🔹 Editar Estudante (PUT) — /api/estudantes/{id}
// =====================================================
//
// • Valida campos obrigatórios
// • Atualiza dados apenas se pertencer ao usuário
func EditarEstudanteHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			writeJSONError(w, http.StatusMethodNotAllowed, "Método não permitido")
			return
		}

		// 🔑 Autenticação
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

		// 🔎 Extrai ID do path
		idStr := strings.TrimPrefix(r.URL.Path, "/api/estudantes/")
		if idStr == "" {
			writeJSONError(w, http.StatusBadRequest, "ID do estudante não informado")
			return
		}

		// 📥 Decodifica body
		var estudante model.Estudante
		if err := json.NewDecoder(r.Body).Decode(&estudante); err != nil {
			writeJSONError(w, http.StatusBadRequest, "Dados inválidos")
			return
		}

		// ✅ Validação
		if strings.TrimSpace(estudante.Nome) == "" ||
			strings.TrimSpace(estudante.CPF) == "" ||
			strings.TrimSpace(estudante.Email) == "" ||
			strings.TrimSpace(estudante.DataNascimento) == "" {
			writeJSONError(w, http.StatusBadRequest, "Nome, CPF, email e data de nascimento são obrigatórios!")
			return
		}

		// ✏️ Atualização
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

// =====================================================
// 🔹 Remover Estudante (DELETE) — /api/estudantes/{id}
// =====================================================
//
// • Exclui estudante apenas se pertencer ao usuário
func RemoverEstudanteHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			writeJSONError(w, http.StatusMethodNotAllowed, "Método não permitido")
			return
		}

		// 🔑 Autenticação
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

		// 🔎 Extrai ID
		idStr := strings.TrimPrefix(r.URL.Path, "/api/estudantes/")
		if idStr == "" {
			writeJSONError(w, http.StatusBadRequest, "ID do estudante não informado")
			return
		}

		// 🗑️ Exclusão
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

// =====================================================
// 🔹 Verificar CPF duplicado (GET)
// =====================================================
//
// • Endpoint: /api/estudantes/check-cpf?cpf=...&ignoreId=...
// • Útil para validação em tempo real no frontend
// • Aceita ignoreId/excludeId para edição
func VerificarCpfHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Método não permitido", http.StatusMethodNotAllowed)
			return
		}

		// 🔑 Autenticação
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

		// 📥 Params
		cpf := strings.TrimSpace(r.URL.Query().Get("cpf"))
		ignoreID := strings.TrimSpace(r.URL.Query().Get("ignoreId"))
		if ignoreID == "" {
			ignoreID = strings.TrimSpace(r.URL.Query().Get("excludeId"))
		}

		if cpf == "" {
			http.Error(w, `{"error":"cpf é obrigatório"}`, http.StatusBadRequest)
			return
		}

		// 🔎 Verificação
		query := `SELECT 1 FROM estudantes WHERE usuario_id=$1 AND cpf=$2`
		args := []interface{}{usuarioID, cpf}
		if ignoreID != "" {
			query += ` AND id<>$3`
			args = append(args, ignoreID)
		}

		var dummy int
		err := db.QueryRow(query, args...).Scan(&dummy)
		exists := (err == nil)

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]bool{"exists": exists})
	}
}

// =====================================================
// 🔹 Verificar E-mail duplicado (GET)
// =====================================================
//
// • Endpoint: /api/estudantes/check-email?email=...&ignoreId=...
// • Aceita ignoreId/excludeId (edição)
// • Comparação case-insensitive (LOWER)
func VerificarEmailHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Método não permitido", http.StatusMethodNotAllowed)
			return
		}

		// 🔑 Autenticação
		emailHeader := strings.TrimSpace(r.Header.Get("X-User-Email"))
		if emailHeader == "" {
			http.Error(w, "Usuário não autenticado", http.StatusUnauthorized)
			return
		}

		var usuarioID int
		if err := db.QueryRow("SELECT id FROM usuarios WHERE email=$1", emailHeader).Scan(&usuarioID); err != nil {
			http.Error(w, "Usuário não encontrado", http.StatusUnauthorized)
			return
		}

		// 📥 Params
		emailParam := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("email")))
		ignoreID := strings.TrimSpace(r.URL.Query().Get("ignoreId"))
		if ignoreID == "" {
			ignoreID = strings.TrimSpace(r.URL.Query().Get("excludeId"))
		}

		if emailParam == "" {
			http.Error(w, `{"error":"email é obrigatório"}`, http.StatusBadRequest)
			return
		}

		// 🔎 Verificação
		query := `SELECT 1 FROM estudantes WHERE usuario_id=$1 AND LOWER(email)=LOWER($2)`
		args := []any{usuarioID, emailParam}
		if ignoreID != "" {
			query += ` AND id<>$3`
			args = append(args, ignoreID)
		}

		var dummy int
		err := db.QueryRow(query, args...).Scan(&dummy)
		exists := (err == nil)

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]bool{"exists": exists})
	}
}
