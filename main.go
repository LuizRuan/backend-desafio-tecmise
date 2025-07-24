package main

import (
	"database/sql"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"regexp"
	"strings"

	"github.com/gorilla/mux"

	_ "github.com/lib/pq"
	"golang.org/x/crypto/bcrypt"
)

var db *sql.DB

// --------- Estruturas ---------
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
	ID             int    `json:"id"`
	Nome           string `json:"nome"`
	CPF            string `json:"cpf"`
	Email          string `json:"email"`
	DataNascimento string `json:"data_nascimento"`
	Telefone       string `json:"telefone"`
	FotoUrl        string `json:"foto_url"`
	AnoId          int    `json:"ano_id"`
	TurmaId        int    `json:"turma_id"`
	UsuarioID      int    `json:"usuario_id"`
}

// --------- Banco ---------
func conectarBanco() {
	connStr := "host=localhost port=5432 user=postgres password=senha123 dbname=clientes_db sslmode=disable"
	var err error
	db, err = sql.Open("postgres", connStr)
	if err != nil {
		log.Fatal("Erro ao conectar ao banco:", err)
	}
	if err = db.Ping(); err != nil {
		log.Fatal("Erro ao pingar banco:", err)
	}
	log.Println("Banco conectado com sucesso!")
}

// --------- CORS ---------
func enableCors(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
}

// --------- Handler Central dos Estudantes (GET/POST) ---------
func estudantesHandler(w http.ResponseWriter, r *http.Request) {
	enableCors(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}
	switch r.Method {
	case http.MethodGet:
		listarEstudantesHandler(w, r)
	case http.MethodPost:
		criarEstudanteHandler(w, r)
	default:
		http.Error(w, "Método não permitido", http.StatusMethodNotAllowed)
	}
}

// --------- Criar Estudante ---------
func criarEstudanteHandler(w http.ResponseWriter, r *http.Request) {
	enableCors(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "Método não permitido", http.StatusMethodNotAllowed)
		return
	}

	var est Estudante
	if err := json.NewDecoder(r.Body).Decode(&est); err != nil {
		http.Error(w, "JSON inválido", http.StatusBadRequest)
		log.Println("JSON inválido:", err)
		return
	}

	_, err := db.Exec(
		"INSERT INTO estudantes (nome, email, telefone, usuario_id, cpf, data_nascimento, foto_url, ano_id, turma_id) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)",
		est.Nome, est.Email, est.Telefone, est.UsuarioID, est.CPF, est.DataNascimento, est.FotoUrl, est.AnoId, est.TurmaId,
	)

	if err != nil {
		if strings.Contains(err.Error(), "unique_email") {
			http.Error(w, "E-mail já cadastrado", http.StatusConflict)
			return
		}
		if strings.Contains(err.Error(), "unique_cpf") {
			http.Error(w, "CPF já cadastrado", http.StatusConflict)
			return
		}
		http.Error(w, "Erro ao salvar estudante", http.StatusInternalServerError)
		log.Println("Erro ao salvar estudante:", err)
		return
	}

	w.WriteHeader(http.StatusCreated)
	w.Write([]byte(`{"ok":true}`))
}

