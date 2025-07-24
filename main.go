package main

import (
	"database/sql"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"regexp"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

// --- Estruturas para Requests e Responses ---
type RegisterRequest struct {
	Nome  string `json:"nome"`
	Email string `json:"email"`
	Senha string `json:"senha"`
}

type LoginRequest struct {
	Email string `json:"email"`
	Senha string `json:"senha"`
}

type UserResponse struct {
	ID    int    `json:"id"`
	Email string `json:"email"`
	Nome  string `json:"nome"`
}

type Estudante struct {
	Nome      string `json:"nome"`
	Email     string `json:"email"`
	Telefone  string `json:"telefone"`
	UsuarioID int    `json:"usuario_id"`
}

// --- Handler para criar estudante ---
func criarEstudanteHandler(w http.ResponseWriter, r *http.Request) {
	enableCors(&w)
	if r.Method != http.MethodPost {
		http.Error(w, "Método não permitido", http.StatusMethodNotAllowed)
		return
	}
	var est Estudante
	if err := json.NewDecoder(r.Body).Decode(&est); err != nil {
		http.Error(w, "JSON inválido", http.StatusBadRequest)
		log.Println("Erro ao decodificar estudante:", err)
		return
	}
	_, err := db.Exec("INSERT INTO estudantes (nome, email, telefone, usuario_id) VALUES ($1, $2, $3, $4)",
		est.Nome, est.Email, est.Telefone, est.UsuarioID)
	if err != nil {
		http.Error(w, "Erro ao salvar estudante", http.StatusInternalServerError)
		log.Println("Erro ao salvar estudante:", err)
		return
	}
	w.WriteHeader(http.StatusCreated)
}

// --- CORS ---
func enableCors(w *http.ResponseWriter) {
	(*w).Header().Set("Access-Control-Allow-Origin", "*")
	(*w).Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
	(*w).Header().Set("Access-Control-Allow-Headers", "Content-Type")
}

// --- MAIN ---
func main() {
	http.HandleFunc("/estudantes", listarEstudantesHandler) // Já deve ter esse, ajuste para GET e POST

	conectarBanco() // Certifique-se que existe em db.go

	http.HandleFunc("/register", registerHandler)
	http.HandleFunc("/login", loginHandler)

	log.Println("Servidor rodando em http://localhost:8080")
	http.ListenAndServe(":8080", nil)
}

// --- CADASTRO DE USUÁRIO ---
func registerHandler(w http.ResponseWriter, r *http.Request) {
	enableCors(&w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "Método não permitido", http.StatusMethodNotAllowed)
		return
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Erro ao ler dados", http.StatusBadRequest)
		log.Println("Erro ao ler dados:", err)
		return
	}
	var req RegisterRequest
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, "JSON inválido", http.StatusBadRequest)
		log.Println("JSON inválido:", err)
		return
	}
	req.Email = strings.TrimSpace(req.Email)
	req.Nome = strings.TrimSpace(req.Nome)
	req.Senha = strings.TrimSpace(req.Senha)

	if !validarEmail(req.Email) {
		http.Error(w, "E-mail inválido", http.StatusBadRequest)
		return
	}
	if len(req.Nome) < 2 {
		http.Error(w, "Nome muito curto", http.StatusBadRequest)
		return
	}
	if len(req.Senha) < 8 {
		http.Error(w, "Senha muito curta", http.StatusBadRequest)
		return
	}
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM usuarios WHERE email=$1", req.Email).Scan(&count)
	if err != nil {
		http.Error(w, "Erro ao verificar e-mail", http.StatusInternalServerError)
		log.Println("Erro ao verificar e-mail:", err)
		return
	}
	if count > 0 {
		http.Error(w, "E-mail já cadastrado", http.StatusConflict)
		return
	}
	hashed, err := bcrypt.GenerateFromPassword([]byte(req.Senha), bcrypt.DefaultCost)
	if err != nil {
		http.Error(w, "Erro ao processar senha", http.StatusInternalServerError)
		log.Println("Erro ao processar senha:", err)
		return
	}
	_, err = db.Exec("INSERT INTO usuarios (nome, email, senha_hash) VALUES ($1, $2, $3)", req.Nome, req.Email, string(hashed))
	if err != nil {
		http.Error(w, "Erro ao salvar usuário", http.StatusInternalServerError)
		log.Println("Erro ao salvar usuário:", err)
		return
	}
	w.WriteHeader(http.StatusCreated)
	w.Write([]byte(`{"ok":true}`))
}

// --- LOGIN DE USUÁRIO ---
func loginHandler(w http.ResponseWriter, r *http.Request) {
	enableCors(&w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "Método não permitido", http.StatusMethodNotAllowed)
		return
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Erro ao ler dados", http.StatusBadRequest)
		return
	}
	var req LoginRequest
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, "JSON inválido", http.StatusBadRequest)
		return
	}
	req.Email = strings.TrimSpace(req.Email)
	req.Senha = strings.TrimSpace(req.Senha)

	if !validarEmail(req.Email) || len(req.Senha) < 8 {
		http.Error(w, "E-mail ou senha inválidos", http.StatusUnauthorized)
		return
	}
	var id int
	var nome string
	var senhaHash string
	err = db.QueryRow("SELECT id, nome, senha_hash FROM usuarios WHERE email=$1", req.Email).Scan(&id, &nome, &senhaHash)
	if err == sql.ErrNoRows {
		http.Error(w, "E-mail ou senha incorretos", http.StatusUnauthorized)
		return
	} else if err != nil {
		http.Error(w, "Erro ao verificar usuário", http.StatusInternalServerError)
		log.Println("Erro ao verificar usuário:", err)
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(senhaHash), []byte(req.Senha)) != nil {
		http.Error(w, "E-mail ou senha incorretos", http.StatusUnauthorized)
		return
	}
	resp := UserResponse{
		ID:    id,
		Email: req.Email,
		Nome:  nome, // <-- nome real do banco
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
func listarEstudantesHandler(w http.ResponseWriter, r *http.Request) {
	enableCors(&w)
	if r.Method != http.MethodGet {
		http.Error(w, "Método não permitido", http.StatusMethodNotAllowed)
		return
	}
	usuarioID := r.URL.Query().Get("usuario_id")
	rows, err := db.Query("SELECT id, nome, cpf, email, data_nascimento, telefone, foto_url, ano_id, turma_id FROM estudantes WHERE usuario_id=$1", usuarioID)
	if err != nil {
		http.Error(w, "Erro ao buscar estudantes", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var estudantes []map[string]interface{}
	for rows.Next() {
		var e struct {
			ID             int
			Nome           string
			CPF            string
			Email          string
			DataNascimento string
			Telefone       string
			FotoUrl        string
			AnoId          int
			TurmaId        int
		}
		if err := rows.Scan(&e.ID, &e.Nome, &e.CPF, &e.Email, &e.DataNascimento, &e.Telefone, &e.FotoUrl, &e.AnoId, &e.TurmaId); err != nil {
			continue
		}
		estudantes = append(estudantes, map[string]interface{}{
			"id": e.ID, "nome": e.Nome, "cpf": e.CPF, "email": e.Email, "dataNascimento": e.DataNascimento,
			"telefone": e.Telefone, "fotoUrl": e.FotoUrl, "anoId": e.AnoId, "turmaId": e.TurmaId,
		})
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(estudantes)
}

// --- Validação simples de e-mail ---
func validarEmail(email string) bool {
	re := regexp.MustCompile(`^[\w\-.]+@([\w-]+\.)+[\w-]{2,4}$`)
	return re.MatchString(email)
}
