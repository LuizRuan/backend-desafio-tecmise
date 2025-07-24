# Backend TecMise - Setup e Guia Completo

Este guia te ajuda a **instalar o banco PostgreSQL**, rodar o **backend em Go**, preparar o ambiente local e usar o **pgAdmin** para gerenciar os dados do sistema TecMise.

---

## 1. Instalação dos Pré-requisitos

### PostgreSQL

- **Windows:**  
  [Baixe o instalador oficial](https://www.postgresql.org/download/windows/)

- **Linux (Ubuntu):**
  ```bash
  sudo apt update
  sudo apt install postgresql postgresql-contrib
Mac (Homebrew):

bash
Copiar
Editar
brew install postgresql
Dica: Após instalar, use o pgAdmin (interface gráfica) ou psql (terminal) para gerenciar o banco.

Go (Golang)
Baixe o Go e instale conforme seu sistema operacional.

Para checar a instalação:

bash
Copiar
Editar
go version
2. Criar o Banco de Dados
Acesse o PostgreSQL (pgAdmin ou terminal):

bash
Copiar
Editar
psql -U postgres
Crie o banco:

sql
Copiar
Editar
CREATE DATABASE clientes_db;
3. Criar as Tabelas
No pgAdmin (Query Tool) ou pelo psql, execute:

sql
Copiar
Editar
-- Usuários
CREATE TABLE IF NOT EXISTS usuarios (
    id serial PRIMARY KEY,
    nome VARCHAR(120) NOT NULL,
    email VARCHAR(200) NOT NULL UNIQUE,
    senha_hash VARCHAR(300) NOT NULL
);

-- Anos
CREATE TABLE IF NOT EXISTS anos (
    id serial PRIMARY KEY,
    nome VARCHAR(32) NOT NULL
);

-- Turmas
CREATE TABLE IF NOT EXISTS turmas (
    id serial PRIMARY KEY,
    nome VARCHAR(32) NOT NULL,
    ano_id INTEGER REFERENCES anos(id) ON DELETE CASCADE
);

-- Estudantes
CREATE TABLE IF NOT EXISTS estudantes (
    id serial PRIMARY KEY,
    nome VARCHAR(120) NOT NULL,
    cpf VARCHAR(14) NOT NULL UNIQUE,
    email VARCHAR(200) NOT NULL UNIQUE,
    data_nascimento DATE NOT NULL,
    telefone VARCHAR(32),
    foto_url TEXT,
    ano_id INTEGER NOT NULL REFERENCES anos(id),
    turma_id INTEGER NOT NULL REFERENCES turmas(id),
    usuario_id INTEGER NOT NULL REFERENCES usuarios(id) ON DELETE CASCADE
);
Exemplo para popular as tabelas:

sql
Copiar
Editar
INSERT INTO anos (nome) VALUES ('6º ano'), ('7º ano');
INSERT INTO turmas (nome, ano_id) VALUES ('A', 1), ('B', 1), ('A', 2);
4. Configurar a Conexão do Go com o Banco
No seu main.go a string padrão de conexão é:

go
Copiar
Editar
connStr := "host=localhost port=5432 user=postgres password=senha123 dbname=clientes_db sslmode=disable"
Altere a senha conforme seu ambiente.

Se o usuário não for postgres, altere também.

Dica: Use variáveis de ambiente ou um .env para maior segurança (opcional).

5. Instalar Dependências do Backend
No terminal, acesse a pasta do backend e rode:

bash
Copiar
Editar
go mod tidy
go get github.com/lib/pq
go get golang.org/x/crypto/bcrypt
6. Rodar o Backend
Na pasta backend, execute:

bash
Copiar
Editar
go run .
O backend estará disponível em:
http://localhost:8080

7. Testar os Endpoints
Cadastro de usuário:
POST http://localhost:8080/register

Login:
POST http://localhost:8080/login

Listar estudantes:
GET http://localhost:8080/estudantes?usuario_id=ID_DO_USUARIO

Cadastrar estudante:
POST http://localhost:8080/estudantes

Atualizar estudante:
PUT http://localhost:8080/estudantes/{id}

Excluir estudante:
DELETE http://localhost:8080/estudantes/{id}

Dica: Teste com o Postman ou similar.

8. Dicas para uso do pgAdmin
Use o Query Tool para executar comandos SQL (criação de tabelas, inserções, consultas, etc).

Clique com o botão direito em uma tabela e vá em View/Edit Data > All Rows para editar/visualizar registros.

Para exportar dados, use o menu de contexto da tabela.

Problemas Comuns
Erro de conexão: Confira host, usuário, senha e nome do banco.

Porta em uso: Certifique-se que o PostgreSQL está na porta 5432.

Permissão negada: Revise permissões do usuário no banco.

Tabela não existe: Execute o script de criação de tabelas.

Backend não roda: Confira se o Go está instalado corretamente e todas dependências estão baixadas.

Observação
O frontend depende do backend rodando. Sempre inicie o backend antes de rodar o frontend Nuxt.

Para configurar o frontend, veja o README na pasta correspondente.