// --------- Listar Estudantes ---------
func listarEstudantesHandler(w http.ResponseWriter, r *http.Request) {
	usuarioID := r.URL.Query().Get("usuario_id")
	if usuarioID == "" {
		http.Error(w, "usuario_id obrigatório", http.StatusBadRequest)
		return
	}
	rows, err := db.Query(
		"SELECT id, nome, cpf, email, data_nascimento, telefone, foto_url, ano_id, turma_id FROM estudantes WHERE usuario_id=$1",
		usuarioID,
	)
	if err != nil {
		http.Error(w, "Erro ao buscar estudantes", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var estudantes []Estudante
	for rows.Next() {
		var e Estudante
		if err := rows.Scan(&e.ID, &e.Nome, &e.CPF, &e.Email, &e.DataNascimento, &e.Telefone, &e.FotoUrl, &e.AnoId, &e.TurmaId); err != nil {
			continue
		}
		estudantes = append(estudantes, e)
	}
	w.Header().Set("Content-Type", "application/json")
	if estudantes == nil {
		estudantes = []Estudante{}
	}

	json.NewEncoder(w).Encode(estudantes)
}

// --------- Cadastro Usuário ---------
func registerHandler(w http.ResponseWriter, r *http.Request) {
	enableCors(w)
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

// --------- Login Usuário ---------
func loginHandler(w http.ResponseWriter, r *http.Request) {
	enableCors(w)
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
		Nome:  nome,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// --------- Validação de E-mail ---------
func validarEmail(email string) bool {
	re := regexp.MustCompile(`^[\w\-.]+@([\w-]+\.)+[\w-]{2,4}$`)
	return re.MatchString(email)
}

// --------- MAIN ---------
func main() {
	conectarBanco()
	r := mux.NewRouter()
	r.HandleFunc("/estudantes/{id}", deletarEstudanteHandler).Methods("DELETE")
	r.HandleFunc("/estudantes/{id}", atualizarEstudanteHandler).Methods("PUT")
	r.HandleFunc("/estudantes", estudantesHandler)
	r.HandleFunc("/register", registerHandler)
	r.HandleFunc("/login", loginHandler)
	r.HandleFunc("/usuario", usuarioHandler)
	r.HandleFunc("/estudantes/{id}", func(w http.ResponseWriter, r *http.Request) {
		enableCors(w)
		w.WriteHeader(http.StatusOK)
	}).Methods("OPTIONS")

	log.Println("Servidor rodando em http://localhost:8080")
	http.ListenAndServe(":8080", r)
}

// Handler para buscar dados de 1 usuário
func usuarioHandler(w http.ResponseWriter, r *http.Request) {
	enableCors(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "Método não permitido", http.StatusMethodNotAllowed)
		return
	}

	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "ID obrigatório", http.StatusBadRequest)
		return
	}

	var u UserResponse
	err := db.QueryRow("SELECT id, nome, email FROM usuarios WHERE id = $1", id).Scan(&u.ID, &u.Nome, &u.Email)
	if err == sql.ErrNoRows {
		http.Error(w, "Usuário não encontrado", http.StatusNotFound)
		return
	} else if err != nil {
		http.Error(w, "Erro ao buscar usuário", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(u)
}
func deletarEstudanteHandler(w http.ResponseWriter, r *http.Request) {
	enableCors(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != http.MethodDelete {
		http.Error(w, "Método não permitido", http.StatusMethodNotAllowed)
		return
	}
	vars := mux.Vars(r)
	id := vars["id"]
	if id == "" {
		http.Error(w, "ID do estudante obrigatório", http.StatusBadRequest)
		return
	}
	_, err := db.Exec("DELETE FROM estudantes WHERE id = $1", id)
	if err != nil {
		http.Error(w, "Erro ao excluir estudante", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func atualizarEstudanteHandler(w http.ResponseWriter, r *http.Request) {
	enableCors(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != http.MethodPut {
		http.Error(w, "Método não permitido", http.StatusMethodNotAllowed)
		return
	}

	vars := mux.Vars(r)
	id := vars["id"]
	if id == "" {
		http.Error(w, "ID do estudante obrigatório", http.StatusBadRequest)
		return
	}

	var est Estudante
	if err := json.NewDecoder(r.Body).Decode(&est); err != nil {
		http.Error(w, "JSON inválido", http.StatusBadRequest)
		return
	}

	// Verificar se já existe outro estudante com o mesmo email (exceto ele mesmo)
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM estudantes WHERE email=$1 AND id<>$2", est.Email, id).Scan(&count)
	if err != nil {
		http.Error(w, "Erro ao verificar e-mail", http.StatusInternalServerError)
		return
	}
	if count > 0 {
		http.Error(w, "E-mail já cadastrado", http.StatusConflict)
		return
	}

	// Verificar se já existe outro estudante com o mesmo cpf (exceto ele mesmo)
	err = db.QueryRow("SELECT COUNT(*) FROM estudantes WHERE cpf=$1 AND id<>$2", est.CPF, id).Scan(&count)
	if err != nil {
		http.Error(w, "Erro ao verificar CPF", http.StatusInternalServerError)
		return
	}
	if count > 0 {
		http.Error(w, "CPF já cadastrado", http.StatusConflict)
		return
	}

	_, err = db.Exec(
		`UPDATE estudantes SET nome=$1, email=$2, telefone=$3, cpf=$4, data_nascimento=$5, foto_url=$6, ano_id=$7, turma_id=$8
        WHERE id=$9`,
		est.Nome, est.Email, est.Telefone, est.CPF, est.DataNascimento, est.FotoUrl, est.AnoId, est.TurmaId, id,
	)
	if err != nil {
		http.Error(w, "Erro ao atualizar estudante", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"ok":true}`))
}
