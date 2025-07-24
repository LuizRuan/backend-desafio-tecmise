package model

type Estudante struct {
	ID             int    `json:"id"`
	Nome           string `json:"nome"`
	CPF            string `json:"cpf"`
	Email          string `json:"email"`
	DataNascimento string `json:"data_nascimento"`
	Telefone       string `json:"telefone"`
	FotoURL        string `json:"foto_url"`
	AnoID          int    `json:"ano_id"`
	TurmaID        int    `json:"turma_id"`
	UsuarioID      int    `json:"usuario_id"`
}
