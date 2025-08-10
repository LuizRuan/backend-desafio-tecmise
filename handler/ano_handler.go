// handler/ano_handler.go
//
// Handlers para CRUD de Anos (Ano Escolar) do TecMise.

package handler

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"
)

// Estrutura Ano
type Ano struct {
	ID   int    `json:"id"`
	Nome string `json:"nome"`
}

// GET /api/anos — Lista todos os anos
func ListarAnosHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rows, err := db.Query("SELECT id, nome FROM anos ORDER BY id ASC")
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

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(anos)
	}
}

// POST /api/anos — Cria um novo ano
func CriarAnoHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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

		var novoID int
		err := db.QueryRow("INSERT INTO anos (nome) VALUES ($1) RETURNING id", input.Nome).Scan(&novoID)
		if err != nil {
			http.Error(w, "Erro ao criar ano: "+err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":   novoID,
			"nome": input.Nome,
		})
	}
}

// DELETE /api/anos/{id} — Remove ano/turma e todos os estudantes desse ano
func RemoverAnoHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := strings.TrimPrefix(r.URL.Path, "/api/anos/")
		if idStr == "" {
			http.Error(w, "ID do ano/turma não informado", http.StatusBadRequest)
			return
		}
		if r.Method != http.MethodDelete {
			http.Error(w, "Método não permitido", http.StatusMethodNotAllowed)
			return
		}

		tx, err := db.Begin()
		if err != nil {
			http.Error(w, "Erro ao iniciar transação", http.StatusInternalServerError)
			return
		}

		// Exclui estudantes vinculados a esse ano/turma
		_, err = tx.Exec("DELETE FROM estudantes WHERE ano_id = $1", idStr)
		if err != nil {
			tx.Rollback()
			http.Error(w, "Erro ao remover estudantes vinculados", http.StatusInternalServerError)
			return
		}

		// Remove o próprio ano/turma
		res, err := tx.Exec("DELETE FROM anos WHERE id = $1", idStr)
		if err != nil {
			tx.Rollback()
			http.Error(w, "Erro ao remover ano/turma", http.StatusInternalServerError)
			return
		}

		rows, _ := res.RowsAffected()
		if rows == 0 {
			tx.Rollback()
			http.Error(w, "Ano/Turma não encontrado", http.StatusNotFound)
			return
		}

		err = tx.Commit()
		if err != nil {
			http.Error(w, "Erro ao confirmar exclusão", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusNoContent) // 204
	}
}
