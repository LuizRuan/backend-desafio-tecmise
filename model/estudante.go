// backend/model/estudante.go
//
// 🔹 Objetivo deste arquivo:
//
//	Definir a estrutura de dados **Estudante**, usada em toda a aplicação
//	(backend e integração com frontend) para representar os alunos cadastrados
//	no sistema TecMise.
//
// ===============================================================
// 📌 Estrutura Estudante
// ===============================================================
//
// Cada campo representa uma coluna na tabela `estudantes` do banco de dados
// (PostgreSQL) e também é serializado/deserializado em JSON para consumo pela
// API (frontend).
//
// Campos:
//
//   - ID (int)                → Identificador único do estudante
//   - Nome (string)           → Nome completo do estudante
//   - CPF (string)            → CPF do estudante (único e validado)
//   - Email (string)          → E-mail do estudante (validado em middleware)
//   - DataNascimento (string) → Data de nascimento (ISO "yyyy-mm-dd")
//   - Telefone (string)       → Telefone de contato
//   - FotoURL (string)        → Caminho/URL da foto do aluno (se disponível)
//   - AnoID (int)             → Chave estrangeira para a tabela `anos`
//   - TurmaID (int)           → Chave estrangeira para a tabela `turmas`
//   - UsuarioID (int)         → ID do usuário dono/criador do estudante
package model

type Estudante struct {
	ID             int    `json:"id"`              // Identificador único do estudante
	Nome           string `json:"nome"`            // Nome completo
	CPF            string `json:"cpf"`             // CPF (documento nacional)
	Email          string `json:"email"`           // E-mail válido
	DataNascimento string `json:"data_nascimento"` // Data de nascimento (ISO 8601)
	Telefone       string `json:"telefone"`        // Número de telefone
	FotoURL        string `json:"foto_url"`        // Foto de perfil do aluno
	AnoID          int    `json:"ano_id"`          // Relacionamento com tabela de anos
	TurmaID        int    `json:"turma_id"`        // Relacionamento com tabela de turmas
	UsuarioID      int    `json:"usuario_id"`      // Usuário dono do registro
}
