package handler

import (
	"backend/model"
	"database/sql"
	"encoding/json"
	"net/http"
)

func CriarEstudanteHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var estudante model.Estudante

		err := json.NewDecoder(r.Body).Decode(&estudante)
		if err != nil {
			http.Error(w, "Dados inválidos", http.StatusBadRequest)
			return
		}

		// Recupera o e-mail do usuário logado no header
		email := r.Header.Get("X-User-Email")
		if email == "" {
			http.Error(w, "Usuário não autenticado", http.StatusUnauthorized)
			return
		}

		// Busca o ID do usuário a partir do e-mail
		var usuarioID int
		err = db.QueryRow("SELECT id FROM usuarios WHERE email = $1", email).Scan(&usuarioID)
		if err != nil {
			http.Error(w, "Usuário não encontrado", http.StatusUnauthorized)
			return
		}

		// Insere o estudante com o ID do usuário logado
		_, err = db.Exec(`
            INSERT INTO estudantes (nome, cpf, email, data_nascimento, telefone, foto_url, ano_id, turma_id, usuario_id)
            VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
        `, estudante.Nome, estudante.CPF, estudante.Email, estudante.DataNascimento,
			estudante.Telefone, estudante.FotoURL, estudante.AnoID, estudante.TurmaID, usuarioID)

		if err != nil {
			http.Error(w, "Erro ao criar estudante", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]string{"message": "Estudante criado com sucesso"})
	}
}
