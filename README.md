TecMise Backend — API de Gestão Escolar

Bem-vindo ao TecMise Backend, o núcleo de processamento e integração de dados do sistema completo de gestão escolar TecMise.

📚 Sobre o Projeto

O TecMise Backend foi feito em Go (Golang) com banco PostgreSQL 16.9 e geração de queries via sqlc.
Ele fornece uma API RESTful para cadastro, autenticação e gerenciamento de estudantes, anos/turmas e usuários.

O diferencial é que cada pessoa pode rodar o sistema com seu próprio banco de dados — basta seguir os passos abaixo.

⚙️ Tecnologias Utilizadas

Go (Golang): v1.24.5

PostgreSQL: v16.9

sqlc: v1.29.0

pgAdmin: (opcional, interface gráfica)

Git: controle de versão

🚀 Guia de Instalação
1. Pré-requisitos

Instale o Go v1.24.5+

Instale o PostgreSQL v16.9

Instale o sqlc v1.29.0

Instale o Git

2. Clone o Projeto
[git clone https://github.com/LuizRuan/tecmise.git](https://github.com/LuizRuan/backend-desafio-tecmise)
cd tecmise/backend

3. Crie o Banco de Dados

Entre no PostgreSQL pelo terminal:

psql -U postgres


Crie o banco:

CREATE DATABASE tecmise;


Conecte nele:

\c tecmise;


Agora crie as tabelas principais (você pode usar pgAdmin ou colar no terminal):

-- Usuários do sistema
CREATE TABLE usuarios (
  id SERIAL PRIMARY KEY,
  nome VARCHAR(100) NOT NULL,
  email VARCHAR(200) UNIQUE NOT NULL,
  senha_hash VARCHAR(300) NOT NULL,
  foto_url TEXT,
  tutorial_visto BOOLEAN DEFAULT FALSE
);

-- Anos/Turmas
CREATE TABLE anos (
  id SERIAL PRIMARY KEY,
  nome VARCHAR(120) NOT NULL,
  usuario_id INT NOT NULL REFERENCES usuarios(id) ON DELETE CASCADE
);

-- Estudantes
CREATE TABLE estudantes (
  id SERIAL PRIMARY KEY,
  nome VARCHAR(120) NOT NULL,
  cpf VARCHAR(14) NOT NULL,
  email VARCHAR(200),
  telefone VARCHAR(32),
  foto_url TEXT,
  ano_id INT REFERENCES anos(id) ON DELETE CASCADE,
  turma_id INT,
  usuario_id INT NOT NULL REFERENCES usuarios(id) ON DELETE CASCADE,
  UNIQUE (cpf, usuario_id)
);

-- Fotos de Perfil
CREATE TABLE fotos_perfil (
  id SERIAL PRIMARY KEY,
  usuario_id INT NOT NULL REFERENCES usuarios(id) ON DELETE CASCADE,
  nome_arquivo VARCHAR(255),
  foto BYTEA,
  data_upload TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

4. Configuração do Ambiente

Copie o exemplo para .env:

cp .env.example .env


Edite o .env:

DATABASE_URL=postgres://postgres:sua_senha@localhost:5432/tecmise?sslmode=disable

5. Instale Dependências
go mod tidy

6. Rode o Servidor
go run .


O backend ficará disponível em:
👉 http://localhost:8080

🛠️ Testando a API

Exemplo com cURL:

# Criar usuário
curl -X POST http://localhost:8080/register \
  -H "Content-Type: application/json" \
  -d '{"nome": "Beatriz", "email": "bea@email.com", "senha": "123456"}'

# Login
curl -X POST http://localhost:8080/login \
  -H "Content-Type: application/json" \
  -d '{"email": "bea@email.com", "senha": "123456"}'

📌 Observações

Cada usuário só acessa seus próprios estudantes, anos e fotos.

Exclusão em cascata garante que, ao apagar um ano, seus estudantes também são removidos.

A autenticação é feita via header X-User-Email.

💬 Contribuição

Sugestões e melhorias são bem-vindas.
Abra uma issue ou mande um pull request!